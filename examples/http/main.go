//go:build ignore

// Example: HTTP server with graceful shutdown.
// Run with: go run main.go
// Stop with: Ctrl+C or kill -TERM <pid>
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	shutdown "github.com/yutaqqq/go-graceful-shutdown"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	s := shutdown.New()
	s.Register(shutdown.Handler{
		Name:    "http",
		Timeout: 10 * time.Second,
		Fn:      srv.Shutdown,
	})

	go func() {
		log.Println("listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http: %v", err)
		}
	}()

	log.Println("send SIGTERM or Ctrl+C to stop")
	if err := s.Listen(context.Background()); err != nil {
		log.Printf("shutdown errors: %v", err)
	}
	log.Println("bye")
}
