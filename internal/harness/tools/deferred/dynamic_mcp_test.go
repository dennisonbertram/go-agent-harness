package deferred

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// dynMCPCall records a single CallTool invocation for assertions.
type dynMCPCall struct {
	server, tool string
	args         json.RawMessage
}

// dynMCPRegistry is a tools.MCPRegistry stub for DynamicMCPTools tests: it
// returns a configurable multi-server tool listing and records every
// CallTool invocation so tests can verify the generated per-tool handler
// closures forward to the correct (server, original tool name) pair.
type dynMCPRegistry struct {
	byServer map[string][]tools.MCPToolDefinition
	listErr  error
	callLog  []dynMCPCall
}

func (r *dynMCPRegistry) ListResources(context.Context, string) ([]tools.MCPResource, error) {
	return nil, nil
}
func (r *dynMCPRegistry) ReadResource(context.Context, string, string) (string, error) {
	return "", nil
}
func (r *dynMCPRegistry) ListTools(context.Context) (map[string][]tools.MCPToolDefinition, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.byServer, nil
}
func (r *dynMCPRegistry) CallTool(_ context.Context, server, tool string, args json.RawMessage) (string, error) {
	r.callLog = append(r.callLog, dynMCPCall{server, tool, args})
	return fmt.Sprintf("result-from-%s-%s", server, tool), nil
}

// TestDynamicMCPTools_ListToolsErrorPropagates verifies a registry listing
// failure aborts tool generation and returns the underlying error, rather
// than silently producing zero tools.
func TestDynamicMCPTools_ListToolsErrorPropagates(t *testing.T) {
	t.Parallel()

	reg := &dynMCPRegistry{listErr: errors.New("upstream unavailable")}
	tt, err := DynamicMCPTools(context.Background(), reg)
	if err == nil {
		t.Fatal("expected error when ListTools fails")
	}
	if err.Error() != "upstream unavailable" {
		t.Errorf("expected error to propagate verbatim, got %q", err.Error())
	}
	if tt != nil {
		t.Errorf("expected nil tool slice on error, got %v", tt)
	}
}

// TestDynamicMCPTools_SanitizesNamesAndForwardsCallsPerTool verifies that:
//  1. each MCP tool becomes a deferred tool named "mcp_<safe-server>_<safe-tool>"
//     (lowercased, spaces/dots normalized to underscores) while the original,
//     un-sanitized server and tool names are preserved for dispatch;
//  2. the generated Definition carries the right tier/action/mutating/tags and
//     passes the original Parameters through unchanged;
//  3. invoking each tool's handler calls back into the registry with that
//     specific tool's own (server, original name) pair — not some other tool's,
//     which is exactly the kind of bug a closure-over-loop-variable mistake
//     would introduce.
func TestDynamicMCPTools_SanitizesNamesAndForwardsCallsPerTool(t *testing.T) {
	t.Parallel()

	params := map[string]any{"type": "object"}
	reg := &dynMCPRegistry{byServer: map[string][]tools.MCPToolDefinition{
		"My Server": {
			{Name: "Do Thing", Description: "does the thing", Parameters: params},
			{Name: "another.tool", Description: "does another thing"},
		},
		"other-srv": {
			{Name: "solo", Description: "the only tool on this server"},
		},
	}}

	generated, err := DynamicMCPTools(context.Background(), reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(generated) != 3 {
		t.Fatalf("expected 3 generated tools, got %d", len(generated))
	}

	byName := make(map[string]tools.Tool, len(generated))
	for _, tl := range generated {
		byName[tl.Definition.Name] = tl
	}

	wantNames := map[string]struct{ server, orig string }{
		"mcp_my_server_do_thing":     {"My Server", "Do Thing"},
		"mcp_my_server_another_tool": {"My Server", "another.tool"},
		"mcp_other_srv_solo":         {"other-srv", "solo"},
	}
	for name, want := range wantNames {
		tl, ok := byName[name]
		if !ok {
			t.Errorf("expected generated tool named %q, got names: %v", name, keysOf(byName))
			continue
		}
		if tl.Definition.Tier != tools.TierDeferred {
			t.Errorf("%s: expected TierDeferred, got %v", name, tl.Definition.Tier)
		}
		if tl.Definition.Action != tools.ActionExecute {
			t.Errorf("%s: expected ActionExecute, got %v", name, tl.Definition.Action)
		}
		if !tl.Definition.Mutating {
			t.Errorf("%s: expected Mutating=true", name)
		}
		if tl.Definition.ParallelSafe {
			t.Errorf("%s: expected ParallelSafe=false", name)
		}
		if !containsTag(tl.Definition.Tags, "mcp") {
			t.Errorf("%s: expected 'mcp' tag, got %v", name, tl.Definition.Tags)
		}

		// Invoke the handler and confirm it dispatches to *this* tool's
		// original (server, name) pair, not another generated tool's.
		raw := json.RawMessage(fmt.Sprintf(`{"marker":%q}`, name))
		result, err := tl.Handler(context.Background(), raw)
		if err != nil {
			t.Fatalf("%s: unexpected handler error: %v", name, err)
		}
		wantResult := fmt.Sprintf("result-from-%s-%s", want.server, want.orig)
		if result != wantResult {
			t.Errorf("%s: expected handler result %q, got %q", name, wantResult, result)
		}
	}

	if len(reg.callLog) != 3 {
		t.Fatalf("expected 3 recorded CallTool invocations, got %d: %v", len(reg.callLog), reg.callLog)
	}
	for _, call := range reg.callLog {
		want, ok := wantNames["mcp_"+sanitizeToolNamePart(call.server)+"_"+sanitizeToolNamePart(call.tool)]
		if !ok {
			t.Errorf("unexpected call recorded for server=%q tool=%q", call.server, call.tool)
			continue
		}
		if call.server != want.server || call.tool != want.orig {
			t.Errorf("call server/tool mismatch: got (%q,%q), want (%q,%q)", call.server, call.tool, want.server, want.orig)
		}
	}

	// Description and Parameters must pass through unsanitized/unmodified.
	doThing := byName["mcp_my_server_do_thing"]
	if doThing.Definition.Description != "does the thing" {
		t.Errorf("expected description passed through, got %q", doThing.Definition.Description)
	}
	if _, ok := doThing.Definition.Parameters["type"]; !ok {
		t.Errorf("expected original Parameters passed through, got %v", doThing.Definition.Parameters)
	}
}

func keysOf(m map[string]tools.Tool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
