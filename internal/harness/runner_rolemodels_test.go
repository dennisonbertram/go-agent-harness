package harness

import (
	"context"
	"testing"
)

// TestResolveRoleModels_NoOverride verifies that with no RoleModels configured,
// resolveRoleModels returns an empty RoleModels (both fields empty).
func TestResolveRoleModels_NoOverride(t *testing.T) {
	t.Parallel()

	r := NewRunner(nil, nil, RunnerConfig{})
	req := RunRequest{Prompt: "hello"}

	got := r.resolveRoleModels(req)
	if got.Primary != "" {
		t.Errorf("Primary: expected empty, got %q", got.Primary)
	}
	if got.Summarizer != "" {
		t.Errorf("Summarizer: expected empty, got %q", got.Summarizer)
	}
}

// TestResolveRoleModels_ConfigOnly verifies that runner-level RoleModels config
// is returned when no per-request RoleModels are set.
func TestResolveRoleModels_ConfigOnly(t *testing.T) {
	t.Parallel()

	r := NewRunner(nil, nil, RunnerConfig{
		RoleModels: RoleModels{
			Primary:    "gpt-4.1",
			Summarizer: "gpt-4.1-mini",
		},
	})
	req := RunRequest{Prompt: "hello"}

	got := r.resolveRoleModels(req)
	if got.Primary != "gpt-4.1" {
		t.Errorf("Primary: expected %q, got %q", "gpt-4.1", got.Primary)
	}
	if got.Summarizer != "gpt-4.1-mini" {
		t.Errorf("Summarizer: expected %q, got %q", "gpt-4.1-mini", got.Summarizer)
	}
}

// TestResolveRoleModels_RequestOverridesConfig verifies that per-request
// RoleModels take precedence over runner-level config.
func TestResolveRoleModels_RequestOverridesConfig(t *testing.T) {
	t.Parallel()

	r := NewRunner(nil, nil, RunnerConfig{
		RoleModels: RoleModels{
			Primary:    "gpt-4.1",
			Summarizer: "gpt-4.1-mini",
		},
	})
	req := RunRequest{
		Prompt: "hello",
		RoleModels: &RoleModels{
			Primary:    "claude-3-5-sonnet-20241022",
			Summarizer: "claude-3-haiku-20240307",
		},
	}

	got := r.resolveRoleModels(req)
	if got.Primary != "claude-3-5-sonnet-20241022" {
		t.Errorf("Primary: expected %q, got %q", "claude-3-5-sonnet-20241022", got.Primary)
	}
	if got.Summarizer != "claude-3-haiku-20240307" {
		t.Errorf("Summarizer: expected %q, got %q", "claude-3-haiku-20240307", got.Summarizer)
	}
}

// TestResolveRoleModels_PartialRequestOverride verifies that only non-empty
// request fields override the config, leaving the other config field intact.
func TestResolveRoleModels_PartialRequestOverride(t *testing.T) {
	t.Parallel()

	r := NewRunner(nil, nil, RunnerConfig{
		RoleModels: RoleModels{
			Primary:    "gpt-4.1",
			Summarizer: "gpt-4.1-mini",
		},
	})
	req := RunRequest{
		Prompt: "hello",
		// Only override Primary; Summarizer is empty (falls back to config).
		RoleModels: &RoleModels{
			Primary: "o3-mini",
		},
	}

	got := r.resolveRoleModels(req)
	if got.Primary != "o3-mini" {
		t.Errorf("Primary: expected %q, got %q", "o3-mini", got.Primary)
	}
	// Summarizer should fall back to the config value.
	if got.Summarizer != "gpt-4.1-mini" {
		t.Errorf("Summarizer: expected %q, got %q", "gpt-4.1-mini", got.Summarizer)
	}
}

