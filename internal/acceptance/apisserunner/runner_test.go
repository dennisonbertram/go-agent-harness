package apisserunner_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-agent-harness/internal/acceptance/apisserunner"
	"go-agent-harness/internal/acceptance/inventory"
	"go-agent-harness/internal/harness"
	tools "go-agent-harness/internal/harness/tools"
)

func TestRunnerExecutesHashBoundOrderedAPISSECaseAndRetainsIndependentEvidence(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{
		Definition: harness.ToolDefinition{Name: "fixture_write", Description: "writes an isolated fixture"},
		Tier:       tools.TierCore, Owner: "test.fixture", Condition: "isolated test fixture",
	}}})
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/v1/runs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run_id":"run-one","conversation_id":"conv-one"}`))
		case "/v1/runs/run-one/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("id: event-one\nevent: tool.call.completed\ndata: {\"run_id\":\"run-one\",\"type\":\"tool.call.completed\"}\n\nid: event-two\nevent: run.completed\ndata: {\"run_id\":\"run-one\",\"type\":\"run.completed\"}\n\n"))
		case "/v1/runs/run-one":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"run-one","conversation_id":"conv-one","status":"completed"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	caseDef := inventory.Case{
		ItemID: "tool:fixture_write", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation,
		OrderedActions:         []inventory.Action{{Kind: "start", Value: "write fixture"}, {Kind: "stream", Value: "run-one"}, {Kind: "probe", Value: "fixture-state"}},
		ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionDurableState, Probe: "fixture-state", AssertionID: "written", Description: "fixture exists"}},
		Cleanup:                "remove isolated fixture",
	}
	root := t.TempDir()
	evidence, err := (apisserunner.Runner{BaseURL: server.URL, ArtifactRoot: root}).Run(context.Background(), compiled, []apisserunner.Plan{{
		Case: caseDef, Prompt: "write fixture",
		Probe: func(context.Context, string, string) ([]inventory.ProbeObservation, error) {
			return []inventory.ProbeObservation{{Kind: inventory.PostconditionDurableState, Probe: "fixture-state", AssertionID: "written", Value: "exists", Verified: true}}, nil
		},
		Cleanup: func(context.Context) (string, error) { return "fixture removed", nil },
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].Outcome != inventory.Pass {
		t.Fatalf("evidence = %#v", evidence)
	}
	if evidence[0].RunID != "run-one" || evidence[0].ConversationID != "conv-one" || strings.Join(evidence[0].EventIDs, ",") != "event-one,event-two" {
		t.Fatalf("runtime identity evidence = %#v", evidence[0])
	}
	if len(evidence[0].Artifacts) < 2 {
		t.Fatalf("artifact evidence = %#v", evidence[0].Artifacts)
	}
	for _, artifact := range evidence[0].Artifacts {
		if _, err := os.Stat(filepath.Join(root, artifact.Path)); err != nil {
			t.Fatalf("artifact %q was not retained: %v", artifact.Path, err)
		}
	}
	if strings.Join(got, ",") != "POST /v1/runs,GET /v1/runs/run-one/events,GET /v1/runs/run-one" {
		t.Fatalf("HTTP sequence = %v", got)
	}
}

func TestRunnerRunLiveFailsBeforeDispatchWhenRegistryDerivedAPICaseIsMissing(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tools" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tools":[{"name":"required","description":"fixture","tier":"core","owner":"test","condition":"test"}],"configured_unavailable_toolsets":[],"unavailable":[]}`))
			return
		}
		called = true
		http.Error(w, "must not dispatch", http.StatusInternalServerError)
	}))
	defer server.Close()
	_, _, err := (apisserunner.Runner{BaseURL: server.URL, ArtifactRoot: t.TempDir()}).RunLive(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "missing acceptance case") {
		t.Fatalf("Run error = %v", err)
	}
	if called {
		t.Fatal("runner dispatched despite missing registry-derived case")
	}
}

func TestRunnerCarriesRestrictedAllowedToolsToRealAdmission(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "blocked"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/runs":
			_ = json.NewDecoder(r.Body).Decode(&request)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run_id":"run","conversation_id":"conv"}`))
		case "/v1/runs/run/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("id: event\nevent: run.completed\ndata: {}\n\n"))
		case "/v1/runs/run":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"completed","conversation_id":"conv"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	caseDef := inventory.Case{ItemID: "tool:blocked", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: []inventory.Action{{Kind: "start", Value: "restricted"}}, ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionExternalState, Probe: "none", AssertionID: "no-mutation", Description: "blocked before handler"}}, Cleanup: "nothing created"}
	_, err = (apisserunner.Runner{BaseURL: server.URL, ArtifactRoot: t.TempDir()}).Run(context.Background(), compiled, []apisserunner.Plan{{Case: caseDef, Prompt: "attempt blocked tool", StartFields: map[string]any{"allowed_tools": []string{"read"}}, Probe: func(context.Context, string, string) ([]inventory.ProbeObservation, error) {
		return []inventory.ProbeObservation{{Kind: inventory.PostconditionExternalState, Probe: "none", AssertionID: "no-mutation", Value: "handler not entered", Verified: true}}, nil
	}, Cleanup: func(context.Context) (string, error) { return "nothing created", nil }}})
	if err != nil {
		t.Fatal(err)
	}
	allowed, ok := request["allowed_tools"].([]any)
	if !ok || len(allowed) != 1 || allowed[0] != "read" {
		t.Fatalf("allowed_tools request = %#v", request)
	}
}

