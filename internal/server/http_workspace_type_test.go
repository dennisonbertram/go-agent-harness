package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
)

func TestPostRunWorkspaceTypeUnknownReturnsWorkspaceUnsupported(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	res := serveWorkspaceTypeRequest(handler, http.MethodPost, "/v1/runs", `{
		"prompt": "hello",
		"workspace_type": "wroktree"
	}`)
	defer res.Body.Close()

	body := readWorkspaceTypeErrorBody(t, res)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", res.StatusCode, body.Raw)
	}
	if body.Error != "workspace_unsupported" {
		t.Fatalf("error = %q, want workspace_unsupported: %s", body.Error, body.Raw)
	}
	if !strings.Contains(body.Message, `workspace_type="wroktree"`) {
		t.Fatalf("message should name invalid workspace_type, got %q", body.Message)
	}
	if strings.Contains(body.Raw, "run_id") || strings.Contains(body.Raw, "queued") {
		t.Fatalf("malformed workspace request should not create a queued run: %s", body.Raw)
	}
}

func TestPostRunWorkspaceTypeWorktreeWithoutRepoReturnsWorkspaceUnsupported(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	res := serveWorkspaceTypeRequest(handler, http.MethodPost, "/v1/runs", `{
		"prompt": "hello",
		"workspace_type": "worktree"
	}`)
	defer res.Body.Close()

	body := readWorkspaceTypeErrorBody(t, res)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", res.StatusCode, body.Raw)
	}
	if body.Error != "workspace_unsupported" {
		t.Fatalf("error = %q, want workspace_unsupported: %s", body.Error, body.Raw)
	}
	for _, want := range []string{"workspace_type=worktree", "HARNESS_WORKSPACE", "git repo"} {
		if !strings.Contains(body.Message, want) {
			t.Fatalf("message should contain %q, got %q", want, body.Message)
		}
	}
	if strings.Contains(body.Raw, "run_id") || strings.Contains(body.Raw, "queued") {
		t.Fatalf("unconfigured worktree request should not create a queued run: %s", body.Raw)
	}
}

func TestPostRunWorkspaceTypeContainerReturnsWorkspaceUnsupported(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	res := serveWorkspaceTypeRequest(handler, http.MethodPost, "/v1/runs", `{
		"prompt": "hello",
		"workspace_type": "container"
	}`)
	defer res.Body.Close()

	body := readWorkspaceTypeErrorBody(t, res)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", res.StatusCode, body.Raw)
	}
	if body.Error != "workspace_unsupported" {
		t.Fatalf("error = %q, want workspace_unsupported: %s", body.Error, body.Raw)
	}
	for _, want := range []string{"workspace_type=container", "standalone harnessd", "container workspace provider"} {
		if !strings.Contains(body.Message, want) {
			t.Fatalf("message should contain %q, got %q", want, body.Message)
		}
	}
	if strings.Contains(body.Raw, "run_id") || strings.Contains(body.Raw, "queued") {
		t.Fatalf("unconfigured container request should not create a queued run: %s", body.Raw)
	}
}

func TestPostRunWorkspaceTypeVMReturnsWorkspaceUnsupported(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	res := serveWorkspaceTypeRequest(handler, http.MethodPost, "/v1/runs", `{
		"prompt": "hello",
		"workspace_type": "vm"
	}`)
	defer res.Body.Close()

	body := readWorkspaceTypeErrorBody(t, res)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", res.StatusCode, body.Raw)
	}
	if body.Error != "workspace_unsupported" {
		t.Fatalf("error = %q, want workspace_unsupported: %s", body.Error, body.Raw)
	}
	for _, want := range []string{"workspace_type=vm", "standalone harnessd", "VM workspace provider"} {
		if !strings.Contains(body.Message, want) {
			t.Fatalf("message should contain %q, got %q", want, body.Message)
		}
	}
	if strings.Contains(body.Raw, "run_id") || strings.Contains(body.Raw, "queued") {
		t.Fatalf("unconfigured VM request should not create a queued run: %s", body.Raw)
	}
}

func TestPostRunWorkspaceTypeLocalStillEmitsProvisioned(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		harness.NewRegistry(),
		harness.RunnerConfig{DefaultModel: "gpt-4.1-mini", MaxSteps: 1},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner, AuthDisabled: true})
	res := serveWorkspaceTypeRequest(handler, http.MethodPost, "/v1/runs", `{
		"prompt": "hello",
		"workspace_type": "local"
	}`)
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want 202: %s", res.StatusCode, string(raw))
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.RunID == "" {
		t.Fatal("expected run_id")
	}

	eventsRes := serveWorkspaceTypeRequest(handler, http.MethodGet, "/v1/runs/"+created.RunID+"/events", "")
	defer eventsRes.Body.Close()
	rawEvents, err := io.ReadAll(eventsRes.Body)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(rawEvents), "event: workspace.provisioned") {
		t.Fatalf("expected workspace.provisioned event, got: %s", string(rawEvents))
	}
}

func serveWorkspaceTypeRequest(handler http.Handler, method, path, body string) *http.Response {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Result()
}

type workspaceTypeErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Raw     string `json:"-"`
}

func readWorkspaceTypeErrorBody(t *testing.T, res *http.Response) workspaceTypeErrorBody {
	t.Helper()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	var body workspaceTypeErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode response body %q: %v", string(raw), err)
	}
	body.Raw = string(raw)
	return body
}
