package core

// Direct unit tests for the compaction helpers (parseTurns, findCompactionBounds,
// the token estimators, transcriptMsgsToMaps).
//
// These moved here from the deleted duplicate tool package. The helpers they
// exercise live in core/compact_history.go, and core had no direct coverage of
// them — only of the tool handler that calls them.

import (
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

func TestParseTurns(t *testing.T) {
	t.Parallel()
	msgs := []tools.TranscriptMessage{
		{Role: "system", Content: "prompt"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "assistant", Content: "reading"},
		{Role: "tool", ToolCallID: "c1", Content: "data"},
		{Role: "user", Content: "bye"},
	}

	turns := parseTurns(msgs)
	if len(turns) != 5 {
		t.Fatalf("expected 5 turns, got %d", len(turns))
	}
	if turns[0].Kind != "system_prefix" {
		t.Errorf("turn 0: expected system_prefix, got %s", turns[0].Kind)
	}
	if turns[1].Kind != "user" {
		t.Errorf("turn 1: expected user, got %s", turns[1].Kind)
	}
	if turns[2].Kind != "assistant_text" {
		t.Errorf("turn 2: expected assistant_text, got %s", turns[2].Kind)
	}
	if turns[3].Kind != "assistant_tool" {
		t.Errorf("turn 3: expected assistant_tool, got %s", turns[3].Kind)
	}
	if turns[4].Kind != "user" {
		t.Errorf("turn 4: expected user, got %s", turns[4].Kind)
	}
}

func TestParseTurns_Empty(t *testing.T) {
	t.Parallel()
	turns := parseTurns(nil)
	if turns != nil {
		t.Errorf("expected nil turns for empty input, got %v", turns)
	}
}

func TestParseTurns_OrphanToolResult(t *testing.T) {
	t.Parallel()
	msgs := []tools.TranscriptMessage{
		{Role: "tool", ToolCallID: "orphan", Content: "orphan data"},
		{Role: "user", Content: "hello"},
	}

	turns := parseTurns(msgs)
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if turns[0].Kind != "assistant_tool" {
		t.Errorf("turn 0: expected assistant_tool for orphan, got %s", turns[0].Kind)
	}
	if turns[1].Kind != "user" {
		t.Errorf("turn 1: expected user, got %s", turns[1].Kind)
	}
}

func TestFindCompactionBounds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		turns      []turn
		keepLast   int
		wantPrefix int
		wantEnd    int
	}{
		{
			name: "basic",
			turns: []turn{
				{Kind: "system_prefix"},
				{Kind: "user"},
				{Kind: "assistant_tool"},
				{Kind: "user"},
				{Kind: "assistant_text"},
			},
			keepLast:   2,
			wantPrefix: 1,
			wantEnd:    3,
		},
		{
			name: "nothing to compact",
			turns: []turn{
				{Kind: "system_prefix"},
				{Kind: "user"},
				{Kind: "assistant_text"},
			},
			keepLast:   4,
			wantPrefix: 1,
			wantEnd:    1,
		},
		{
			name: "multiple system prefix",
			turns: []turn{
				{Kind: "system_prefix"},
				{Kind: "compact_summary"},
				{Kind: "user"},
				{Kind: "assistant_tool"},
				{Kind: "user"},
				{Kind: "assistant_text"},
			},
			keepLast:   2,
			wantPrefix: 2,
			wantEnd:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prefix, end := findCompactionBounds(tt.turns, tt.keepLast)
			if prefix != tt.wantPrefix {
				t.Errorf("prefixEnd = %d, want %d", prefix, tt.wantPrefix)
			}
			if end != tt.wantEnd {
				t.Errorf("compactEnd = %d, want %d", end, tt.wantEnd)
			}
		})
	}
}

func TestEstimateTextTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hi", 1},          // (2+3)/4 = 1
		{"hello", 2},       // (5+3)/4 = 2
		{"hello world", 3}, // (11+3)/4 = 3
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := estimateTextTokens(tt.input)
			if got != tt.want {
				t.Errorf("estimateTextTokens(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEstimateTranscriptTokens(t *testing.T) {
	t.Parallel()
	msgs := []tools.TranscriptMessage{
		{Content: "hello"}, // 2 tokens
		{Content: ""},      // 0 tokens
		{Content: "hi"},    // 1 token
	}
	got := estimateTranscriptTokens(msgs)
	if got != 3 {
		t.Errorf("estimateTranscriptTokens = %d, want 3", got)
	}
}

func TestTranscriptMsgsToMaps(t *testing.T) {
	t.Parallel()
	msgs := []tools.TranscriptMessage{
		{Role: "user", Content: "hello"},
		{Role: "tool", ToolCallID: "c1", Content: "data"},
		{Role: "system", Name: "compact_summary", Content: "summary"},
	}

	maps := transcriptMsgsToMaps(msgs)
	if len(maps) != 3 {
		t.Fatalf("expected 3 maps, got %d", len(maps))
	}
	if maps[0]["role"] != "user" {
		t.Error("expected user role")
	}
	if maps[1]["tool_call_id"] != "c1" {
		t.Error("expected tool_call_id")
	}
	if maps[2]["name"] != "compact_summary" {
		t.Error("expected name field")
	}
}
