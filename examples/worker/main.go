//go:build ignore

// Example: background worker with graceful shutdown.
// Demonstrates LIFO ordering: worker stops before the database connection closes.
// Run with: go run main.go
// Stop with: Ctrl+C or kill -TERM <pid>
package main

import (
	"context"
	"log"
	"time"

	shutdown "github.com/yutaqqq/go-graceful-shutdown"
)

func main() {
	workerStop := make(chan struct{})
	workerDone := make(chan struct{})

	s := shutdown.New()

	// Register DB first — it will shut down last (LIFO).
	s.Register(shutdown.Handler{
		Name:    "db",
		Timeout: 5 * time.Second,
		Fn: func(_ context.Context) error {
			log.Println("db: closing connection pool")
			return nil
		},
	})

	// Register worker second — it will shut down first (LIFO).
	s.Register(shutdown.Handler{
		Name:    "worker",
		Timeout: 5 * time.Second,
		Fn: func(_ context.Context) error {
			log.Println("worker: signalling stop")
			close(workerStop)
			<-workerDone
			log.Println("worker: stopped")
			return nil
		},
	})

	go func() {
		defer close(workerDone)
		for {
			select {
			case <-workerStop:
				return
			case <-time.After(1 * time.Second):
				log.Println("worker: tick")
			}
		}
	}()

	log.Println("running — send SIGTERM or Ctrl+C to stop")
	if err := s.Listen(context.Background()); err != nil {
		log.Printf("shutdown errors: %v", err)
	}
	log.Println("bye")
}
