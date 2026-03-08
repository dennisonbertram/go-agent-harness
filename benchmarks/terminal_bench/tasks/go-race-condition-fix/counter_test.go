package main

import (
	"sync"
	"testing"
)

func TestCounterConcurrentIncrement(t *testing.T) {
	c := NewCounter()
	var wg sync.WaitGroup
	n := 100
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Increment()
			}
		}()
	}
	wg.Wait()
	if got := c.Value(); got != n*1000 {
		t.Errorf("expected %d, got %d", n*1000, got)
	}
}

func TestCounterReset(t *testing.T) {
	c := NewCounter()
	c.Increment()
	c.Increment()
	c.Reset()
	if got := c.Value(); got != 0 {
		t.Errorf("expected 0 after reset, got %d", got)
	}
}
