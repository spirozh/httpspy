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
	fmt.Println(len(database.GetRequests()), " request(s) stored.")

	mux.Handle("/SSEUpdate", sseHandler(serverCtx, &database))
	mux.Handle("/clear", clearHandler(&database))
	mux.Handle("/", everythingElseHandler(serverCtx, &database))

	return mux
}

func staticHandler(body []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Content-Type", contentType)
		io.Copy(w, bytes.NewReader(body))
	}
}

type SSEEvent struct {
	Event string
	Data  []byte
}

func sseHandler(serverCtx context.Context, db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		updateCh, tok := notification.New()
		defer notification.Close(tok)

		// write headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		requestCtx := r.Context()

		requests := db.GetRequests()
		data, err := json.Marshal(requests)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(w, "event: all\ndata: %s\n\n", data)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		for {
			select {
			case sseEvent := <-updateCh:
				if sseEvent.Event != "" {
					fmt.Fprintf(w, "event: %s\n", sseEvent.Event)
				}
				if sseEvent.Data != nil {
					fmt.Fprintf(w, "data: %s\n", sseEvent.Data)
				}
				fmt.Fprintln(w)

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

func clearHandler(db *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		err := db.DeleteRequests()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}

		fmt.Fprintln(w, "cleared")

		go notification.NotifyClear()
	}
}

func everythingElseHandler(serverCtx context.Context, db *db.DB) http.HandlerFunc {

	// request writer (with plenty of room to grow)
	writeCh := make(chan data.Request, 1000)

	// write requests from a single thread (required by sqlite)
	go func() {
		var done bool
		for !done {
			select {
			case req := <-writeCh:
				err := db.WriteRequest(&req)
				if err != nil {
					panic("failed to write")
				}
				notification.NotifyNew(req)
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

		reqCh, token := notification.New()
		defer notification.Close(token)

		req := data.Request{
			Timestamp: time.Now().UTC(),
			Method:    r.Method,
			URL:       r.URL.String(),
			Headers:   string(headers),
			Body:      body.String(),
		}

		// write request to db writing thread (and get request with id back on channel)
		writeCh <- req

		// wait for request to come back
		reqs := <-reqCh

		w.Header().Add("Content-Type", "application/json")
		io.Copy(w, bytes.NewReader(reqs.Data))
		fmt.Fprintln(w)
	}
}
