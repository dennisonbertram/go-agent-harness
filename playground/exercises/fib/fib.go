package main

// Fibonacci computes the n-th Fibonacci number using memoization, with no global mutations.
func Fibonacci(n int) int {
	if n < 0 {
		panic("negative input not allowed")
	}
	cache := make(map[int]int)
	var fib func(int) int
	fib = func(k int) int {
		if k < 2 {
			return k
		}
		if v, ok := cache[k]; ok {
			return v
		}
		res := fib(k-1) + fib(k-2)
		cache[k] = res
		return res
	}
	return fib(n)
}
