package main

import "testing"

func TestGetOrSet(t *testing.T) {
	c := NewCache()
	got := c.GetOrSet("x", "hello")
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}
