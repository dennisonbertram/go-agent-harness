package harness

import (
	"testing"
)

func TestEventTypeConstants(t *testing.T) {
	// Spot-check that constants match their string values
	tests := []struct {
		got  EventType
		want string
	}{
		{EventRunStarted, "run.started"},
		{EventRunCompleted, "run.completed"},
		{EventRunFailed, "run.failed"},
		{EventToolCallStarted, "tool.call.started"},
		{EventAssistantMessageDelta, "assistant.message.delta"},
		{EventUsageDelta, "usage.delta"},
		{EventHookStarted, "hook.started"},
		{EventMemoryObserveStarted, "memory.observe.started"},
		{EventAssistantThinkingDelta, "assistant.thinking.delta"},
		{EventProviderResolved, "provider.resolved"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("EventType %q != %q", tt.got, tt.want)
		}
	}
}

func TestIsTerminalEvent(t *testing.T) {
	tests := []struct {
		event EventType
		want  bool
	}{
		{EventRunCompleted, true},
		{EventRunFailed, true},
		{EventRunStarted, false},
		{EventLLMTurnRequested, false},
		{EventType(""), false},
		{EventType("unknown.event"), false},
	}
	for _, tt := range tests {
		got := IsTerminalEvent(tt.event)
		if got != tt.want {
			t.Errorf("IsTerminalEvent(%q) = %v, want %v", tt.event, got, tt.want)
		}
	}
}

func TestAllEventTypes_Count(t *testing.T) {
	all := AllEventTypes()
	if len(all) != 33 {
		t.Errorf("AllEventTypes() returned %d events, want 33", len(all))
	}
	// Verify no duplicates
	seen := make(map[EventType]bool)
	for _, et := range all {
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}
}

func TestRunCompletedPayload_RoundTrip(t *testing.T) {
	orig := RunCompletedPayload{
		Output:      "done",
		UsageTotals: map[string]any{"prompt_tokens": float64(100)},
		CostTotals:  map[string]any{"total_usd": float64(0.01)},
	}
	payload := orig.ToPayload()
	parsed, err := ParseRunCompletedPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Output != orig.Output {
		t.Errorf("Output = %q, want %q", parsed.Output, orig.Output)
	}
}

func TestRunFailedPayload_RoundTrip(t *testing.T) {
	orig := RunFailedPayload{
		Error:       "something broke",
		UsageTotals: map[string]any{"prompt_tokens": float64(50)},
	}
	payload := orig.ToPayload()
	parsed, err := ParseRunFailedPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Error != orig.Error {
		t.Errorf("Error = %q, want %q", parsed.Error, orig.Error)
	}
}

func TestParseEventID(t *testing.T) {
	tests := []struct {
		id      string
		wantRun string
		wantSeq uint64
		wantErr bool
	}{
		{"run_1:0", "run_1", 0, false},
		{"run_1:7", "run_1", 7, false},
		{"run_99:123", "run_99", 123, false},
		{"", "", 0, true},
		{"no-colon", "", 0, true},
		{"trailing:", "", 0, true},
		{"run_1:abc", "", 0, true},
		{"run_1:-1", "", 0, true},
		{":0", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			runID, seq, err := ParseEventID(tt.id)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseEventID(%q) expected error, got runID=%q seq=%d", tt.id, runID, seq)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseEventID(%q) unexpected error: %v", tt.id, err)
			}
			if runID != tt.wantRun {
				t.Errorf("ParseEventID(%q) runID = %q, want %q", tt.id, runID, tt.wantRun)
			}
			if seq != tt.wantSeq {
				t.Errorf("ParseEventID(%q) seq = %d, want %d", tt.id, seq, tt.wantSeq)
			}
		})
	}
}

func TestUsageDeltaPayload_RoundTrip(t *testing.T) {
	orig := UsageDeltaPayload{
		Step:              1,
		UsageStatus:       "provider_reported",
		CostStatus:        "available",
		TurnCostUSD:       0.001,
		CumulativeCostUSD: 0.003,
	}
	payload := orig.ToPayload()
	parsed, err := ParseUsageDeltaPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Step != orig.Step {
		t.Errorf("Step = %d, want %d", parsed.Step, orig.Step)
	}
	if parsed.CostStatus != orig.CostStatus {
		t.Errorf("CostStatus = %q, want %q", parsed.CostStatus, orig.CostStatus)
	}
}
