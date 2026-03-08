package main

// Counter tracks a single integer value.
// BUG: This implementation is NOT safe for concurrent access.
type Counter struct {
	value int
}

// NewCounter returns a zero-valued Counter.
func NewCounter() *Counter {
	return &Counter{}
}

// Increment adds one to the counter.
func (c *Counter) Increment() {
	c.value++
}

// Value returns the current count.
func (c *Counter) Value() int {
	return c.value
}

// Reset sets the counter back to zero.
func (c *Counter) Reset() {
	c.value = 0
}
