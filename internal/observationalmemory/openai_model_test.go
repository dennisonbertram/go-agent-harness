package observationalmemory

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIModelComplete(t *testing.T) {
	t.Parallel()

	var authHeader string
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  observed output  "}}]}`))
	}))
	t.Cleanup(srv.Close)

	model, err := NewOpenAIModel(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   "gpt-5-nano",
	})
	if err != nil {
		t.Fatalf("new openai model: %v", err)
	}

	out, err := model.Complete(context.Background(), ModelRequest{
		Messages: []PromptMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "user"},
		},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if out != "observed output" {
		t.Fatalf("unexpected output: %q", out)
	}
	if authHeader != "Bearer test-key" {
		t.Fatalf("unexpected auth header: %q", authHeader)
	}
	if !strings.Contains(body, `"model":"gpt-5-nano"`) {
		t.Fatalf("request body missing model: %s", body)
	}
	if !strings.Contains(body, `"role":"system"`) || !strings.Contains(body, `"role":"user"`) {
		t.Fatalf("request body missing messages: %s", body)
	}
}

func TestNewOpenAIModelRequiresAPIKey(t *testing.T) {
	t.Parallel()

	_, err := NewOpenAIModel(OpenAIConfig{})
	if err == nil {
		t.Fatalf("expected api key error")
	}
}
