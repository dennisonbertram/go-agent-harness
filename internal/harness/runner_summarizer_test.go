package harness

import (
	"context"
	"errors"
	"testing"
)

func TestNewMessageSummarizer(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{Content: "ok"}}}
	r := NewRunner(provider, nil, RunnerConfig{})
	s := r.NewMessageSummarizer()
	if s == nil {
		t.Fatal("NewMessageSummarizer returned nil")
	}
}

func TestRunnerMessageSummarizer_Adapts(t *testing.T) {
	t.Parallel()

	cp := &capturingProvider{
		turns: []CompletionResult{{Content: "test summary"}},
	}
	r := NewRunner(cp, nil, RunnerConfig{DefaultModel: "test-model"})
	s := r.NewMessageSummarizer()

	input := []map[string]any{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi there", "name": "bot"},
		{"role": "tool", "content": "result", "tool_call_id": "call-1"},
	}

	summary, err := s.SummarizeMessages(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "test summary" {
		t.Fatalf("expected %q, got %q", "test summary", summary)
	}

	// Verify the provider received the correct messages
	if len(cp.calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(cp.calls))
	}
	req := cp.calls[0]
	if req.Model != "test-model" {
		t.Fatalf("expected model %q, got %q", "test-model", req.Model)
	}
	// Input has 3 messages + the appended summarization prompt = 4 total
	if len(req.Messages) != 4 {
		t.Fatalf("expected 4 messages in request, got %d", len(req.Messages))
	}

	// Check the converted messages match the input
	if req.Messages[0].Role != "user" || req.Messages[0].Content != "hello" {
		t.Fatalf("message 0 mismatch: %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "assistant" || req.Messages[1].Content != "hi there" || req.Messages[1].Name != "bot" {
		t.Fatalf("message 1 mismatch: %+v", req.Messages[1])
	}
	if req.Messages[2].Role != "tool" || req.Messages[2].Content != "result" || req.Messages[2].ToolCallID != "call-1" {
		t.Fatalf("message 2 mismatch: %+v", req.Messages[2])
	}
	// Last message is the summarization prompt
	if req.Messages[3].Role != "user" {
		t.Fatalf("expected last message to be user role, got %q", req.Messages[3].Role)
	}
}

func TestRunnerMessageSummarizer_ProviderError(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("provider failure")
	ep := &errorProvider{err: providerErr}
	r := NewRunner(ep, nil, RunnerConfig{})
	s := r.NewMessageSummarizer()

	_, err := s.SummarizeMessages(context.Background(), []map[string]any{
		{"role": "user", "content": "hello"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, providerErr) {
		t.Fatalf("expected provider error, got: %v", err)
	}
}

func TestRunnerSummarizeMessages_EmptySummary(t *testing.T) {
	t.Parallel()

	// Provider returns empty content — should produce an error
	provider := &stubProvider{turns: []CompletionResult{{Content: "   "}}}
	r := NewRunner(provider, nil, RunnerConfig{})

	_, err := r.SummarizeMessages(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error for empty summary")
	}
	if err.Error() != "empty summary from provider" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunnerGetSummarizer_NilProvider(t *testing.T) {
	t.Parallel()

	r := &Runner{}
	if got := r.GetSummarizer(); got != nil {
		t.Fatalf("expected nil summarizer for nil provider, got %T", got)
	}
}

func TestRunnerGetSummarizer_WithProvider(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{turns: []CompletionResult{{Content: "summary"}}}
	r := NewRunner(provider, nil, RunnerConfig{})
	if got := r.GetSummarizer(); got == nil {
		t.Fatal("expected non-nil summarizer when provider is set")
	}
}

func TestRunnerSummarizeMessages_NilProvider(t *testing.T) {
	t.Parallel()

	r := &Runner{}
	_, err := r.SummarizeMessages(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
	if err.Error() != "provider not configured" {
		t.Fatalf("unexpected error message: %v", err)
	}
}
