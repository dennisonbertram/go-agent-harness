package harnessmcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPHandlerServesTheSameToolsAsStdio is the core of issue #1317.
//
// Two independent MCP delegation APIs existed with the same tool names and
// different schemas — the HTTP one's start_run took a prompt and nothing else,
// so it could not even select a model. One transport must not know a different
// set of tools from the other.
func TestHTTPHandlerServesTheSameToolsAsStdio(t *testing.T) {
	h := NewHTTPHandler(func(string) *HarnessClient { return NewHarnessClient("http://127.0.0.1:1") })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("tools/list = %d, want 200", rec.Code)
	}
	var resp struct {
		Result struct {
			Tools []Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		got[tool.Name] = true
	}
	for _, want := range toolDefs() {
		if !got[want.Name] {
			t.Errorf("HTTP transport is missing tool %q that stdio advertises", want.Name)
		}
	}
	if len(resp.Result.Tools) != len(toolDefs()) {
		t.Errorf("HTTP advertises %d tools, stdio advertises %d", len(resp.Result.Tools), len(toolDefs()))
	}
}

// TestHTTPHandlerForwardsCallerToken — the handler runs inside harnessd and
// reaches the REST API over loopback, so the caller's credential has to travel
// with it. Dropping it would either break authenticated daemons or, worse,
// require an unauthenticated bypass.
func TestHTTPHandlerForwardsCallerToken(t *testing.T) {
	var seen string
	h := NewHTTPHandler(func(token string) *HarnessClient {
		seen = token
		return NewHarnessClient("http://127.0.0.1:1")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer caller-token")
	h.ServeHTTP(rec, req)

	if seen != "caller-token" {
		t.Errorf("client built with token %q, want the caller's bearer token", seen)
	}
}

// TestHTTPHandlerRejectsNonPost — JSON-RPC over HTTP is POST; anything else is a
// client error rather than a silent 200.
func TestHTTPHandlerRejectsNonPost(t *testing.T) {
	h := NewHTTPHandler(func(string) *HarnessClient { return NewHarnessClient("http://127.0.0.1:1") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /mcp = %d, want 405", rec.Code)
	}
}

// TestHTTPHandlerReturnsParseErrorForMalformedBody keeps the JSON-RPC contract.
func TestHTTPHandlerReturnsParseErrorForMalformedBody(t *testing.T) {
	h := NewHTTPHandler(func(string) *HarnessClient { return NewHarnessClient("http://127.0.0.1:1") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":`)))

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Errorf("malformed body must produce a -32700 parse error, got: %+v", resp.Error)
	}
}
