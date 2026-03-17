package main

import "testing"

var data = func() []string {
	s := make([]string, 10000)
	for i := range s {
		s[i] = string(rune('a'+i%26)) + string(rune('0'+i%10))
	}
	return s
}()

func BenchmarkSlow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ContainsStr(data, "z9")
	}
}

func BenchmarkFast(b *testing.B) {
	idx := BuildIndex(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ContainsFast(idx, "z9")
	}
}
