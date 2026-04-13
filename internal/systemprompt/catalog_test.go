package systemprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFileEngineLoadsCatalog(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)

	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	out, err := engine.Resolve(ResolveRequest{Model: "gpt-5-nano"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if out.ResolvedIntent != "general" {
		t.Fatalf("expected default intent general, got %q", out.ResolvedIntent)
	}
	if out.ResolvedModelProfile != "openai_gpt5" {
		t.Fatalf("expected gpt5 profile, got %q", out.ResolvedModelProfile)
	}
	if !strings.Contains(out.StaticPrompt, "BASE_PROMPT") {
		t.Fatalf("expected base prompt in output: %q", out.StaticPrompt)
	}
}

func TestResolveUsesExplicitAutoresearchPromptProfile(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)

	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	out, err := engine.Resolve(ResolveRequest{
		Model:         "gpt-4.1-mini",
		PromptProfile: "autoresearch",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if out.ResolvedModelProfile != "autoresearch" {
		t.Fatalf("expected autoresearch profile, got %q", out.ResolvedModelProfile)
	}
	if !strings.Contains(out.StaticPrompt, "MODEL_AUTORESEARCH") {
		t.Fatalf("expected autoresearch profile content in output: %q", out.StaticPrompt)
	}
}

func TestNewFileEngineRejectsUnsupportedCatalogVersion(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	catalogPath := filepath.Join(root, "catalog.yaml")
	if err := os.WriteFile(catalogPath, []byte("version: 2\n"), 0o644); err != nil {
		t.Fatalf("overwrite catalog: %v", err)
	}

	_, err := NewFileEngine(root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewFileEngineRejectsMissingPromptFiles(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)
	if err := os.Remove(filepath.Join(root, "models/default.md")); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	_, err := NewFileEngine(root)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "models/default.md") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileEngineExtensionDirs(t *testing.T) {
	t.Parallel()
	root := makePromptFixture(t)

	engine, err := NewFileEngine(root)
	if err != nil {
		t.Fatalf("new file engine: %v", err)
	}

	behaviorsDir, talentsDir := engine.ExtensionDirs()
	if behaviorsDir == "" {
		t.Fatal("expected non-empty behaviorsDir")
	}
	if talentsDir == "" {
		t.Fatal("expected non-empty talentsDir")
	}

	// Verify dirs are absolute and correct.
	expectedBehaviorsDir := filepath.Join(root, "extensions", "behaviors")
	expectedTalentsDir := filepath.Join(root, "extensions", "talents")
	if behaviorsDir != expectedBehaviorsDir {
		t.Errorf("behaviorsDir: got %q, want %q", behaviorsDir, expectedBehaviorsDir)
	}
	if talentsDir != expectedTalentsDir {
		t.Errorf("talentsDir: got %q, want %q", talentsDir, expectedTalentsDir)
	}
}
