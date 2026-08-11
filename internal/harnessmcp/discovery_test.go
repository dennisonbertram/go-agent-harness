package harnessmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestListProfilesTool — start_run accepts a `profile` name, and before this an
// MCP caller had no way to discover a single valid value (issue #1324).
func TestListProfilesTool(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"count":2,"profiles":[
		  {"name":"bash-runner","description":"Script execution","model":"gpt-4.1-mini","allowed_tools":["bash"],"source_tier":"built-in"},
		  {"name":"file-writer","description":"Code changes","model":"gpt-4.1-mini","allowed_tools":["read","write"],"source_tier":"built-in"}
		]}`))
	}))
	defer srv.Close()

	res, err := newListProfilesHandler(NewHarnessClient(srv.URL))(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list_profiles: %v", err)
	}
	if gotPath != "/v1/profiles" {
		t.Errorf("hit %q, want /v1/profiles", gotPath)
	}
	text := res.Content[0].Text
	for _, want := range []string{"bash-runner", "file-writer", "Script execution"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q: %s", want, text)
		}
	}
}

// TestListToolsTool — allowed_tools and denied_tools take tool names that were
// equally undiscoverable.
func TestListToolsTool(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"count":2,"tools":[
		  {"name":"read","description":"Read a file","tier":"core","parameters":{"properties":{"path":{"type":"string"}}}},
		  {"name":"bash","description":"Run a command","tier":"core","parameters":{"properties":{"command":{"type":"string"}}}}
		]}`))
	}))
	defer srv.Close()

	res, err := newListToolsHandler(NewHarnessClient(srv.URL))(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list_tools: %v", err)
	}
	if gotPath != "/v1/tools" {
		t.Errorf("hit %q, want /v1/tools", gotPath)
	}

	var out struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Tier        string          `json:"tier"`
			Parameters  json.RawMessage `json:"parameters"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(out.Tools))
	}
	if out.Tools[0].Name != "read" || out.Tools[0].Description != "Read a file" {
		t.Errorf("unexpected first tool: %+v", out.Tools[0])
	}
	// Full JSON Schemas for 67 tools would swamp the caller's context, and a
	// caller filling allowed_tools needs names, not parameter schemas.
	if len(out.Tools[0].Parameters) != 0 {
		t.Errorf("parameter schemas must be omitted to keep the listing small, got: %s", out.Tools[0].Parameters)
	}
}

// TestDiscoveryToolsAdvertisedAndDispatchable — a tool nobody can discover or
// call is not exposed.
func TestDiscoveryToolsAdvertisedAndDispatchable(t *testing.T) {
	advertised := map[string]bool{}
	for _, tool := range toolDefs() {
		advertised[tool.Name] = true
	}
	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})

	for _, name := range []string{"list_profiles", "list_tools"} {
		if !advertised[name] {
			t.Errorf("%q is not advertised in tools/list", name)
		}
		if _, ok := d.tools[name]; !ok {
			t.Errorf("%q has no handler", name)
		}
	}
}
