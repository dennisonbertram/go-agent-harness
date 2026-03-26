package tools

import (
	"context"
	"encoding/json"
	"strings"
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

type largeTranscriptReaderStub struct {
	messages []TranscriptMessage
}

func (s largeTranscriptReaderStub) Snapshot(limit int, includeTools bool) TranscriptSnapshot {
	return TranscriptSnapshot{
		RunID:          "run_parent",
		TenantID:       "tenant_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		Messages:       append([]TranscriptMessage(nil), s.messages...),
		GeneratedAt:    time.Now().UTC(),
	}
}

func TestBuildParentContextHandoffFromContext_TruncatesByMessagesRunesAndBytes(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("abcdef", 90)
	messages := make([]TranscriptMessage, 0, 20)
	for i := 0; i < 20; i++ {
		messages = append(messages, TranscriptMessage{
			Index:   int64(i),
			Role:    "user",
			Content: long,
		})
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyRunMetadata, RunMetadata{
		RunID:          "run_parent",
		TenantID:       "tenant_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
	})
	ctx = context.WithValue(ctx, ContextKeyTranscriptReader, largeTranscriptReaderStub{messages: messages})

	handoff, ok := BuildParentContextHandoffFromContext(ctx)
	if !ok {
		t.Fatal("expected non-empty handoff")
	}
	if got := handoff.ParentRunID; got != "run_parent" {
		t.Fatalf("ParentRunID = %q, want %q", got, "run_parent")
	}
	if got := handoff.ParentTenantID; got != "tenant_1" {
		t.Fatalf("ParentTenantID = %q, want %q", got, "tenant_1")
	}
	if got := handoff.ParentConversationID; got != "conv_1" {
		t.Fatalf("ParentConversationID = %q, want %q", got, "conv_1")
	}
	if got := handoff.ParentAgentID; got != "agent_1" {
		t.Fatalf("ParentAgentID = %q, want %q", got, "agent_1")
	}
	if len(handoff.Messages) != defaultParentContextMaxMessages {
		t.Fatalf("messages len = %d, want %d", len(handoff.Messages), defaultParentContextMaxMessages)
	}
	for _, msg := range handoff.Messages {
		if len([]rune(msg.Content)) > defaultParentContextMaxMessageRunes {
			t.Fatalf("message content len = %d runes, want <= %d", len([]rune(msg.Content)), defaultParentContextMaxMessageRunes)
		}
		if !strings.HasSuffix(msg.Content, "…") {
			t.Fatalf("expected truncated content marker in %q", msg.Content)
		}
	}
	payload, err := json.Marshal(handoff)
	if err != nil {
		t.Fatalf("marshal handoff: %v", err)
	}
	if len(payload) > defaultParentContextHandoffMaxBytes {
		t.Fatalf("serialized handoff = %d bytes, want <= %d", len(payload), defaultParentContextHandoffMaxBytes)
	}
	if handoff.Messages[0].Index != 4 {
		t.Fatalf("first message index = %d, want 4", handoff.Messages[0].Index)
	}
}

func TestRenderPromptWithParentContext_PrependsHandoffBeforeTask(t *testing.T) {
	t.Parallel()

	handoff := ParentContextHandoff{
		ParentRunID: "run_parent",
		Messages: []ParentContextMessage{{
			Index:   1,
			Role:    "user",
			Content: "Investigate the failing integration path.",
		}},
	}

	prompt := RenderPromptWithParentContext("child task", handoff)
	handoffIdx := strings.Index(prompt, parentContextHandoffHeader)
	taskHeaderIdx := strings.Index(prompt, parentContextTaskHeader)
	taskBodyIdx := strings.LastIndex(prompt, "child task")
	if handoffIdx == -1 || taskHeaderIdx == -1 || taskBodyIdx == -1 {
		t.Fatalf("expected handoff + task markers in prompt, got %q", prompt)
	}
	if !(handoffIdx < taskHeaderIdx && taskHeaderIdx < taskBodyIdx) {
		t.Fatalf("unexpected order in prompt: handoff=%d taskHeader=%d taskBody=%d", handoffIdx, taskHeaderIdx, taskBodyIdx)
	}

	trimmed := RenderPromptWithParentContext("  child task  ", ParentContextHandoff{})
	if trimmed != "child task" {
		t.Fatalf("expected trimmed task when no handoff, got %q", trimmed)
	}
}

func TestRenderParentContextHandoffBlock_SerializesJSON(t *testing.T) {
	t.Parallel()

	handoff := ParentContextHandoff{
		ParentRunID: "run_parent",
		Messages: []ParentContextMessage{{
			Index:   2,
			Role:    "assistant",
			Content: "Summary",
		}},
	}

	block := RenderParentContextHandoffBlock(handoff)
	if !strings.HasPrefix(block, parentContextHandoffHeader) {
		t.Fatalf("expected handoff header prefix, got %q", block)
	}
	jsonStart := strings.Index(block, "{")
	jsonEnd := strings.LastIndex(block, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		t.Fatalf("expected JSON block, got %q", block)
	}
	var decoded ParentContextHandoff
	if err := json.Unmarshal([]byte(block[jsonStart:jsonEnd+1]), &decoded); err != nil {
		t.Fatalf("unmarshal handoff block JSON: %v", err)
	}
	if decoded.ParentRunID != "run_parent" {
		t.Fatalf("decoded ParentRunID = %q, want %q", decoded.ParentRunID, "run_parent")
	}
}