func TestRunnerCleansUpAfterAcceptedRunWhenSSEFails(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "fixture"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/runs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run_id":"run","conversation_id":"conv"}`))
		case "/v1/runs/run/events":
			http.Error(w, "stream unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cleaned := false
	caseDef := inventory.Case{ItemID: "tool:fixture", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: []inventory.Action{{Kind: "start", Value: "fixture"}}, ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionExternalState, Probe: "probe", AssertionID: "a", Description: "fixture"}}, Cleanup: "cleanup"}
	_, err = (apisserunner.Runner{BaseURL: server.URL, ArtifactRoot: t.TempDir()}).Run(context.Background(), compiled, []apisserunner.Plan{{Case: caseDef, Prompt: "fixture", Probe: func(context.Context, string, string) ([]inventory.ProbeObservation, error) { return nil, nil }, Cleanup: func(context.Context) (string, error) { cleaned = true; return "cleaned", nil }}})
	if err == nil {
		t.Fatal("Run unexpectedly succeeded")
	}
	if !cleaned {
		t.Fatal("cleanup was not called after accepted run failed to stream")
	}
}

func TestRunnerPreservesPrimaryAndCleanupErrorsAfterAcceptance(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "fixture"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/runs":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run_id":"run","conversation_id":"conv"}`))
		case "/v1/runs/run/events":
			http.Error(w, "stream unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	caseDef := inventory.Case{ItemID: "tool:fixture", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: []inventory.Action{{Kind: "start", Value: "fixture"}}, ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionExternalState, Probe: "probe", AssertionID: "a", Description: "fixture"}}, Cleanup: "cleanup"}
	_, err = (apisserunner.Runner{BaseURL: server.URL, ArtifactRoot: t.TempDir()}).Run(context.Background(), compiled, []apisserunner.Plan{{Case: caseDef, Prompt: "fixture", Probe: func(context.Context, string, string) ([]inventory.ProbeObservation, error) { return nil, nil }, Cleanup: func(context.Context) (string, error) { return "", errors.New("cleanup failed") }}})
	if err == nil || !strings.Contains(err.Error(), "SSE status") || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("combined error = %v", err)
	}
}

func TestLoadLiveInventoryBindsToolCatalogAndRejectsAbsentResolverEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
	}{
		{name: "complete", body: `{"tools":[{"name":"fixture","description":"fixture","tier":"core","owner":"test","condition":"fixture"}],"configured_unavailable_toolsets":[],"unavailable":[]}`},
		{name: "absent resolver evidence", body: `{"tools":[]}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/tools" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			compiled, err := (apisserunner.Runner{BaseURL: server.URL}).LoadLiveInventory(context.Background())
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "resolver evidence") {
					t.Fatalf("LoadLiveInventory error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range compiled.Items {
				if item.ID == "tool:fixture" {
					found = true
				}
			}
			if !found {
				t.Fatalf("live compiled inventory omitted fixture: %#v", compiled.Items)
			}
		})
	}
}

