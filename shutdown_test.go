package shutdown_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	shutdown "github.com/yutaqqq/go-graceful-shutdown"
)

func TestLIFOOrdering(t *testing.T) {
	var order []string
	s := shutdown.New()
	s.Register(shutdown.Handler{Name: "first", Fn: func(_ context.Context) error {
		order = append(order, "first")
		return nil
	}})
	s.Register(shutdown.Handler{Name: "second", Fn: func(_ context.Context) error {
		order = append(order, "second")
		return nil
	}})
	s.Register(shutdown.Handler{Name: "third", Fn: func(_ context.Context) error {
		order = append(order, "third")
		return nil
	}})

	if err := s.Shutdown(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"third", "second", "first"}
	if len(order) != len(want) {
		t.Fatalf("got %d calls, want %d", len(order), len(want))
	}
	for i, got := range order {
		if got != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestHandlerTimeout(t *testing.T) {
	s := shutdown.New()
	s.Register(shutdown.Handler{
		Name:    "slow",
		Timeout: 50 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
				return nil
			}
		},
	})

	err := s.Shutdown()
	if err == nil {
		t.Fatal("expected error from timed-out handler")
	}

	var he *shutdown.HandlerError
	if !errors.As(err, &he) {
		t.Fatalf("expected *HandlerError, got %T", err)
	}
	if he.Name != "slow" {
		t.Errorf("handler name: got %q, want %q", he.Name, "slow")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded wrapped in error, got %v", err)
	}
}

func TestMultipleErrors(t *testing.T) {
	errA := fmt.Errorf("error-a")
	errB := fmt.Errorf("error-b")

	s := shutdown.New()
	s.Register(shutdown.Handler{Name: "a", Fn: func(_ context.Context) error { return errA }})
	s.Register(shutdown.Handler{Name: "b", Fn: func(_ context.Context) error { return errB }})

	err := s.Shutdown()
	if err == nil {
		t.Fatal("expected errors, got nil")
	}
	if !errors.Is(err, errA) {
		t.Errorf("errA not found in joined error: %v", err)
	}
	if !errors.Is(err, errB) {
		t.Errorf("errB not found in joined error: %v", err)
	}
}

func TestHandlerError_Error(t *testing.T) {
	he := &shutdown.HandlerError{Name: "myhandler", Err: fmt.Errorf("boom")}
	got := he.Error()
	want := `handler "myhandler": boom`
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestHandlerError_Unwrap(t *testing.T) {
	sentinel := fmt.Errorf("sentinel")
	he := &shutdown.HandlerError{Name: "x", Err: sentinel}
	if !errors.Is(he, sentinel) {
		t.Error("Unwrap should expose the wrapped error via errors.Is")
	}
}

func TestListenContextCancel(t *testing.T) {
	var called bool
	s := shutdown.New()
	s.Register(shutdown.Handler{
		Name: "h",
		Fn: func(_ context.Context) error {
			called = true
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	if err := s.Listen(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called after context cancel")
	}
}

func TestListenSignal(t *testing.T) {
	var called bool
	s := shutdown.New()
	s.Register(shutdown.Handler{
		Name: "h",
		Fn: func(_ context.Context) error {
			called = true
			return nil
		},
	})

	done := make(chan error, 1)
	go func() {
		done <- s.Listen(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Listen did not return after SIGTERM")
	}
	if !called {
		t.Error("handler was not called after SIGTERM")
	}
}

func TestWithSignals(t *testing.T) {
	var called bool
	s := shutdown.New(shutdown.WithSignals(syscall.SIGHUP))
	s.Register(shutdown.Handler{
		Name: "h",
		Fn: func(_ context.Context) error {
			called = true
			return nil
		},
	})

	done := make(chan error, 1)
	go func() {
		done <- s.Listen(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	_ = syscall.Kill(os.Getpid(), syscall.SIGHUP)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Listen did not return after SIGHUP")
	}
	if !called {
		t.Error("handler was not called after SIGHUP")
	}
}

func TestRegisterChaining(t *testing.T) {
	s := shutdown.New()
	result := s.Register(shutdown.Handler{
		Name: "a",
		Fn:   func(_ context.Context) error { return nil },
	})
	if result != s {
		t.Error("Register should return the receiver for chaining")
	}
}

func TestEmptyHandlers(t *testing.T) {
	s := shutdown.New()
	if err := s.Shutdown(); err != nil {
		t.Fatalf("unexpected error with no handlers: %v", err)
	}
}

func TestDefaultTimeout(t *testing.T) {
	// A handler with zero Timeout must not panic or fail immediately.
	s := shutdown.New()
	s.Register(shutdown.Handler{
		// Timeout left at zero — should use the 30s default.
		Fn: func(_ context.Context) error { return nil },
	})
	if err := s.Shutdown(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
