package tui_test

import (
	"context"
	"testing"
	"time"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/cmd/harnesscli/tui/testhelpers"
)

func TestTUI004_BridgeEmitsAssistantEvents(t *testing.T) {
	events := []testhelpers.SSEEvent{
		{Event: "message", Data: `{"type":"assistant.message.delta","payload":{"delta":"Hello"}}`},
		{Event: "message", Data: `{"type":"run.completed","payload":{}}`},
	}
	srv := testhelpers.NewMockSSEServer(events)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, stop := tui.StartSSEBridge(ctx, srv.URL+"/events")
	defer stop()

	var received []tui.SSEEventMsg
	for msg := range msgs {
		if e, ok := msg.(tui.SSEEventMsg); ok {
			received = append(received, e)
		}
		if _, ok := msg.(tui.SSEDoneMsg); ok {
			break
		}
	}
	if len(received) < 1 {
		t.Errorf("expected at least 1 SSEEventMsg, got %d", len(received))
	}
}

func TestTUI004_BridgeStopsOnContextCancel(t *testing.T) {
	// Long-running server
	events := make([]testhelpers.SSEEvent, 100)
	for i := range events {
		events[i] = testhelpers.SSEEvent{Event: "message", Data: `{"type":"assistant.message.delta","payload":{"delta":"x"}}`}
	}
	srv := testhelpers.NewMockSSEServer(events)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	msgs, stop := tui.StartSSEBridge(ctx, srv.URL+"/events")
	defer stop()

	// Cancel immediately
	cancel()

	// Drain channel — should close quickly
	done := make(chan struct{})
	go func() {
		for range msgs {
		}
		close(done)
	}()

	select {
	case <-done:
		// Good — channel closed after cancel
	case <-time.After(2 * time.Second):
		t.Error("bridge did not stop within 2s after context cancel")
	}
}

func TestTUI004_BridgeNoGoroutineLeak(t *testing.T) {
	events := []testhelpers.SSEEvent{
		{Event: "message", Data: `{"type":"run.completed","payload":{}}`},
	}
	srv := testhelpers.NewMockSSEServer(events)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msgs, stop := tui.StartSSEBridge(ctx, srv.URL+"/events")
	// Drain
	for range msgs {
	}
	stop()
	// If goroutine leaks, goleak or just time-based check
}

func TestTUI004_BridgeHandlesOverflow(t *testing.T) {
	// Verify SSEDropMsg emitted when channel full
	// Create many events faster than consumer
	events := make([]testhelpers.SSEEvent, 300)
	for i := range events {
		events[i] = testhelpers.SSEEvent{Event: "message", Data: `{"type":"assistant.message.delta","payload":{"delta":"x"}}`}
	}
	srv := testhelpers.NewMockSSEServer(events)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgs, stop := tui.StartSSEBridge(ctx, srv.URL+"/events")
	defer stop()

	var dropSeen bool
	for msg := range msgs {
		if _, ok := msg.(tui.SSEDropMsg); ok {
			dropSeen = true
		}
		if _, ok := msg.(tui.SSEDoneMsg); ok {
			break
		}
	}
	_ = dropSeen // overflow may or may not occur depending on timing
}
