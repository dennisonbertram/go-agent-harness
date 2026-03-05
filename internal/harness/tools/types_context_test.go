package tools

import (
	"context"
	"testing"
	"time"
)

type transcriptReaderStub struct{}

func (transcriptReaderStub) Snapshot(limit int, includeTools bool) TranscriptSnapshot {
	return TranscriptSnapshot{
		RunID:       "run_1",
		Messages:    []TranscriptMessage{{Index: int64(limit), Role: "user"}},
		GeneratedAt: time.Now().UTC(),
	}
}

func TestContextHelpers(t *testing.T) {
	t.Parallel()

	if got := RunIDFromContext(nil); got != "" {
		t.Fatalf("expected empty run id for nil context, got %q", got)
	}
	if got := ToolCallIDFromContext(nil); got != "" {
		t.Fatalf("expected empty tool call id for nil context, got %q", got)
	}
	if _, ok := RunMetadataFromContext(nil); ok {
		t.Fatalf("expected no metadata for nil context")
	}
	if _, ok := TranscriptReaderFromContext(nil); ok {
		t.Fatalf("expected no transcript reader for nil context")
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyRunID, "run_from_key")
	ctx = context.WithValue(ctx, ContextKeyToolCallID, "call_1")
	ctx = context.WithValue(ctx, ContextKeyRunMetadata, RunMetadata{
		RunID:          "run_from_metadata",
		TenantID:       "tenant",
		ConversationID: "conversation",
		AgentID:        "agent",
	})
	reader := transcriptReaderStub{}
	ctx = context.WithValue(ctx, ContextKeyTranscriptReader, reader)

	if got := RunIDFromContext(ctx); got != "run_from_metadata" {
		t.Fatalf("expected metadata run id precedence, got %q", got)
	}
	if got := ToolCallIDFromContext(ctx); got != "call_1" {
		t.Fatalf("unexpected tool call id: %q", got)
	}
	meta, ok := RunMetadataFromContext(ctx)
	if !ok || meta.TenantID != "tenant" {
		t.Fatalf("unexpected metadata: %+v (ok=%v)", meta, ok)
	}
	gotReader, ok := TranscriptReaderFromContext(ctx)
	if !ok {
		t.Fatalf("expected transcript reader in context")
	}
	snap := gotReader.Snapshot(7, true)
	if snap.RunID != "run_1" || len(snap.Messages) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}
