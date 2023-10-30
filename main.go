package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

const DB_NAME = "./httpspy.db"

//go:embed "watch.html"
var watchPage []byte

//go:embed "watch.js"
var watchJs []byte

//go:embed "watch.css"
var watchCss []byte

func main() {
	serverCtx, serverDone := context.WithCancel(context.Background())
	mux := http.NewServeMux()

	ensureDB()

	// open database
	db, err := sql.Open("sqlite3", DB_NAME)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	updateChRWMu := sync.RWMutex{}
	updateChs := map[int]chan struct{}{}
	update := func() {
		updateChRWMu.RLock()
		defer updateChRWMu.RUnlock()

		for _, ch := range updateChs {
			ch <- struct{}{}
		}
	}

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/watch", staticHandler(watchPage, "text/html"))
	mux.HandleFunc("/watch.js", staticHandler(watchJs, "text/javascript"))
	mux.HandleFunc("/watch.css", staticHandler(watchCss, "text/css"))

	mux.HandleFunc("/SSEUpdate", func(w http.ResponseWriter, r *http.Request) {
		updateCh := make(chan struct{}, 1)
		defer close(updateCh)

		updateCh <- struct{}{}

		// loop to ensure unique id
		for {
			id := rand.Int()
			if _, exists := updateChs[id]; !exists {
				updateChRWMu.Lock()
				updateChs[id] = updateCh
				updateChRWMu.Unlock()

				// when all is said and done, close the
				defer func() {
					updateChRWMu.Lock()
					delete(updateChs, id)
					updateChRWMu.Unlock()
				}()
				break
			}
		}

		// write headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		requestCtx := r.Context()

		for {
			select {
			case <-updateCh:
				fmt.Fprint(w, "data: updated\n\n")

				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				continue
			case <-requestCtx.Done():
			case <-serverCtx.Done():
			}
			break
		}
	})

	mux.HandleFunc("/requests", func(w http.ResponseWriter, r *http.Request) {
		// show requests
		reqs, err := getRequests(db, r.URL.Query().Get("url"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		if reqs == nil {
			reqs = []Request{}
		}
		b, err := json.MarshalIndent(reqs, "", " ")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		fmt.Fprintln(w, string(b))
	})

	mux.HandleFunc("/clear", func(w http.ResponseWriter, _ *http.Request) {
		_, err := db.Exec("delete from requests")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		update()
	})

	// request writer (with plenty of room to grow)
	writeCh := make(chan *Request, 1000)

	go func() {
		done := false
		for !done {
			select {
			case req := <-writeCh:
				writeRequest(db, req)
				update()
			case <-serverCtx.Done():
				done = true
			}
		}
	}()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// log request
		headers, err := json.Marshal(r.Header)
		if err != nil {
			panic(err)
		}

		var body strings.Builder
		io.Copy(&body, r.Body)

		req := &Request{
			Timestamp:   time.Now().UTC(),
			Method:      r.Method,
			URL:         r.URL.String(),
			Headers:     string(headers),
			Body:        body.String(),
			idChan:      make(chan int64, 1),
			dbErrorChan: make(chan error, 1),
		}
		writeCh <- req
		if err := <-req.dbErrorChan; err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		req.Id = <-req.idChan
		reqBytes, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			panic(err)
		}

		w.Header().Add("Content-Type", "application/json")
		io.Copy(w, bytes.NewReader(reqBytes))
	})

	requests, err := getRequests(db, "")
	if err != nil {
		panic(err)
	}
	fmt.Println(len(requests), " request(s) stored.")

	addr := ":6969"
	fmt.Printf("--\naddr: %q\n", addr)

	s := http.Server{
		Addr:        addr,
		Handler:     mux,
		BaseContext: func(_ net.Listener) context.Context { return serverCtx },
	}
	go s.ListenAndServe()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	select {
	case <-c:
		serverDone()
	case <-serverCtx.Done():
	}

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second)
	defer ctxCancel()
	s.Shutdown(ctx)
}

func ensureDB() {
	// if database doesn't exist, create it
	if _, err := os.Stat(DB_NAME); os.IsNotExist(err) {
		db, err := sql.Open("sqlite3", DB_NAME)
		if err != nil {
			panic(err)
		}
		defer db.Close()

		// create tables
		_, err = db.Exec(`
		create table requests (
			id        integer not null primary key,
			method    text,
			url       text,
			headers   text,
			body      text,
			timestamp datetime
		)
	`)
		if err != nil {
			panic(err)
		}
	}
}

func getRequests(db *sql.DB, url string) (requests []Request, err error) {
	rows, err := db.Query(`
		select id, method, url, headers, body, timestamp from requests where ''=$1 or url=$1 order by timestamp desc
	`, url)
	if err != nil {
		return requests, err
	}
	defer rows.Close()

	for rows.Next() {
		var r Request

		rowErr := rows.Scan(&r.Id, &r.Method, &r.URL, &r.Headers, &r.Body, &r.Timestamp)
		err = errors.Join(err, rowErr)

		requests = append(requests, r)
	}
	return requests, err
}

func writeRequest(db *sql.DB, req *Request) {
	res, err := db.Exec(`
		insert into requests(method, url, headers, body, timestamp) values ($1, $2, $3, $4, $5) returning id
	`, req.Method, req.URL, req.Headers, req.Body, req.Timestamp)
	if err != nil {
		req.dbErrorChan <- err
		req.idChan <- -1
		return
	}

	id, err := res.LastInsertId()
	req.dbErrorChan <- err
	req.idChan <- id
}

func staticHandler(body []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Content-Type", contentType)
		io.Copy(w, bytes.NewReader(body))
	}
}

type Request struct {
	Id          int64
	Timestamp   time.Time
	Method      string
	URL         string
	Headers     string
	Body        string
	idChan      chan int64
	dbErrorChan chan error
}

func (r Request) String() string {
	return fmt.Sprintf(`
		id: %d
		timestamp: %s
		method url: %s %s,
		header: %v,
		body: %q
	`, r.Id, r.Timestamp, r.Method, r.URL, r.Headers, r.Body)
}
