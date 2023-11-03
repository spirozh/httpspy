// Package app is ...
package app

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"os"
	"os/signal"
	"time"

	handler "www.github.com/spirozh/httpspy/internal/handler"
)

// Run is the entry to the app
func Run(addr string) {
	serverCtx, serverDone := context.WithCancel(context.Background())
	doneChan := make(chan struct{})

	s := http.Server{
		Addr:        addr,
		Handler:     handler.New(serverCtx, doneChan),
		BaseContext: func(_ net.Listener) context.Context { return serverCtx },
	}
	go func() {
		fmt.Printf("--\naddr: %q\n", addr)
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

	<-doneChan

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second)
	defer ctxCancel()
	s.Shutdown(ctx)

}
