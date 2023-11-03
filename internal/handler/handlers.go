// Package handler contains all the handlers for this app
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"www.github.com/spirozh/httpspy/internal/data"
	"www.github.com/spirozh/httpspy/internal/db"
	"www.github.com/spirozh/httpspy/internal/notification"
	"www.github.com/spirozh/httpspy/internal/static"
)

// New creates a serveMux that handles all the http requests
func New(serverCtx context.Context, doneChan chan struct{}) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/favicon.ico", staticHandler(static.WatchFavicon, "image/x-icon"))
	mux.Handle("/watch", staticHandler(static.WatchPage, "text/html"))
	mux.Handle("/watch.js", staticHandler(static.WatchJS, "text/javascript"))
	mux.Handle("/watch.css", staticHandler(static.WatchCSS, "text/css"))

	database := db.OpenDb()
	go func() {
		<-serverCtx.Done()
		database.Close()
		close(doneChan)
	}()
	requests, err := database.GetRequests("")
	if err != nil {
		panic(err)
	}
	fmt.Println(len(requests), " request(s) stored.")

	mux.Handle("/SSEUpdate", sseHandler(serverCtx))
	mux.Handle("/requests", requestHandler(database))
	mux.Handle("/clear", clearHandler(database))
	mux.Handle("/", everythingElseHandler(serverCtx, database))

	return mux
}

func staticHandler(body []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Content-Type", contentType)
		io.Copy(w, bytes.NewReader(body))
	}
}

func sseHandler(serverCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		updateCh, tok := notification.New()
		defer notification.Close(tok)

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
	}
}

func requestHandler(db db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// show requests
		reqs, err := db.GetRequests(r.URL.Query().Get("url"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		if reqs == nil {
			reqs = []data.Request{}
		}
		b, err := json.MarshalIndent(reqs, "", " ")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		fmt.Fprintln(w, string(b))
	}
}

func clearHandler(db db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		err := db.DeleteRequests()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		go notification.Notify()
	}
}

func everythingElseHandler(serverCtx context.Context, db db.DB) http.HandlerFunc {
	type requestToRunner struct {
		req   data.Request
		idCh  chan<- int64
		errCh chan<- error
	}

	// request writer (with plenty of room to grow)
	writeCh := make(chan *requestToRunner, 1000)

	// write requests from a single thread (required by sqlite)
	go func() {
		var done bool
		for !done {
			select {
			case req := <-writeCh:
				id, err := db.WriteRequest(req.req)
				req.idCh <- id
				req.errCh <- err
				go notification.Notify()
			case <-serverCtx.Done():
				done = true
			}
		}
	}()

	return func(w http.ResponseWriter, r *http.Request) {
		// log request
		headers, err := json.Marshal(r.Header)
		if err != nil {
			panic(err)
		}

		var body strings.Builder
		io.Copy(&body, r.Body)

		req := data.Request{
			Timestamp: time.Now().UTC(),
			Method:    r.Method,
			URL:       r.URL.String(),
			Headers:   string(headers),
			Body:      body.String(),
		}

		// write request to db writing thread (and get id+errs back on channels)
		idChan := make(chan int64, 1)
		errChan := make(chan error, 1)
		writeCh <- &requestToRunner{req, idChan, errChan}

		if err := <-errChan; err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		req.ID = <-idChan
		reqBytes, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			panic(err)
		}

		w.Header().Add("Content-Type", "application/json")
		io.Copy(w, bytes.NewReader(reqBytes))
	}
}
