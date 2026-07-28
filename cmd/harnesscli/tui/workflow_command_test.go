package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWorkflowCommandRegistered verifies /workflow dispatches (not CmdUnknown)
// and appears alongside existing commands without shadowing them.
func TestWorkflowCommandRegistered(t *testing.T) {
	t.Parallel()
	reg := NewCommandRegistry()
	cmd, ok := ParseCommand("/workflow")
	if !ok {
		t.Fatal("parse failed")
	}
	result := reg.Dispatch(cmd)
	if result.Status == CmdUnknown {
		t.Fatalf("/workflow is not registered: %+v", result)
	}

	for _, name := range []string{"runs", "hooks", "dashboard"} {
		c, _ := ParseCommand("/" + name)
		if r := reg.Dispatch(c); r.Status == CmdUnknown {
			t.Errorf("/%s became unknown after adding /workflow", name)
		}
	}
}

// TestListScriptWorkflowsCmdDecodes verifies the API client fetches and
// decodes the /v1/script-workflows payload.
func TestListScriptWorkflowsCmdDecodes(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/script-workflows" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflows": []map[string]any{{"name": "swarm", "description": "planner/worker fan-out"}},
		})
	}))
	defer ts.Close()

	msg := listScriptWorkflowsCmd(ts.URL, "")()
	loaded, ok := msg.(ScriptWorkflowsListedMsg)
	if !ok {
		t.Fatalf("expected ScriptWorkflowsListedMsg, got %T (%+v)", msg, msg)
	}
	if len(loaded.Workflows) != 1 || loaded.Workflows[0].Name != "swarm" {
		t.Fatalf("workflows payload: %+v", loaded.Workflows)
	}
}

// TestStartScriptWorkflowCmdDecodes verifies the API client posts args and
// decodes the 202 response from /v1/script-workflows/{name}/runs.
func TestStartScriptWorkflowCmdDecodes(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/script-workflows/swarm/runs" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Args map[string]any `json:"args"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Args["goal"] != "ship it" {
			t.Fatalf("args not forwarded: %+v", body.Args)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"run_id": "wf_1", "status": "queued", "workflow_name": "swarm",
		})
	}))
	defer ts.Close()

	msg := startScriptWorkflowCmd(ts.URL, "swarm", map[string]any{"goal": "ship it"}, "")()
	started, ok := msg.(ScriptWorkflowStartedMsg)
	if !ok {
		t.Fatalf("expected ScriptWorkflowStartedMsg, got %T (%+v)", msg, msg)
	}
	if started.RunID != "wf_1" || started.Status != "queued" || started.WorkflowName != "swarm" {
		t.Fatalf("unexpected started msg: %+v", started)
	}
}

// TestGetScriptWorkflowRunCmdDecodes verifies the API client fetches and
// decodes a run's status/result from /v1/script-workflow-runs/{id}.
func TestGetScriptWorkflowRunCmdDecodes(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/script-workflow-runs/wf_1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "wf_1", "workflow_name": "swarm", "status": "completed", "result_json": `{"ok":true}`,
		})
	}))
	defer ts.Close()

	msg := getScriptWorkflowRunCmd(ts.URL, "wf_1", "")()
	run, ok := msg.(ScriptWorkflowRunFetchedMsg)
	if !ok {
		t.Fatalf("expected ScriptWorkflowRunFetchedMsg, got %T (%+v)", msg, msg)
	}
	if run.Status != "completed" || run.ResultJSON != `{"ok":true}` {
		t.Fatalf("unexpected run msg: %+v", run)
	}
}

// TestExecuteWorkflowCommandDispatch covers the three argument shapes
// executeWorkflowCommand routes between: list, status, and start-with-args.
func TestExecuteWorkflowCommandDispatch(t *testing.T) {
	t.Parallel()
	m := &Model{config: TUIConfig{BaseURL: "http://example.invalid"}}

	t.Run("no args lists", func(t *testing.T) {
		cmds, handled := executeWorkflowCommand(m, Command{})
		if handled {
			t.Fatal("expected handled=false (async tea.Cmd)")
		}
		if len(cmds) != 2 {
			t.Fatalf("expected status + list cmd, got %d", len(cmds))
		}
	})

	t.Run("status with no run-id errors", func(t *testing.T) {
		cmds, _ := executeWorkflowCommand(m, Command{Args: []string{"status"}})
		if len(cmds) != 1 {
			t.Fatalf("expected a single usage-error status cmd, got %d", len(cmds))
		}
	})

	t.Run("invalid json args errors", func(t *testing.T) {
		cmds, _ := executeWorkflowCommand(m, Command{Args: []string{"swarm", "{not-json}"}})
		if len(cmds) != 1 {
			t.Fatalf("expected a single error status cmd, got %d", len(cmds))
		}
	})

	t.Run("name with valid json starts", func(t *testing.T) {
		cmds, _ := executeWorkflowCommand(m, Command{Args: []string{"swarm", `{"goal":"ship"}`}})
		if len(cmds) != 2 {
			t.Fatalf("expected status + start cmd, got %d", len(cmds))
		}
	})
}

// TestFormatScriptWorkflowLines covers the rendered list and the empty state.
func TestFormatScriptWorkflowLines(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()
		lines := formatScriptWorkflowLines(ScriptWorkflowsListedMsg{
			Workflows: []scriptWorkflowMeta{{Name: "swarm", Description: "planner/worker fan-out"}},
		})
		joined := strings.Join(lines, "\n")
		for _, want := range []string{"swarm", "planner/worker fan-out"} {
			if !strings.Contains(joined, want) {
				t.Errorf("rendered output missing %q:\n%s", want, joined)
			}
		}
	})

	t.Run("empty state", func(t *testing.T) {
		t.Parallel()
		lines := formatScriptWorkflowLines(ScriptWorkflowsListedMsg{})
		joined := strings.Join(lines, "\n")
		if !strings.Contains(joined, "No script workflows registered") {
			t.Errorf("empty state should say no workflows registered:\n%s", joined)
		}
	})
}
