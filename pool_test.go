package main

import (
	"sync"
	"testing"
	"time"
)

func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	pool := NewWorkerPool(8)
	defer pool.Shutdown()

	var wg sync.WaitGroup
	nJobs := 50
	results := make([]<-chan interface{}, nJobs)

	for i := 0; i < nJobs; i++ {
		i := i
		wg.Add(1)
		results[i] = pool.Submit(func() interface{} {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			return i * i
		})
	}

	wg.Wait()

	seen := make(map[int]bool)
	for i, rc := range results {
		val := (<-rc).(int)
		if val != i*i {
			t.Errorf("expected %d, got %d", i*i, val)
		}
		if seen[val] {
			t.Errorf("duplicate result %d", val)
		}
		seen[val] = true
	}
}

func TestWorkerPool_GracefulShutdown(t *testing.T) {
	pool := NewWorkerPool(4)
	nJobs := 20
	chans := make([]<-chan interface{}, nJobs)
	for i := range chans {
		i := i
		chans[i] = pool.Submit(func() interface{} {
			time.Sleep(5 * time.Millisecond)
			return i
		})
	}
	pool.Shutdown()
	// jobs submitted before shutdown must return
	for i, c := range chans {
		v := (<-c).(int)
		if v != i {
			t.Fatalf("expected %d, got %d", i, v)
		}
	}
	// further submits after Shutdown must panic or block forever (do not test)
}
