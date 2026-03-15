package main

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestClosedToOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)
	errFn := func() error { return errors.New("fail") }
	for i := 0; i < 3; i++ {
		cb.Execute(errFn)
	}
	if cb.State() != Open {
		t.Fatalf("Expected state Open, got %v", cb.State())
	}
}

func TestOpenToHalfOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(2, 1, 50*time.Millisecond)
	cb.Execute(func() error { return errors.New("fail") })
	cb.Execute(func() error { return errors.New("fail") })
	if cb.State() != Open {
		t.Fatalf("Expected Open state")
	}
	time.Sleep(60 * time.Millisecond)
	if cb.State() != HalfOpen {
		t.Fatalf("Expected HalfOpen state, got %v", cb.State())
	}
}

func TestHalfOpenToClosedTransition(t *testing.T) {
	cb := NewCircuitBreaker(1, 2, 20*time.Millisecond)
	cb.setState(HalfOpen)
	cb.Execute(func() error { return nil })
	if cb.State() != HalfOpen {
		t.Fatalf("Expected still HalfOpen, got %v", cb.State())
	}
	cb.Execute(func() error { return nil })
	if cb.State() != Closed {
		t.Fatalf("Expected transition to Closed, got %v", cb.State())
	}
}

func TestHalfOpenToOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(1, 1, 20*time.Millisecond)
	cb.setState(HalfOpen)
	err := errors.New("fail")
	cb.Execute(func() error { return err })
	if cb.State() != Open {
		t.Fatalf("Expected transition to Open after error in HalfOpen, got %v", cb.State())
	}
}

func TestOpenDeniesExecute(t *testing.T) {
	cb := NewCircuitBreaker(1, 1, 500*time.Millisecond)
	cb.setState(Open)
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("Expected ErrCircuitOpen but got: %v", err)
	}
}

func TestConcurrentExecute(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 50*time.Millisecond)
	var wg sync.WaitGroup
	failureFn := func() error { return errors.New("fail") }
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Execute(failureFn)
		}()
	}
	wg.Wait()
	if cb.State() != Open {
		t.Fatalf("Expected state Open after concurrent failures, got %v", cb.State())
	}
}