// TestResolveRoleModels_NilRequestRoleModels verifies that a nil RoleModels
// pointer in the request does not override config values.
func TestResolveRoleModels_NilRequestRoleModels(t *testing.T) {
	t.Parallel()

	r := NewRunner(nil, nil, RunnerConfig{
		RoleModels: RoleModels{
			Primary:    "gpt-4.1",
			Summarizer: "gpt-4.1-mini",
		},
	})
	req := RunRequest{
		Prompt:     "hello",
		RoleModels: nil, // explicitly nil
	}

	got := r.resolveRoleModels(req)
	if got.Primary != "gpt-4.1" {
		t.Errorf("Primary: expected %q, got %q", "gpt-4.1", got.Primary)
	}
	if got.Summarizer != "gpt-4.1-mini" {
		t.Errorf("Summarizer: expected %q, got %q", "gpt-4.1-mini", got.Summarizer)
	}
}

// TestSummarizeMessages_RoleModelOverride verifies that when RoleModels.Summarizer
// is configured, SummarizeMessages uses that model instead of DefaultModel.
func TestSummarizeMessages_RoleModelOverride(t *testing.T) {
	t.Parallel()

	cp := &capturingProvider{
		turns: []CompletionResult{{Content: "brief summary"}},
	}
	r := NewRunner(cp, nil, RunnerConfig{
		DefaultModel: "gpt-4.1",
		RoleModels: RoleModels{
			Summarizer: "gpt-4.1-mini",
		},
	})

	_, err := r.SummarizeMessages(context.Background(), []Message{
		{Role: "user", Content: "tell me something"},
		{Role: "assistant", Content: "something interesting"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cp.calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(cp.calls))
	}
	req := cp.calls[0]
	if req.Model != "gpt-4.1-mini" {
		t.Errorf("expected summarizer model %q, got %q", "gpt-4.1-mini", req.Model)
	}
}

// TestSummarizeMessages_NoRoleModelFallsBackToDefault verifies backward
// compatibility: when no Summarizer role model is set, DefaultModel is used.
func TestSummarizeMessages_NoRoleModelFallsBackToDefault(t *testing.T) {
	t.Parallel()

	cp := &capturingProvider{
		turns: []CompletionResult{{Content: "brief summary"}},
	}
	r := NewRunner(cp, nil, RunnerConfig{
		DefaultModel: "gpt-4.1",
		// No RoleModels configured.
	})

	_, err := r.SummarizeMessages(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cp.calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(cp.calls))
	}
	req := cp.calls[0]
	if req.Model != "gpt-4.1" {
		t.Errorf("expected default model %q, got %q", "gpt-4.1", req.Model)
	}
}

// TestSummarizeMessages_NoRoleModelNoDefaultFallsBackToHardcoded verifies that
// when neither RoleModels.Summarizer nor DefaultModel is set, the hardcoded
// fallback "gpt-4.1-mini" is used.
func TestSummarizeMessages_NoRoleModelNoDefaultFallsBackToHardcoded(t *testing.T) {
	t.Parallel()

	cp := &capturingProvider{
		turns: []CompletionResult{{Content: "brief summary"}},
	}
	r := NewRunner(cp, nil, RunnerConfig{
		// No DefaultModel and no RoleModels.
	})

	_, err := r.SummarizeMessages(context.Background(), []Message{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cp.calls) != 1 {
		t.Fatalf("expected 1 provider call, got %d", len(cp.calls))
	}
	req := cp.calls[0]
	if req.Model != "gpt-4.1-mini" {
		t.Errorf("expected hardcoded fallback %q, got %q", "gpt-4.1-mini", req.Model)
	}
}

// TestRoleModels_JSONRoundTrip verifies that RoleModels serializes to and from
// JSON with the expected field names (backward-compatible with omitempty).
func TestRoleModels_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	rm := RoleModels{
		Primary:    "gpt-4.1",
		Summarizer: "gpt-4.1-mini",
	}

	got := mustJSON(rm)
	if got != `{"primary":"gpt-4.1","summarizer":"gpt-4.1-mini"}` {
		t.Errorf("JSON mismatch: got %s", got)
	}
}

// TestRoleModels_JSONOmitempty verifies that empty RoleModels serializes to "{}".
func TestRoleModels_JSONOmitempty(t *testing.T) {
	t.Parallel()

	rm := RoleModels{}
	got := mustJSON(rm)
	if got != "{}" {
		t.Errorf("expected {}, got %s", got)
	}
}
