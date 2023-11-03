// Package main is the main package for the httpspy application
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	serverCtx, serverDone := context.WithCancel(context.Background())
	mux := http.NewServeMux()

	// open database
	db := openDb()
	defer db.Close()

	updateListeners := newTokenChanMap()

	mux.HandleFunc("/favicon.ico", staticHandler(watchFavicon, "image/x-icon"))
	mux.HandleFunc("/watch", staticHandler(watchPage, "text/html"))
	mux.HandleFunc("/watch.js", staticHandler(watchJs, "text/javascript"))
	mux.HandleFunc("/watch.css", staticHandler(watchCSS, "text/css"))

	mux.HandleFunc("/SSEUpdate", func(w http.ResponseWriter, r *http.Request) {
		tok, updateCh := updateListeners.newToken()
		defer updateListeners.close(tok)

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

		updateListeners.notify()
	})

	// request writer (with plenty of room to grow)
	writeCh := make(chan *Request, 1000)

	// write requests from a single thread (required by sqlite)
	go func() {
		var done bool
		for !done {
			select {
			case req := <-writeCh:
				writeRequest(db, req)
				updateListeners.notify()
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

		// write request to db (and get id+errs back on channels)
		writeCh <- req

		if err := <-req.dbErrorChan; err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		req.ID = <-req.idChan
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

	addr := ":6968"
	fmt.Printf("--\naddr: %q\n", addr)

	s := http.Server{
		Addr:        addr,
		Handler:     mux,
		BaseContext: func(_ net.Listener) context.Context { return serverCtx },
	}
	go func() {
		err := s.ListenAndServe()
		fmt.Println(err)
		serverDone()
	}()

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

func staticHandler(body []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Content-Type", contentType)
		io.Copy(w, bytes.NewReader(body))
	}
}

// Request contains information about an incoming request, plus two channels for async updates
type Request struct {
	ID        int64
	Timestamp time.Time
	Method    string
	URL       string
	Headers   string
	Body      string

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
	`, r.ID, r.Timestamp, r.Method, r.URL, r.Headers, r.Body)
}
