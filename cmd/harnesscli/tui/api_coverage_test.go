package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStartRunCmdIncludesWorkspacePath(t *testing.T) {
	t.Parallel()

	var got runCreateRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/runs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(runCreateResponse{RunID: "run-workspace"}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer ts.Close()

	msg := startRunCmd(ts.URL, "hello", "", "gpt-test", "openai", "low", "default", "/tmp/project-root", "", nil, nil)()
	if _, ok := msg.(RunStartedMsg); !ok {
		t.Fatalf("expected RunStartedMsg, got %T: %+v", msg, msg)
	}
	if got.WorkspacePath != "/tmp/project-root" {
		t.Fatalf("workspace_path = %q, want /tmp/project-root", got.WorkspacePath)
	}
}

// TestInitCommand_MissingRunIDStartResponseCannotLeakIntoLaterRun proves a
// malformed but successful-looking run-create response fails closed. Without
// a run ID, /init must be abandoned before a later ordinary run can bind its
// pending AGENTS.md write.
func TestInitCommand_MissingRunIDStartResponseCannotLeakIntoLaterRun(t *testing.T) {
	ws := t.TempDir()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	cfg := DefaultTUIConfig()
	cfg.BaseURL = ts.URL
	cfg.Workspace = ws
	m := New(cfg)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = m2.(Model)

	cmds, _ := executeInitCommand(&m, Command{Name: "init", Raw: "/init"})
	if len(cmds) < 2 {
		t.Fatal("/init must queue a start command")
	}
	msg := cmds[len(cmds)-1]()
	if _, ok := msg.(RunFailedMsg); !ok {
		t.Fatalf("missing run_id must return RunFailedMsg, got %T: %#v", msg, msg)
	}
	m2, _ = m.Update(msg)
	m = m2.(Model)

	// Even a later close notification and ordinary run must not revive /init.
	m2, _ = m.Update(SSEDoneMsg{EventType: "bridge.closed"})
	m = m2.(Model)
	m2, _ = m.Update(RunStartedMsg{RunID: "run-next"})
	m = m2.(Model)
	m2, _ = m.Update(AssistantDeltaMsg{Delta: "# Ordinary Run\n"})
	m = m2.(Model)
	m2, _ = m.Update(RunCompletedMsg{RunID: "run-next"})
	m = m2.(Model)

	if _, err := os.Stat(filepath.Join(ws, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("missing-ID /init start leaked into later run; AGENTS.md stat err = %v", err)
	}
}

// TestStartRunCmdNormalizesAndValidatesRunID keeps protocol identities exact:
// accepted IDs are whitespace-normalized, while a blank JSON value is the same
// invalid response as an omitted run_id.
func TestStartRunCmdNormalizesAndValidatesRunID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		response  string
		wantRunID string
	}{
		{name: "surrounding whitespace is trimmed", response: `{"run_id":"  run-trimmed  "}`, wantRunID: "run-trimmed"},
		{name: "whitespace only is rejected", response: `{"run_id":" \t\n "}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.response))
			}))
			defer ts.Close()

			msg := startRunCmd(ts.URL, "hello", "", "gpt-test", "openai", "", "default", "/tmp/ws", "", nil, nil)()
			if tc.wantRunID == "" {
				if _, ok := msg.(RunFailedMsg); !ok {
					t.Fatalf("blank run_id must return RunFailedMsg, got %T: %#v", msg, msg)
				}
				return
			}
			started, ok := msg.(RunStartedMsg)
			if !ok {
				t.Fatalf("expected RunStartedMsg, got %T: %#v", msg, msg)
			}
			if started.RunID != tc.wantRunID {
				t.Fatalf("RunID = %q, want %q", started.RunID, tc.wantRunID)
			}
		})
	}
}

func TestStartRunCmdSendsCapabilityProfileAsProfileField(t *testing.T) {
	t.Parallel()

	var rawBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&rawBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(runCreateResponse{RunID: "run-profile"})
	}))
	defer ts.Close()

	// A capability profile selected via /profiles (e.g. "researcher") must be
	// sent in the "profile" field (harness.RunRequest.ProfileName), NOT in
	// "prompt_profile" — the server rejects unknown prompt profiles with HTTP 400.
	msg := startRunCmd(ts.URL, "hello", "", "gpt-test", "openai", "", "researcher", "/tmp/x", "", nil, nil)()
	if _, ok := msg.(RunStartedMsg); !ok {
		t.Fatalf("expected RunStartedMsg, got %T: %+v", msg, msg)
	}
	if got, ok := rawBody["profile"]; !ok || got != "researcher" {
		t.Errorf(`request must include "profile":"researcher"; got profile=%v (present=%v)`, got, ok)
	}
	if _, ok := rawBody["prompt_profile"]; ok {
		t.Errorf(`capability profile must NOT be sent as "prompt_profile"; body=%v`, rawBody)
	}
}

func TestLoadSubagentsCmdReturnsDecodedSubagents(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/subagents" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"subagents": []RemoteSubagent{{
				ID:            "sub-1",
				Status:        "running",
				Isolation:     "worktree",
				CleanupPolicy: "destroy",
			}},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer ts.Close()

	msg := loadSubagentsCmd(ts.URL, "")()
	loaded, ok := msg.(SubagentsLoadedMsg)
	if !ok {
		t.Fatalf("expected SubagentsLoadedMsg, got %T", msg)
	}
	if len(loaded.Subagents) != 1 || loaded.Subagents[0].ID != "sub-1" {
		t.Fatalf("unexpected subagents payload: %+v", loaded.Subagents)
	}
}

func TestFormatRunErrorRendersJSONFields(t *testing.T) {
	t.Parallel()

	lines := formatRunError(`provider completion failed: {"error":{"message":"boom","type":"invalid_request"},"request_id":"req_123","ignored":null}`)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "✗ provider completion failed") {
		t.Fatalf("expected failure prefix, got %q", joined)
	}
	if !strings.Contains(joined, "message: boom") {
		t.Fatalf("expected nested message field, got %q", joined)
	}
	if !strings.Contains(joined, "type: invalid_request") {
		t.Fatalf("expected nested type field, got %q", joined)
	}
	if !strings.Contains(joined, "request_id: req_123") {
		t.Fatalf("expected top-level request id, got %q", joined)
	}
	if strings.Contains(joined, "ignored") {
		t.Fatalf("expected nil field to be omitted, got %q", joined)
	}
}

func TestFlattenJSONRendersNestedMapsAndSkipsNil(t *testing.T) {
	t.Parallel()

	lines := flattenJSON(map[string]any{
		"outer": map[string]any{"inner": "value"},
		"count": 3,
		"skip":  nil,
	}, "  ")
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "outer:") {
		t.Fatalf("expected parent key, got %q", joined)
	}
	if !strings.Contains(joined, "inner: value") {
		t.Fatalf("expected nested key/value, got %q", joined)
	}
	if !strings.Contains(joined, "count: 3") {
		t.Fatalf("expected scalar field, got %q", joined)
	}
	if strings.Contains(joined, "skip") {
		t.Fatalf("expected nil field to be skipped, got %q", joined)
	}
}

func TestStartRunCmdSetsAllowFallback(t *testing.T) {
	t.Parallel()

	var got runCreateRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/runs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(runCreateResponse{RunID: "run-fallback"}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer ts.Close()

	msg := startRunCmd(ts.URL, "hello", "", "gpt-test", "openai", "low", "default", "", "", nil, nil)()
	if _, ok := msg.(RunStartedMsg); !ok {
		t.Fatalf("expected RunStartedMsg, got %T: %+v", msg, msg)
	}
	if !got.AllowFallback {
		t.Fatalf("expected allow_fallback=true in POST body, got false")
	}
}

func TestFormatSubagentsLinesRendersSummaryAndDetails(t *testing.T) {
	t.Parallel()

	if got := formatSubagentsLines(nil, nil); len(got) != 1 || got[0] != "No managed subagents." {
		t.Fatalf("unexpected empty-state lines: %v", got)
	}

	lines := formatSubagentsLines([]RemoteSubagent{{
		ID:               "sub-1",
		Status:           "completed",
		Isolation:        "worktree",
		CleanupPolicy:    "destroy",
		WorkspaceCleaned: true,
		BranchName:       "codex/coverage-fix",
		BaseRef:          "main",
		WorkspacePath:    "/tmp/sub-1",
	}}, nil)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "sub-1 [completed] worktree (destroy) cleaned") {
		t.Fatalf("expected summary line, got %q", joined)
	}
	if !strings.Contains(joined, "branch=codex/coverage-fix") || !strings.Contains(joined, "base=main") || !strings.Contains(joined, "path=/tmp/sub-1") {
		t.Fatalf("expected detail line, got %q", joined)
	}
}
