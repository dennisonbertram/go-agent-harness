package harnessmcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureServer records the body of the last POST it received.
func captureServer(t *testing.T, status int, respBody string) (*httptest.Server, *map[string]any) {
	t.Helper()
	captured := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &captured)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// callStartRun drives the start_run tool handler with raw JSON arguments.
func callStartRun(t *testing.T, baseURL, args string) ToolResult {
	t.Helper()
	h := newStartRunHandler(NewHarnessClient(baseURL))
	res, err := h(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("start_run handler: %v", err)
	}
	return res
}

// TestStartRunForwardsIsolationAndToolScoping is the core of issue #1316: the
// fields that make a delegated run safe must actually reach the server.
func TestStartRunForwardsIsolationAndToolScoping(t *testing.T) {
	srv, captured := captureServer(t, http.StatusOK, `{"run_id":"run-1"}`)

	callStartRun(t, srv.URL, `{
		"prompt":"do the thing",
		"workspace_type":"worktree",
		"extra_dirs":["/tmp/a"],
		"allowed_tools":["read_file"],
		"denied_tools":["bash"],
		"profile":"reviewer",
		"system_prompt":"be terse",
		"provider_name":"openai",
		"reasoning_effort":"high",
		"max_turns":4,
		"plan_mode":true,
		"agent_intent":"general"
	}`)

	c := *captured
	for field, want := range map[string]any{
		"workspace_type":   "worktree",
		"profile":          "reviewer",
		"system_prompt":    "be terse",
		"provider_name":    "openai",
		"reasoning_effort": "high",
		"agent_intent":     "general",
	} {
		if got := c[field]; got != want {
			t.Errorf("posted %s = %v, want %v", field, got, want)
		}
	}
	if c["plan_mode"] != true {
		t.Errorf("posted plan_mode = %v, want true", c["plan_mode"])
	}
	if got := c["max_turns"]; got != float64(4) {
		t.Errorf("posted max_turns = %v, want 4", got)
	}
	for field, want := range map[string]string{
		"allowed_tools": "read_file",
		"denied_tools":  "bash",
		"extra_dirs":    "/tmp/a",
	} {
		list, ok := c[field].([]any)
		if !ok || len(list) != 1 || list[0] != want {
			t.Errorf("posted %s = %v, want [%q]", field, c[field], want)
		}
	}
}

// TestStartRunOmitsUnsetFields is the false-positive control: a prompt-only call
// must not silently opt the caller into isolation or any other new behavior.
func TestStartRunOmitsUnsetFields(t *testing.T) {
	srv, captured := captureServer(t, http.StatusOK, `{"run_id":"run-2"}`)

	callStartRun(t, srv.URL, `{"prompt":"just a prompt"}`)

	c := *captured
	if c["prompt"] != "just a prompt" {
		t.Fatalf("prompt did not survive: %v", c["prompt"])
	}
	for _, field := range []string{
		"workspace_type", "extra_dirs", "allowed_tools", "denied_tools", "profile",
		"system_prompt", "provider_name", "reasoning_effort", "max_turns",
		"plan_mode", "plan_file", "agent_intent", "task_context",
		"model", "conversation_id", "max_steps", "max_cost_usd",
	} {
		if _, present := c[field]; present {
			t.Errorf("unset field %q was posted (%v); it must be omitted", field, c[field])
		}
	}
}

// TestStartRunSurfacesServerValidationError — a field the server rejects must
// reach the caller as an error, not be swallowed into a successful-looking result.
func TestStartRunSurfacesServerValidationError(t *testing.T) {
	srv, _ := captureServer(t, http.StatusBadRequest,
		`{"error":{"code":"invalid_request","message":"unknown workspace_type \"banana\""}}`)

	res := callStartRun(t, srv.URL, `{"prompt":"x","workspace_type":"banana"}`)

	if !res.IsError {
		t.Fatal("a rejected run must return an error result")
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "banana") {
		t.Errorf("error must carry the server's message, got: %q", text)
	}
}

// TestToolsListAdvertisesFullSurface — a tool no client can discover is not exposed.
func TestToolsListAdvertisesFullSurface(t *testing.T) {
	byName := map[string]Tool{}
	for _, tool := range toolDefs() {
		byName[tool.Name] = tool
	}

	for _, name := range []string{
		"start_run", "get_run_status", "wait_for_run", "continue_run", "list_runs",
		"cancel_run", "approve_run", "deny_run", "steer_run",
		"list_models", "list_providers",
	} {
		if _, ok := byName[name]; !ok {
			t.Errorf("tool %q is not advertised in tools/list", name)
		}
	}

	props := byName["start_run"].InputSchema.Properties
	for _, field := range []string{
		"workspace_type", "extra_dirs", "allowed_tools", "denied_tools", "profile",
		"system_prompt", "provider_name", "reasoning_effort", "max_turns", "plan_mode",
	} {
		if _, ok := props[field]; !ok {
			t.Errorf("start_run schema does not advertise %q", field)
		}
	}
}

// TestCancelRunTool checks the run-control tools hit their endpoints.
func TestCancelRunTool(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"run_id":"run-9"}`))
	}))
	defer srv.Close()

	h := newCancelRunHandler(NewHarnessClient(srv.URL))
	res, err := h(context.Background(), json.RawMessage(`{"run_id":"run-9"}`))
	if err != nil {
		t.Fatalf("cancel_run: %v", err)
	}
	if res.IsError {
		t.Fatalf("cancel_run returned an error: %s", res.Content[0].Text)
	}
	if gotPath != "/v1/runs/run-9/cancel" {
		t.Errorf("cancel_run hit %q, want /v1/runs/run-9/cancel", gotPath)
	}
}

// TestListModelsTool makes model discovery reachable without shelling out to curl.
func TestListModelsTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"id":"m1","provider":"p1","context_window":1000}]}`))
	}))
	defer srv.Close()

	h := newListModelsHandler(NewHarnessClient(srv.URL))
	res, err := h(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list_models: %v", err)
	}
	text := res.Content[0].Text
	for _, want := range []string{"m1", "p1"} {
		if !strings.Contains(text, want) {
			t.Errorf("list_models output missing %q: %s", want, text)
		}
	}
}

// TestEveryAdvertisedToolIsDispatchable pins tools/list against the dispatcher.
// A tool advertised but not registered is worse than an absent one: the client
// sees it, calls it, and gets "unknown tool" at the point of use.
func TestEveryAdvertisedToolIsDispatchable(t *testing.T) {
	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})

	for _, tool := range toolDefs() {
		if _, ok := d.tools[tool.Name]; !ok {
			t.Errorf("tool %q is advertised in tools/list but has no handler", tool.Name)
		}
	}
	// And the reverse: a handler nobody can discover is dead weight.
	advertised := map[string]bool{}
	for _, tool := range toolDefs() {
		advertised[tool.Name] = true
	}
	for name := range d.tools {
		if !advertised[name] {
			t.Errorf("handler %q is registered but not advertised in tools/list", name)
		}
	}
}
