// Package httpspy is the main package for the httpspy application
package httpspy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"www.github.com/spirozh/httpspy/internal/httpspy"
)

// App is the core struct
type App struct {
	Addr            string
	updateListeners *httpspy.TokenChanMap
}

// New creates a new instance of the app
func New(addr string) App {
	return App{
		Addr: addr,
	}
}

// Run is the entry to the app
func (app App) Run() {
	serverCtx, serverDone := context.WithCancel(context.Background())

	db := httpspy.OpenDb()
	defer db.Close()

	s := http.Server{
		Addr:        app.Addr,
		Handler:     httpspy.ServeMux(serverCtx, db),
		BaseContext: func(_ net.Listener) context.Context { return serverCtx },
	}
	go func() {
		fmt.Printf("--\naddr: %q\n", app.Addr)
		err := s.ListenAndServe()
		fmt.Println(err)
		serverDone()
	}()

	// shutdown on Interrupt or if server context is closed
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
