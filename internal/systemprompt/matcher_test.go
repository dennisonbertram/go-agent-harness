package systemprompt

import "testing"

func TestResolveFallsBackToDefaultProfile(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	out, err := engine.Resolve(ResolveRequest{Model: "not-mapped-model"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if out.ResolvedModelProfile != "default" {
		t.Fatalf("expected default profile, got %q", out.ResolvedModelProfile)
	}
	if !out.ModelFallback {
		t.Fatalf("expected model fallback true")
	}
}

func TestResolveUsesExplicitPromptProfile(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	out, err := engine.Resolve(ResolveRequest{Model: "not-mapped-model", PromptProfile: "openai_gpt5"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if out.ResolvedModelProfile != "openai_gpt5" {
		t.Fatalf("expected explicit profile, got %q", out.ResolvedModelProfile)
	}
	if out.ModelFallback {
		t.Fatalf("expected model fallback false")
	}
}

func TestResolveRejectsUnknownPromptProfile(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	_, err = engine.Resolve(ResolveRequest{Model: "gpt-5-nano", PromptProfile: "missing"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
