package main

import "sync"

type Counter struct {
	mu sync.Mutex
	n  int
}

func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *Counter) Val() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}
