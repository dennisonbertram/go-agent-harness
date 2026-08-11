package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/internal/goals"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/server"
)

func TestToolsEndpoint_EnumeratesCatalog(t *testing.T) {
	// A default registry with a goals manager wired: exercises both an
	// always-registered tool (deploy) and a conditionally-registered one (goals).
	reg := harness.NewDefaultRegistryWithOptions(t.TempDir(), harness.DefaultRegistryOptions{
		GoalManager: goals.NewManager(nil),
	})
	h := server.NewWithOptions(server.ServerOptions{
		AuthDisabled: true,
		Tools:        reg,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Count int `json:"count"`
		Tools []struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Tier        string   `json:"tier"`
			Tags        []string `json:"tags"`
			Owner       string   `json:"owner"`
			Condition   string   `json:"condition"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count == 0 || resp.Count != len(resp.Tools) {
		t.Fatalf("count %d does not match tools len %d", resp.Count, len(resp.Tools))
	}

	byName := make(map[string]struct {
		owner     string
		condition string
	}, len(resp.Tools))
	for _, tool := range resp.Tools {
		byName[tool.Name] = struct {
			owner     string
			condition string
		}{tool.Owner, tool.Condition}
		if tool.Name == "" || tool.Tier == "" || tool.Owner == "" || tool.Condition == "" {
			t.Errorf("tool has empty catalog metadata: %+v", tool)
		}
	}
	for _, want := range []string{"deploy", "goals", "todos", "read", "bash"} {
		if _, found := byName[want]; !found {
			t.Errorf("expected tool %q in catalog; got %v", want, keysOf(byName))
		}
	}
	if goalsMeta := byName["goals"]; goalsMeta.owner != "harness.goals" || goalsMeta.condition != "goal manager configured" {
		t.Errorf("goals provenance = %#v", goalsMeta)
	}
}

func TestToolsEndpoint_ExposesConfiguredUnavailableResolverEvidence(t *testing.T) {
	reg := harness.NewRegistry()
	reg.SetToolsetResolutionSnapshot(harness.ToolsetResolutionSnapshot{
		ConfiguredUnavailable: []harness.ConfiguredUnavailableToolset{{
			Name: "mcp:calendar", Owner: "harness.mcp", Condition: "MCP server calendar configured",
			Provenance: harness.ToolsetResolverProvenance{Source: "runtime.mcp_registry", Provider: "calendar"},
		}},
		Unavailable: []harness.UnavailableToolsetObservation{{
			Name: "mcp:calendar", Owner: "harness.mcp", Condition: "MCP server calendar configured", Reason: "mcp_tool_discovery_failed",
			Provenance: harness.ToolsetResolverProvenance{Source: "runtime.mcp_registry", Provider: "calendar"},
		}},
	})
	h := server.NewWithOptions(server.ServerOptions{AuthDisabled: true, Tools: reg})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tools", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Configured  []harness.ConfiguredUnavailableToolset  `json:"configured_unavailable_toolsets"`
		Unavailable []harness.UnavailableToolsetObservation `json:"unavailable"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Configured) != 1 || len(payload.Unavailable) != 1 || payload.Configured[0].Name != "mcp:calendar" || payload.Unavailable[0].Reason != "mcp_tool_discovery_failed" {
		t.Fatalf("/v1/tools resolver evidence = %#v", payload)
	}
}

func TestToolsEndpoint_FailsClosedForIncompleteToolsetResolution(t *testing.T) {
	reg := harness.NewRegistry()
	reg.SetToolsetResolutionSnapshot(harness.ToolsetResolutionSnapshot{
		Incomplete: true, IncompleteReason: "mcp_tool_discovery_failed_without_provider_identity",
	})
	h := server.NewWithOptions(server.ServerOptions{AuthDisabled: true, Tools: reg})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tools", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "toolset_resolution_incomplete") {
		t.Fatalf("body = %s, want explicit incomplete-resolution code", rec.Body.String())
	}
}

func TestToolsEndpoint_NotConfigured501(t *testing.T) {
	h := server.NewWithOptions(server.ServerOptions{AuthDisabled: true})
	req := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}

func keysOf(m map[string]struct {
	owner     string
	condition string
}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
