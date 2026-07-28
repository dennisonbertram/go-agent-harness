package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
)

// The ChatGPT Codex backend rejects a Responses request that does not set
// store=false with 400 "Store must be set to false". Omitting the field is not
// enough — OpenAI defaults it to true — so it has to be on the wire explicitly.
func TestResponsesRequestAlwaysSendsStoreFalse(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[{"type":"message","role":"assistant",` +
			`"content":[{"type":"output_text","text":"hi"}]}]}`))
	}))
	defer server.Close()

	client := newResponsesClient(t, server.URL)
	_, err := client.Complete(context.Background(), harness.CompletionRequest{
		Model:    "gpt-5.1-codex-mini",
		Messages: []harness.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("request body was not JSON: %v", err)
	}
	store, present := sent["store"]
	if !present {
		t.Fatalf("store was omitted; the Codex backend 400s on that. body: %s", body)
	}
	if store != false {
		t.Fatalf("store = %v, want false", store)
	}
}
