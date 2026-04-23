// Package shutdown provides a minimal, stdlib-only graceful shutdown orchestrator
// for Go applications. Handlers are invoked in LIFO order (last registered,
// first shutdown), matching the behaviour of defer.
//
// Example — HTTP server with a database connection pool:
//
//	s := shutdown.New()
//	s.Register(shutdown.Handler{Name: "db",   Timeout: 5*time.Second,  Fn: db.Close})
//	s.Register(shutdown.Handler{Name: "http", Timeout: 10*time.Second, Fn: srv.Shutdown})
//	// On SIGTERM/SIGINT: http shuts down first, then db.
//	if err := s.Listen(ctx); err != nil {
//	    log.Printf("shutdown errors: %v", err)
//	}
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const defaultTimeout = 30 * time.Second

// Handler describes one shutdown step.
type Handler struct {
	// Name identifies this handler in error messages.
	Name string

	// Timeout is the per-handler deadline. Zero uses a 30-second default.
	Timeout time.Duration

	// Fn is called during shutdown. The provided context expires after Timeout.
	Fn func(ctx context.Context) error
}

// HandlerError records which handler failed and the underlying error.
type HandlerError struct {
	Name string
	Err  error
}

// Error implements the error interface.
func (e *HandlerError) Error() string {
	return fmt.Sprintf("handler %q: %v", e.Name, e.Err)
}

// Unwrap allows errors.Is/As to inspect the underlying error.
func (e *HandlerError) Unwrap() error { return e.Err }

// Option configures a Shutdown orchestrator.
type Option func(*Shutdown)

// WithSignals replaces the default signal set (SIGTERM, SIGINT).
func WithSignals(sigs ...os.Signal) Option {
	return func(s *Shutdown) { s.sigs = sigs }
}

// Shutdown orchestrates graceful shutdown in LIFO order.
type Shutdown struct {
	mu       sync.Mutex
	handlers []Handler
	sigs     []os.Signal
}

// New creates a Shutdown orchestrator. By default it listens for SIGTERM and SIGINT.
func New(opts ...Option) *Shutdown {
	s := &Shutdown{
		sigs: []os.Signal{syscall.SIGTERM, syscall.SIGINT},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Register adds a handler. Returns the receiver for method chaining.
// Safe to call concurrently.
func (s *Shutdown) Register(h Handler) *Shutdown {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, h)
	return s
}

// Listen blocks until one of the configured signals is received or ctx is
// cancelled, then runs all registered handlers and returns any errors.
func (s *Shutdown) Listen(ctx context.Context) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, s.sigs...)
	defer signal.Stop(ch)

	select {
	case <-ctx.Done():
	case <-ch:
	}
	return s.run()
}

// Shutdown runs all registered handlers immediately without waiting for a signal.
// Handlers are invoked in LIFO order; all errors are collected and returned.
func (s *Shutdown) Shutdown() error {
	return s.run()
}

func (s *Shutdown) run() error {
	s.mu.Lock()
	handlers := make([]Handler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.Unlock()

	var errs []error
	for i := len(handlers) - 1; i >= 0; i-- {
		h := handlers[i]
		timeout := h.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}
		hCtx, cancel := context.WithTimeout(context.Background(), timeout)
		if err := h.Fn(hCtx); err != nil {
			errs = append(errs, &HandlerError{Name: h.Name, Err: err})
		}
		cancel()
	}
	return errors.Join(errs...)
}
