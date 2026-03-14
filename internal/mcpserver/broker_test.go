package mcpserver

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// T3: Publish to run_id delivers to subscribed channel within 100ms.
func TestBroker_PublishDelivered(t *testing.T) {
	b := NewBroker()
	ch, cancel := b.Subscribe("run-abc")
	defer cancel()

	n := Notification{Method: "run/event", Params: json.RawMessage(`{"run_id":"run-abc"}`)}
	b.Publish("run-abc", n)

	select {
	case got := <-ch:
		if got.Method != "run/event" {
			t.Errorf("expected method run/event, got %q", got.Method)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for notification")
	}
}

// T4: Publishing to run_id shows up on SubscribeAll channel.
func TestBroker_PublishShowsOnGlobalSub(t *testing.T) {
	b := NewBroker()
	ch, cancel := b.SubscribeAll()
	defer cancel()

	n := Notification{Method: "run/completed", Params: json.RawMessage(`{"run_id":"run-xyz","status":"completed"}`)}
	b.Publish("run-xyz", n)

	select {
	case got := <-ch:
		if got.Method != "run/completed" {
			t.Errorf("expected method run/completed, got %q", got.Method)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for notification on global subscriber")
	}
}

// T6: Two concurrent SubscribeAll channels both receive same notification.
func TestBroker_TwoConcurrentSubscribeAllBothReceive(t *testing.T) {
	b := NewBroker()

	ch1, cancel1 := b.SubscribeAll()
	defer cancel1()
	ch2, cancel2 := b.SubscribeAll()
	defer cancel2()

	n := Notification{Method: "run/event", Params: json.RawMessage(`{"event_type":"status_changed"}`)}
	b.PublishAll(n)

	timeout := time.After(100 * time.Millisecond)

	var got1, got2 bool
	for i := 0; i < 2; i++ {
		select {
		case <-ch1:
			got1 = true
		case <-ch2:
			got2 = true
		case <-timeout:
			t.Fatalf("timed out: ch1_received=%v ch2_received=%v", got1, got2)
		}
	}
	if !got1 {
		t.Error("ch1 did not receive notification")
	}
	if !got2 {
		t.Error("ch2 did not receive notification")
	}
}

// T7: Notification for run_id "A" has correct run_id in params.
func TestBroker_NotificationHasCorrectRunID(t *testing.T) {
	b := NewBroker()
	ch, cancel := b.Subscribe("run-A")
	defer cancel()

	params := map[string]any{"run_id": "run-A", "status": "running"}
	raw, _ := json.Marshal(params)
	n := Notification{Method: "run/event", Params: raw}
	b.Publish("run-A", n)

	select {
	case got := <-ch:
		var p map[string]any
		if err := json.Unmarshal(got.Params, &p); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if p["run_id"] != "run-A" {
			t.Errorf("expected run_id=run-A, got %v", p["run_id"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for notification")
	}
}

// TestBroker_CancelRemovesSubscription verifies cancel removes the subscription.
func TestBroker_CancelRemovesSubscription(t *testing.T) {
	b := NewBroker()

	_, cancel1 := b.Subscribe("run-cancel")
	_, cancel2 := b.SubscribeAll()

	if b.ActiveSubscriptions() != 2 {
		t.Errorf("expected 2 active subscriptions, got %d", b.ActiveSubscriptions())
	}

	cancel1()
	cancel2()

	if b.ActiveSubscriptions() != 0 {
		t.Errorf("expected 0 active subscriptions after cancel, got %d", b.ActiveSubscriptions())
	}
}

// TestBroker_NonBlockingDrop verifies that a full channel does not block Publish.
func TestBroker_NonBlockingDrop(t *testing.T) {
	b := NewBroker()
	ch, cancel := b.SubscribeAll()
	defer cancel()

	// Fill the channel buffer completely.
	n := Notification{Method: "run/event", Params: json.RawMessage(`{}`)}
	for i := 0; i < channelBufSize; i++ {
		b.PublishAll(n)
	}

	// This publish should not block even though channel is full.
	done := make(chan struct{})
	go func() {
		b.PublishAll(n)
		close(done)
	}()

	select {
	case <-done:
		// Good — did not block.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Publish blocked on full channel")
	}

	// Drain to avoid goroutine leaks.
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// TestBroker_ActiveSubscriptions verifies the count is correct.
func TestBroker_ActiveSubscriptions(t *testing.T) {
	b := NewBroker()

	if b.ActiveSubscriptions() != 0 {
		t.Errorf("expected 0, got %d", b.ActiveSubscriptions())
	}

	_, c1 := b.Subscribe("run-1")
	_, c2 := b.Subscribe("run-1")
	_, c3 := b.SubscribeAll()

	if b.ActiveSubscriptions() != 3 {
		t.Errorf("expected 3, got %d", b.ActiveSubscriptions())
	}

	c1()
	if b.ActiveSubscriptions() != 2 {
		t.Errorf("expected 2, got %d", b.ActiveSubscriptions())
	}
	c2()
	c3()
	if b.ActiveSubscriptions() != 0 {
		t.Errorf("expected 0, got %d", b.ActiveSubscriptions())
	}
}

// TestBroker_ConcurrentPublishSubscribe verifies race-free concurrent access.
func TestBroker_ConcurrentPublishSubscribe(t *testing.T) {
	b := NewBroker()
	var wg sync.WaitGroup

	// Start multiple publishers and subscribers concurrently.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ch, cancel := b.SubscribeAll()
			defer cancel()
			n := Notification{Method: "run/event", Params: json.RawMessage(`{}`)}
			b.PublishAll(n)
			// Drain any received notifications.
			select {
			case <-ch:
			default:
			}
		}(i)
	}

	wg.Wait()
}

// TestBroker_PerRunDoesNotDeliverToOtherRun verifies isolation.
func TestBroker_PerRunDoesNotDeliverToOtherRun(t *testing.T) {
	b := NewBroker()

	chA, cancelA := b.Subscribe("run-A")
	defer cancelA()
	chB, cancelB := b.Subscribe("run-B")
	defer cancelB()

	n := Notification{Method: "run/event", Params: json.RawMessage(`{"run_id":"run-A"}`)}
	b.Publish("run-A", n)

	// chA should receive.
	select {
	case <-chA:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("run-A subscriber did not receive notification")
	}

	// chB should NOT receive (it's for run-B).
	select {
	case unexpected := <-chB:
		t.Errorf("run-B subscriber received unexpected notification: %+v", unexpected)
	case <-time.After(20 * time.Millisecond):
		// Good — no cross-delivery.
	}
}
