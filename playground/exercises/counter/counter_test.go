package main

import (
	"sync"
	"testing"
)

func TestRace(t *testing.T) {
	c := &Counter{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()
	if c.Val() != 100 {
		t.Fatalf("got %d", c.Val())
	}
}
