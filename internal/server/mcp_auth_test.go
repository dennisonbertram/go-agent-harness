package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/server"
	"go-agent-harness/internal/store"
)

// stubMCPHandler stands in for the real MCP endpoint; the point under test is
// whether the request reaches it at all.
func stubMCPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
	})
}

func newMCPServer(t *testing.T, authEnabled bool) (http.Handler, string) {
	t.Helper()
	opts := server.ServerOptions{
		Runner: harness.NewRunner(&authTestStaticProvider{}, harness.NewRegistry(),
			harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", DefaultSystemPrompt: "t", MaxSteps: 1}),
		MCPHandler: stubMCPHandler(),
	}
	var token string
	if authEnabled {
		ms := store.NewMemoryStore()
		raw, key := generateFastAPIKey(t, "tenant-mcp", "k", []string{store.ScopeRunsRead, store.ScopeRunsWrite})
		if err := ms.CreateAPIKey(context.Background(), key); err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		opts.Store = ms
		token = raw
	} else {
		opts.AuthDisabled = true
	}
	return server.NewWithOptions(opts), token
}

// TestMCPEndpointRequiresAuthWhenEnabled is the core of issue #1328. /mcp was
// mounted outside the authenticated mux, so it inherited no protection: /v1
// returned 401 while /mcp served start_run, steer_run, and conversation reads to
// anyone who could reach the port.
func TestMCPEndpointRequiresAuthWhenEnabled(t *testing.T) {
	h, token := newMCPServer(t, true)
	ts := httptest.NewServer(h)
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	res, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated POST /mcp = %d, want 401", res.StatusCode)
	}

	// Control: the same request with a valid token must succeed, so the fix is
	// not simply "reject everything".
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	authed, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated POST /mcp: %v", err)
	}
	defer authed.Body.Close()
	if authed.StatusCode != http.StatusOK {
		t.Errorf("authenticated POST /mcp = %d, want 200", authed.StatusCode)
	}
}

// TestMCPEndpointOpenWhenAuthDisabled — local development with auth off must be
// unaffected.
func TestMCPEndpointOpenWhenAuthDisabled(t *testing.T) {
	h, _ := newMCPServer(t, false)
	ts := httptest.NewServer(h)
	defer ts.Close()

	res, err := http.Post(ts.URL+"/mcp", "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("POST /mcp with auth disabled = %d, want 200", res.StatusCode)
	}
}
