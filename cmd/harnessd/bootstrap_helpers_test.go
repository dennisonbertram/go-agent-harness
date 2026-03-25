package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
	openai "go-agent-harness/internal/provider/openai"
)

func TestBuildCatalogBootstrapFallsBackToWorkspaceCatalog(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(workspace+"/catalog", 0o755); err != nil {
		t.Fatalf("mkdir catalog: %v", err)
	}
	if err := os.WriteFile(workspace+"/catalog/models.json", []byte(`{
  "catalog_version": "1.0.0",
  "providers": {
    "openrouter": {
      "display_name": "OpenRouter",
      "base_url": "https://openrouter.ai/api/v1",
      "api_key_env": "OPENROUTER_API_KEY",
      "models": {
        "openai/gpt-4.1-mini": {
          "display_name": "GPT-4.1 mini",
          "context_window": 128000,
          "modalities": ["text"],
          "tool_calling": true,
          "streaming": true,
          "api": "responses"
        }
      }
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}

	bootstrap, err := buildCatalogBootstrap(catalogBootstrapOptions{
		workspace: workspace,
		getenv:    func(string) string { return "" },
		newProvider: func(openai.Config) (harness.Provider, error) {
			return &noopProvider{}, nil
		},
	})
	if err != nil {
		t.Fatalf("buildCatalogBootstrap: %v", err)
	}
	if bootstrap.modelCatalog == nil {
		t.Fatal("expected model catalog")
	}
	if bootstrap.providerRegistry == nil {
		t.Fatal("expected provider registry")
	}
	if got := bootstrap.lookupModelAPI("openrouter", "openai/gpt-4.1-mini"); got != "responses" {
		t.Fatalf("lookupModelAPI: got %q", got)
	}
}

func TestBuildTriggerRuntimeHonorsSecrets(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GITHUB_WEBHOOK_SECRET": "gh-secret",
		"SLACK_SIGNING_SECRET":  "slack-secret",
	}
	var logs []string
	runtime := buildTriggerRuntime(func(key string) string { return env[key] }, func(format string, args ...any) {
		logs = append(logs, format)
	})

	if runtime.validators == nil {
		t.Fatal("expected validator registry")
	}
	if validator, ok := runtime.validators.Get("github"); !ok {
		t.Fatal("expected github validator")
	} else if got := fmt.Sprintf("%T", validator); !strings.HasSuffix(got, ".GitHubValidator") {
		t.Fatalf("github validator type: got %q", got)
	}
	if validator, ok := runtime.validators.Get("slack"); !ok {
		t.Fatal("expected slack validator")
	} else if got := fmt.Sprintf("%T", validator); !strings.HasSuffix(got, ".SlackValidator") {
		t.Fatalf("slack validator type: got %q", got)
	}
	if _, ok := runtime.validators.Get("linear"); ok {
		t.Fatal("did not expect linear validator")
	}

	if got := fmt.Sprintf("%T", runtime.github); !strings.HasSuffix(got, ".GitHubAdapter") {
		t.Fatal("expected github adapter")
	}
	if got := fmt.Sprintf("%T", runtime.slack); !strings.HasSuffix(got, ".SlackAdapter") {
		t.Fatal("expected slack adapter")
	}
	if runtime.linear != nil {
		t.Fatal("did not expect linear adapter")
	}

	logText := strings.Join(logs, "\n")
	for _, want := range []string{
		"registered GitHub webhook validator",
		"registered Slack webhook validator",
		"registered GitHub webhook adapter for /v1/webhooks/github",
		"registered Slack webhook adapter for /v1/webhooks/slack",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("expected log %q in %q", want, logText)
		}
	}
}