func TestBuildCoverageReportKeepsDynamicNASeparateFromMissing(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "covered"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}, {Definition: harness.ToolDefinition{Name: "missing"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}, Unavailable: []inventory.ResolverObservation{{Kind: inventory.ToolsetKind, Name: "mcp:calendar", Owner: "mcp", Condition: "configured", Reason: "unavailable", Provenance: inventory.ResolverProvenance{Source: "test", Provider: "calendar"}}}})
	if err != nil {
		t.Fatal(err)
	}
	caseDef := inventory.Case{ItemID: "tool:covered", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: []inventory.Action{{Kind: "start", Value: "fixture"}}, ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionDurableState, Probe: "probe", AssertionID: "a", Description: "fixture"}}, Cleanup: "cleanup"}
	report, err := apisserunner.BuildCoverageReport(compiled, apisserunner.Manifest{InventoryHash: compiled.Hash, Cases: []inventory.Case{caseDef}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Available != 2 || report.Planned != 1 || strings.Join(report.Missing, ",") != "tool:missing" || len(report.NotApplicable) != 1 || !strings.Contains(report.NotApplicable[0], "mcp:calendar") {
		t.Fatalf("coverage report = %#v", report)
	}
}

func TestBuildCoverageReportRejectsStaleAndNotApplicableManifestCases(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "live"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}, Unavailable: []inventory.ResolverObservation{{Kind: inventory.ToolsetKind, Name: "mcp:offline", Owner: "mcp", Condition: "configured", Reason: "offline", Provenance: inventory.ResolverProvenance{Source: "test", Provider: "offline"}}}})
	if err != nil {
		t.Fatal(err)
	}
	base := func(id string) inventory.Case {
		return inventory.Case{ItemID: id, Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: []inventory.Action{{Kind: "start", Value: "fixture"}}, ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionExternalState, Probe: "probe", AssertionID: "a", Description: "fixture"}}, Cleanup: "cleanup"}
	}
	for _, tc := range []struct{ name, id string }{{"stale", "tool:removed"}, {"not applicable", "toolset:mcp:offline"}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := apisserunner.BuildCoverageReport(compiled, apisserunner.Manifest{InventoryHash: compiled.Hash, Cases: []inventory.Case{base(tc.id)}}); err == nil {
				t.Fatal("BuildCoverageReport accepted invalid manifest case")
			}
		})
	}
}

func TestBuildCoverageReportRejectsCurrentCatalogHashDriftEvenWhenCaseIDsMatch(t *testing.T) {
	compile := func(description string) inventory.Compiled {
		t.Helper()
		compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "stable-id", Description: description}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}})
		if err != nil {
			t.Fatal(err)
		}
		return compiled
	}
	authored := compile("original catalog definition")
	live := compile("changed catalog definition")
	if authored.Hash == live.Hash {
		t.Fatal("fixture catalogs must have distinct hashes")
	}
	caseDef := inventory.Case{ItemID: "tool:stable-id", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: []inventory.Action{{Kind: "start", Value: "fixture"}}, ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionDurableState, Probe: "probe", AssertionID: "a", Description: "fixture"}}, Cleanup: "cleanup"}
	_, err := apisserunner.BuildCoverageReport(live, apisserunner.Manifest{InventoryHash: authored.Hash, Cases: []inventory.Case{caseDef}})
	if err == nil || !strings.Contains(err.Error(), "does not match live inventory hash") {
		t.Fatalf("BuildCoverageReport hash drift error = %v", err)
	}
}

func TestRunnerCleansUpImmediatelyAfterAcceptedUnusableStartResponse(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "fixture"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	caseDef := inventory.Case{ItemID: "tool:fixture", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: []inventory.Action{{Kind: "start", Value: "fixture"}}, ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionExternalState, Probe: "probe", AssertionID: "a", Description: "fixture"}}, Cleanup: "cleanup"}
	for _, tc := range []struct {
		name, body, primary string
	}{
		{name: "malformed JSON", body: `{"run_id":`, primary: "unexpected EOF"},
		{name: "missing run identity", body: `{}`, primary: "omitted run identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/runs" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			cleaned := false
			_, err := (apisserunner.Runner{BaseURL: server.URL, ArtifactRoot: t.TempDir()}).Run(context.Background(), compiled, []apisserunner.Plan{{Case: caseDef, Prompt: "fixture", Cleanup: func(context.Context) (string, error) {
				cleaned = true
				return "", errors.New("cleanup failed")
			}}})
			if !cleaned {
				t.Fatal("cleanup was not called after accepted unusable response")
			}
			if err == nil || !strings.Contains(err.Error(), tc.primary) || !strings.Contains(err.Error(), "cleanup failed") {
				t.Fatalf("combined accepted-start cleanup error = %v", err)
			}
		})
	}
}
