package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Burst10At1RPS(t *testing.T) {
	hit := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
		w.Write([]byte("ok"))
	})

	rl := NewRateLimiter(1, handler)

	ts := httptest.NewServer(rl)
	defer ts.Close()

	results := make([]int, 10)
	for i := 0; i < 10; i++ {
		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		results[i] = resp.StatusCode
		_, _ = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}

	count429 := 0
	for _, code := range results {
		if code == http.StatusTooManyRequests {
			count429++
		}
	}
	if count429 == 0 {
		t.Errorf("expected at least one 429, got none: codes=%v", results)
	}
}
