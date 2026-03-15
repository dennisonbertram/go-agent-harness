package main

import (
	"sync"
	"testing"
	"time"
)

func TestBasicPubSub(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("topic1")
	bus.Publish("topic1", "hello")
	select {
	case msg := <-ch:
		if msg != "hello" {
			t.Fatalf("expected 'hello', got %s", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestWildcardPubSub(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("user.*")
	bus.Publish("user.created", "created")
	bus.Publish("user.deleted", "deleted")
	msgs := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case msg := <-ch:
			msgs[msg] = true
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for wildcard message")
		}
	}
	if !msgs["created"] || !msgs["deleted"] {
		t.Fatal("did not receive all wildcard messages")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe("topic2")
	bus.Unsubscribe("topic2", ch)
	bus.Publish("topic2", "bye")
	select {
	case msg, ok := <-ch:
		if ok {
			t.Fatalf("expected closed channel, got %s", msg)
		}
	case <-time.After(200 * time.Millisecond):
		// ok: channel closed, no value
	}
}

func TestConcurrentPubSub(t *testing.T) {
	bus := NewEventBus()
	const npub = 5
	const nsub = 10
	var wg sync.WaitGroup
	chans := make([]<-chan string, nsub)
	for j := 0; j < nsub; j++ {
		chans[j] = bus.Subscribe("conc.topic")
	}
	wg.Add(npub)
	for i := 0; i < npub; i++ {
		go func(id int) {
			defer wg.Done()
			for k := 0; k < 10; k++ {
				bus.Publish("conc.topic", "msg")
			}
		}(i)
	}

	recv := make([]int, nsub)
	for i := 0; i < nsub; i++ {
		go func(idx int, ch <-chan string) {
			for m := 0; m < npub*10; m++ {
				<-ch
				recv[idx]++
			}
		}(i, chans[i])
	}
	wg.Wait()
	// Allow delivery
	select {
	case <-time.After(500 * time.Millisecond):
	}
	for i, ch := range chans {
		bus.Unsubscribe("conc.topic", ch)
		if recv[i] != npub*10 {
			t.Fatalf("subscriber %d: got %d msgs, want %d", i, recv[i], npub*10)
		}
	}
}
