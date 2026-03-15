package testhelpers_test

import (
	"context"
	"testing"
	"time"

	"go-agent-harness/cmd/harnesscli/tui/testhelpers"
)

func TestTUI007_MockSSERelayEmitsEvents(t *testing.T) {
	events := []testhelpers.SSEEvent{
		{Event: "message", Data: `{"type":"assistant.message.delta","payload":{"delta":"Hello"}}`},
		{Event: "message", Data: `{"type":"run.completed","payload":{}}`},
	}
	srv := testhelpers.NewMockSSEServer(events)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	received, err := testhelpers.CollectSSEEvents(ctx, srv.URL+"/events")
	if err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 {
		t.Errorf("expected 2 events, got %d", len(received))
	}
	if received[0].Event != "message" {
		t.Errorf("expected event[0].Event = %q, got %q", "message", received[0].Event)
	}
	if received[0].Data != events[0].Data {
		t.Errorf("expected event[0].Data = %q, got %q", events[0].Data, received[0].Data)
	}
	if received[1].Data != events[1].Data {
		t.Errorf("expected event[1].Data = %q, got %q", events[1].Data, received[1].Data)
	}
}

func TestTUI007_SnapshotRoundTrip(t *testing.T) {
	td := t.TempDir()
	content := "hello TUI snapshot\n"
	testhelpers.WriteGolden(t, td, "test-snapshot", content)
	got := testhelpers.ReadGolden(t, td, "test-snapshot")
	if got != content {
		t.Errorf("golden round-trip: got %q, want %q", got, content)
	}
}

func TestTUI007_ZeroEventStream(t *testing.T) {
	srv := testhelpers.NewMockSSEServer(nil)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	received, err := testhelpers.CollectSSEEvents(ctx, srv.URL+"/events")
	if err != nil {
		t.Fatal(err)
	}
	if len(received) != 0 {
		t.Errorf("expected 0 events, got %d", len(received))
	}
}

func TestTUI007_AssertGoldenCreate(t *testing.T) {
	td := t.TempDir()
	content := "new golden content\n"
	// First call with update=true should create the file
	testhelpers.AssertGolden(t, td, "assert-create", content, true)
	// Read it back
	got := testhelpers.ReadGolden(t, td, "assert-create")
	if got != content {
		t.Errorf("AssertGolden create: got %q, want %q", got, content)
	}
}

func TestTUI007_AssertGoldenMatch(t *testing.T) {
	td := t.TempDir()
	content := "matching content\n"
	testhelpers.WriteGolden(t, td, "assert-match", content)
	// Should not fail when content matches
	testhelpers.AssertGolden(t, td, "assert-match", content, false)
}

func TestTUI007_LargeEventStream(t *testing.T) {
	var events []testhelpers.SSEEvent
	for i := 0; i < 100; i++ {
		events = append(events, testhelpers.SSEEvent{
			Event: "message",
			Data:  `{"type":"assistant.message.delta","payload":{"delta":"chunk"}}`,
		})
	}
	srv := testhelpers.NewMockSSEServer(events)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	received, err := testhelpers.CollectSSEEvents(ctx, srv.URL+"/events")
	if err != nil {
		t.Fatal(err)
	}
	if len(received) != 100 {
		t.Errorf("expected 100 events, got %d", len(received))
	}
}

func TestTUI007_SSEEventFields(t *testing.T) {
	events := []testhelpers.SSEEvent{
		{Event: "status", Data: `{"type":"run.started","payload":{}}`},
		{Event: "error", Data: `{"type":"run.failed","payload":{"error":"boom"}}`},
	}
	srv := testhelpers.NewMockSSEServer(events)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	received, err := testhelpers.CollectSSEEvents(ctx, srv.URL+"/events")
	if err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if received[0].Event != "status" {
		t.Errorf("expected event type %q, got %q", "status", received[0].Event)
	}
	if received[1].Event != "error" {
		t.Errorf("expected event type %q, got %q", "error", received[1].Event)
	}
}
