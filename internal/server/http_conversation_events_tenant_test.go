package server_test

// http_conversation_events_tenant_test.go — proves GET
// /v1/conversations/{id}/events (issue #950) is gated the same way as its
// sibling conversation sub-resource routes (messages, export, runs): it
// requires runs:read scope, and it 404s (rather than leaking existence or
// content) for a conversation owned by a different tenant.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-agent-harness/internal/fakeprovider"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/server"
	"go-agent-harness/internal/store"
)

// Issue #1158: the additive watermark must not weaken the messages route's
// authorization order. A caller without runs:read is rejected before even an
// unknown conversation is resolved; a scoped caller reaches the normal 404.
func TestConversationMessagesWatermark_RequiresRunsReadBeforeLookup(t *testing.T) {
	t.Parallel()

	ms := store.NewMemoryStore()
	const tenantID = "tenant-conv-messages-watermark"
	noScopeToken, noScopeKey := generateFastAPIKey(t, tenantID, "no scopes", nil)
	if err := ms.CreateAPIKey(context.Background(), noScopeKey); err != nil {
		t.Fatalf("CreateAPIKey(no scope): %v", err)
	}
	readToken, readKey := generateFastAPIKey(t, tenantID, "read", []string{store.ScopeRunsRead})
	if err := ms.CreateAPIKey(context.Background(), readKey); err != nil {
		t.Fatalf("CreateAPIKey(read): %v", err)
	}

	runner := harness.NewRunner(fakeprovider.New(nil), harness.NewRegistry(), harness.RunnerConfig{Store: ms})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	handler := server.NewWithOptions(server.ServerOptions{Runner: runner, Store: ms})

	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/conversations/not-visible/messages", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if response := request(noScopeToken); response.Code != http.StatusForbidden {
		t.Fatalf("no-scope status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if response := request(readToken); response.Code != http.StatusNotFound {
		t.Fatalf("read-scope status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
}

// BT-004: the route requires runs:read scope, same as every sibling GET
// conversation sub-resource route.
func TestConversationEvents_RequiresRunsReadScope(t *testing.T) {
	t.Parallel()

	ms := store.NewMemoryStore()
	const tenantID = "tenant-conv-events-scope"

	noScopeToken, noScopeKey := generateFastAPIKey(t, tenantID, "no scopes", nil)
	if err := ms.CreateAPIKey(context.Background(), noScopeKey); err != nil {
		t.Fatalf("CreateAPIKey(no scope): %v", err)
	}
	readToken, readKey := generateFastAPIKey(t, tenantID, "read", []string{store.ScopeRunsRead})
	if err := ms.CreateAPIKey(context.Background(), readKey); err != nil {
		t.Fatalf("CreateAPIKey(read): %v", err)
	}

	prov := fakeprovider.New([]fakeprovider.Turn{{Hang: true}})
	runner := harness.NewRunner(prov, harness.NewRegistry(), harness.RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     3,
	})
	handler := server.NewWithOptions(server.ServerOptions{Runner: runner, Store: ms})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	t.Cleanup(func() { runner.Shutdown(context.Background()) })

	run, err := runner.StartRun(harness.RunRequest{Prompt: "hello", TenantID: tenantID})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	t.Cleanup(prov.Release)

	// No-scope caller must be rejected -- same as every sibling GET
	// conversation sub-resource route -- before conversation existence or
	// tenant ownership is even considered.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel1()
	req1, _ := http.NewRequestWithContext(ctx1, http.MethodGet, ts.URL+"/v1/conversations/"+run.ConversationID+"/events", nil)
	req1.Header.Set("Authorization", "Bearer "+noScopeToken)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("GET events (no scope): %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp1.Body)
		t.Fatalf("no-scope GET events: status = %d, want %d; body=%s", resp1.StatusCode, http.StatusForbidden, body)
	}
	var denied struct {
		Error    string `json:"error"`
		Required string `json:"required"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&denied); err != nil {
		t.Fatalf("decode forbidden body: %v", err)
	}
	if denied.Error != "insufficient_scope" || denied.Required != store.ScopeRunsRead {
		t.Errorf("forbidden body = %+v, want error=insufficient_scope required=%s", denied, store.ScopeRunsRead)
	}

	// Positive control: the same route with runs:read succeeds. Without this,
	// a blanket-403 implementation could pass the assertion above too.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	req2, _ := http.NewRequestWithContext(ctx2, http.MethodGet, ts.URL+"/v1/conversations/"+run.ConversationID+"/events", nil)
	req2.Header.Set("Authorization", "Bearer "+readToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("GET events (read scope): %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("read-scope GET events: status = %d, want %d; body=%s", resp2.StatusCode, http.StatusOK, body)
	}
}

// BT-005: a conversation owned by tenant A must be invisible to tenant B on
// this route, matching blockConversationCrossTenant's contract for sibling
// GET conversation sub-resource routes (messages, export, runs).
func TestConversationEvents_CrossTenantDenied(t *testing.T) {
	t.Parallel()

	f := newTwoTenantFixture(t)
	runID := f.startRun(t, f.tokenA)
	f.waitInFlight(t)
	convID := f.convIDFromRun(t, runID)

	// Owner (tenant A) can open the conversation stream.
	if code, body := f.doByID(t, http.MethodGet, f.tokenA, "/v1/conversations/"+convID+"/events"); code != http.StatusOK {
		t.Errorf("owner GET conversation events: got %d, want 200; body %s", code, body)
	}

	// Tenant B must not be able to open (or even learn of the existence of)
	// tenant A's conversation stream.
	if code, body := f.doByID(t, http.MethodGet, f.tokenB, "/v1/conversations/"+convID+"/events"); code != http.StatusNotFound {
		t.Errorf("tenant B GET conversation events: got %d, want 404; body %s", code, body)
	}
}
