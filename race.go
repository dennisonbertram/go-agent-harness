package main

import "sync"

// Counter has an int field protected by a Mutex
// Fix: mutex added to prevent data races
type Counter struct {
	value int
	mu    sync.Mutex
}

// Increment increases the counter safely
func (c *Counter) Increment() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

// Value returns the current value safely
func (c *Counter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}
