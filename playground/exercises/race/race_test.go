package main

import (
	"sync"
	"testing"
)

func TestCounter_NoRace(t *testing.T) {
	c := &Counter{}
	wg := sync.WaitGroup{}
	concurrency := 100
	increments := 1000

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < increments; j++ {
				c.Increment()
			}
			wg.Done()
		}()
	}
	wg.Wait()

	expected := concurrency * increments
	if c.Value() != expected {
		t.Fatalf("expected %d, got %d", expected, c.Value())
	}
}
