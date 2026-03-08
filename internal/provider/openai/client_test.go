package openai

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/provider/pricing"
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
			],
			"usage":{
				"prompt_tokens":120,
				"completion_tokens":30,
				"total_tokens":150,
				"prompt_tokens_details":{"cached_tokens":20,"audio_tokens":0},
				"completion_tokens_details":{"reasoning_tokens":12,"audio_tokens":2}
			}
		}`))
	}))
	defer testServer.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: testServer.URL,
		Model:   "gpt-4.1-mini",
		PricingResolver: pricing.NewResolverFromCatalog(&pricing.Catalog{
			PricingVersion: "vtest",
			Providers: map[string]pricing.ProviderCatalog{
				"openai": {
					Models: map[string]pricing.Rates{
						"gpt-4.1-mini": {
							InputPer1MTokensUSD:     1.00,
							OutputPer1MTokensUSD:    2.00,
							CacheReadPer1MTokensUSD: 0.50,
						},
					},
				},
			},
		}),
	})
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
	if result.Usage == nil {
		t.Fatalf("expected usage")
	}
	if result.UsageStatus != harness.UsageStatusProviderReported {
		t.Fatalf("unexpected usage status: %q", result.UsageStatus)
	}
	if result.Usage.PromptTokens != 120 || result.Usage.CompletionTokens != 30 || result.Usage.TotalTokens != 150 {
		t.Fatalf("unexpected usage values: %+v", result.Usage)
	}
	if result.Usage.CachedPromptTokens == nil || *result.Usage.CachedPromptTokens != 20 {
		t.Fatalf("expected cached prompt tokens: %+v", result.Usage)
	}
	if result.Usage.ReasoningTokens == nil || *result.Usage.ReasoningTokens != 12 {
		t.Fatalf("expected reasoning tokens: %+v", result.Usage)
	}
	if result.CostStatus != harness.CostStatusAvailable {
		t.Fatalf("unexpected cost status: %q", result.CostStatus)
	}
	if result.Cost == nil || result.CostUSD == nil {
		t.Fatalf("expected cost values")
	}
	if result.Cost.PricingVersion != "vtest" {
		t.Fatalf("unexpected pricing version: %+v", result.Cost)
	}
	// expected: non-cached input (100)*1.0 + output (30)*2.0 + cache-read (20)*0.5 per 1M tokens.
	expected := (100.0/1_000_000.0)*1.0 + (30.0/1_000_000.0)*2.0 + (20.0/1_000_000.0)*0.5
	if math.Abs(*result.CostUSD-expected) > 1e-12 {
		t.Fatalf("unexpected total cost: got=%f want=%f", *result.CostUSD, expected)
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

func TestClientCompleteStreamsAssistantAndToolCallDeltas(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, `"stream":true`) {
			t.Fatalf("expected stream=true in request body: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, `"include_usage":true`) {
			t.Fatalf("expected stream_options.include_usage=true in request body: %s", bodyStr)
		}

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"content":"Hel"}}]}`,
			``,
			`data: {"choices":[{"delta":{"content":"lo"}}]}`,
			``,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"write","arguments":"{\"path\":\""}}]}}]}`,
			``,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"demo.txt\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	defer testServer.Close()

	client, err := NewClient(Config{APIKey: "test-key", BaseURL: testServer.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	var deltas []harness.CompletionDelta
	result, err := client.Complete(context.Background(), harness.CompletionRequest{
		Messages: []harness.Message{{Role: "user", Content: "Hi"}},
		Stream: func(delta harness.CompletionDelta) {
			deltas = append(deltas, delta)
		},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if result.Content != "Hello" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "write" || result.ToolCalls[0].Arguments != `{"path":"demo.txt"}` {
		t.Fatalf("unexpected tool call: %+v", result.ToolCalls[0])
	}
	if result.Usage == nil || result.Usage.TotalTokens != 14 {
		t.Fatalf("expected streamed usage totals, got %+v", result.Usage)
	}

	var contentParts []string
	var toolArgParts []string
	for _, delta := range deltas {
		if delta.Content != "" {
			contentParts = append(contentParts, delta.Content)
		}
		if delta.ToolCall.Arguments != "" {
			toolArgParts = append(toolArgParts, delta.ToolCall.Arguments)
		}
	}
	if !slices.Equal(contentParts, []string{"Hel", "lo"}) {
		t.Fatalf("unexpected content deltas: %+v", contentParts)
	}
	if !slices.Equal(toolArgParts, []string{`{"path":"`, `demo.txt"}`}) {
		t.Fatalf("unexpected tool argument deltas: %+v", toolArgParts)
	}
}

func TestProcessStreamBlock_ReasoningContent(t *testing.T) {
	t.Parallel()

	raw := `data: {"choices":[{"delta":{"reasoning_content":"Let me think"}}]}`
	state := &streamedCompletionState{}
	var received []harness.CompletionDelta
	streamFn := func(delta harness.CompletionDelta) {
		received = append(received, delta)
	}

	done, err := processStreamBlock(raw, state, streamFn)
	if err != nil {
		t.Fatalf("processStreamBlock error: %v", err)
	}
	if done {
		t.Fatalf("expected done=false")
	}
	if state.reasoning.String() != "Let me think" {
		t.Fatalf("expected reasoning buffer = %q, got %q", "Let me think", state.reasoning.String())
	}
	if len(received) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(received))
	}
	if received[0].Reasoning != "Let me think" {
		t.Fatalf("expected delta.Reasoning = %q, got %q", "Let me think", received[0].Reasoning)
	}
	if received[0].Content != "" {
		t.Fatalf("expected delta.Content to be empty, got %q", received[0].Content)
	}
}

func TestClientCompleteStreamsReasoningContent(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"choices":[{"delta":{"reasoning_content":"Think"}}]}`,
			``,
			`data: {"choices":[{"delta":{"reasoning_content":"ing..."}}]}`,
			``,
			`data: {"choices":[{"delta":{"content":"Answer"}}]}`,
			``,
			`data: {"choices":[{"delta":{}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n"))
	}))
	defer testServer.Close()

	client, err := NewClient(Config{APIKey: "test-key", BaseURL: testServer.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	var deltas []harness.CompletionDelta
	result, err := client.Complete(context.Background(), harness.CompletionRequest{
		Messages: []harness.Message{{Role: "user", Content: "Hi"}},
		Stream: func(delta harness.CompletionDelta) {
			deltas = append(deltas, delta)
		},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if result.Content != "Answer" {
		t.Fatalf("unexpected content: %q", result.Content)
	}

	var reasoningParts []string
	var contentParts []string
	for _, d := range deltas {
		if d.Reasoning != "" {
			reasoningParts = append(reasoningParts, d.Reasoning)
		}
		if d.Content != "" {
			contentParts = append(contentParts, d.Content)
		}
	}
	if !slices.Equal(reasoningParts, []string{"Think", "ing..."}) {
		t.Fatalf("unexpected reasoning deltas: %+v", reasoningParts)
	}
	if !slices.Equal(contentParts, []string{"Answer"}) {
		t.Fatalf("unexpected content deltas: %+v", contentParts)
	}
}

func TestClientCompleteMissingUsageReturnsProviderUnreported(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"ok","tool_calls":[]}}]
		}`))
	}))
	defer testServer.Close()

	client, err := NewClient(Config{APIKey: "test-key", BaseURL: testServer.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	result, err := client.Complete(context.Background(), harness.CompletionRequest{
		Messages: []harness.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result.Usage == nil {
		t.Fatalf("expected usage object")
	}
	if result.Usage.PromptTokens != 0 || result.Usage.CompletionTokens != 0 || result.Usage.TotalTokens != 0 {
		t.Fatalf("expected zero usage, got %+v", result.Usage)
	}
	if result.UsageStatus != harness.UsageStatusProviderUnreported {
		t.Fatalf("unexpected usage status: %q", result.UsageStatus)
	}
	if result.CostStatus != harness.CostStatusProviderUnreported {
		t.Fatalf("unexpected cost status: %q", result.CostStatus)
	}
	if result.CostUSD == nil || *result.CostUSD != 0 {
		t.Fatalf("expected zero cost, got %+v", result.CostUSD)
	}
}

func TestClientCompleteUnpricedModelReturnsUnpricedStatus(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"ok","tool_calls":[]}}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`))
	}))
	defer testServer.Close()

	client, err := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: testServer.URL,
		Model:   "gpt-unpriced",
		PricingResolver: pricing.NewResolverFromCatalog(&pricing.Catalog{
			PricingVersion: "v1",
			Providers: map[string]pricing.ProviderCatalog{
				"openai": {
					Models: map[string]pricing.Rates{
						"another-model": {
							InputPer1MTokensUSD:  1,
							OutputPer1MTokensUSD: 1,
						},
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	result, err := client.Complete(context.Background(), harness.CompletionRequest{
		Messages: []harness.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result.CostStatus != harness.CostStatusUnpricedModel {
		t.Fatalf("unexpected cost status: %q", result.CostStatus)
	}
	if result.CostUSD == nil || *result.CostUSD != 0 {
		t.Fatalf("expected zero cost, got %+v", result.CostUSD)
	}
}
