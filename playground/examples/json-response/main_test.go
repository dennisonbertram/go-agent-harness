package main

import "testing"

func TestGetName(t *testing.T) {
	got := GetName(nil)
	if got != "" {
		t.Errorf("want empty, got %q", got)
	}
}
