package main

import "testing"

func TestFibonacci(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int
	}{
		{"F(0)", 0, 0},
		{"F(1)", 1, 1},
		{"F(2)", 2, 1},
		{"F(5)", 5, 5},
		{"F(10)", 10, 55},
		{"F(20)", 20, 6765},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Fibonacci(tt.n)
			if got != tt.want {
				t.Errorf("Fibonacci(%d) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
}
