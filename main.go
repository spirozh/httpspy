package main

import (
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
	"time"

	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

const DB_NAME = "./httpspy.db"

//go:embed "watch.html"
var watchPage string

//go:embed "watch.js"
var watchJs string

//go:embed "watch.css"
var watchCss string

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

	updateChs := map[int]chan struct{}{}
	update := func() {
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
		updateCh <- struct{}{}
		for {
			id := rand.Int()
			if _, exists := updateChs[id]; !exists {
				updateChs[id] = updateCh
				defer close(updateCh)
				defer delete(updateChs, id)
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

		reqs, err := getRequests(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// log request
		headers, err := json.Marshal(r.Header)
		if err != nil {
			panic(err)
		}

		var body strings.Builder
		io.Copy(&body, r.Body)

		req := Request{
			Timestamp: time.Now().UTC(),
			Method:    r.Method,
			URL:       r.URL.String(),
			Headers:   string(headers),
			Body:      body.String(),
		}

		res, errEx := db.ExecContext(r.Context(), `
			insert into requests(method, url, headers, body, timestamp) values (?, ?, ?, ?, ?) returning id
		`, req.Method, req.URL, req.Headers, req.Body, req.Timestamp)
		if errEx != nil {
			panic(errEx)
		}

		id, errEx := res.LastInsertId()

		if errEx != nil {
			panic(errEx)
		}
		req.Id = int(id)
		update()
	})

	requests, err := getRequests(db)
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

func getRequests(db *sql.DB) (requests []Request, err error) {
	rows, err := db.Query(`
		select id, method, url, headers, body, timestamp from requests order by timestamp desc
	`)
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

func staticHandler(body string, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Content-Type", contentType)
		fmt.Fprint(w, body)
	}
}

type Request struct {
	Id        int
	Timestamp time.Time
	Method    string
	URL       string
	Headers   string
	Body      string
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
