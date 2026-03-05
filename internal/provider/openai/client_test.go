package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
)

func TestClientCompleteParsesToolCalls(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"tool_choice":"auto"`) {
			t.Fatalf("expected tool_choice in request body: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[
				{
					"message":{
						"content":"",
						"tool_calls":[
							{"id":"call-1","type":"function","function":{"name":"list_files","arguments":"{\"path\":\".\"}"}}
						]
					}
				}
			]
		}`))
	}))
	defer testServer.Close()

	client, err := NewClient(Config{APIKey: "test-key", BaseURL: testServer.URL, Model: "gpt-4.1-mini"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	result, err := client.Complete(context.Background(), harness.CompletionRequest{
		Model: "gpt-4.1-mini",
		Messages: []harness.Message{
			{Role: "user", Content: "List files"},
		},
		Tools: []harness.ToolDefinition{{
			Name:        "list_files",
			Description: "List files",
			Parameters:  map[string]any{"type": "object"},
		}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "list_files" {
		t.Fatalf("unexpected tool call: %+v", result.ToolCalls[0])
	}
}

func TestClientCompleteFailsWithoutChoices(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer testServer.Close()

	client, err := NewClient(Config{APIKey: "test-key", BaseURL: testServer.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Complete(context.Background(), harness.CompletionRequest{
		Messages: []harness.Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("unexpected error: %v", err)
	}
}
