package server

// http_workspace_path_test.go — POST /v1/runs validates workspace_path (issue
// #1372) synchronously, mirroring http_extra_dirs_test.go: a relative or
// nonexistent path gets a 400 at creation instead of being silently dropped
// (the pre-fix behavior, which ran tools under the daemon's own workspace).

import (
	"net/http"
	"strings"
	"testing"
)

func TestPostRunWorkspacePathNonexistentReturns400(t *testing.T) {
	t.Parallel()

	ts := newExtraDirsTestServer(t)
	status, raw := postExtraDirsRun(t, ts, `{"prompt":"hello","workspace_path":"/definitely/does/not/exist"}`)

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", status, raw)
	}
	if !strings.Contains(raw, "workspace_path") {
		t.Fatalf("error body should name workspace_path, got %s", raw)
	}
	if strings.Contains(raw, "run_id") {
		t.Fatalf("invalid workspace_path must not create a run: %s", raw)
	}
}

func TestPostRunWorkspacePathRelativeReturns400(t *testing.T) {
	t.Parallel()

	ts := newExtraDirsTestServer(t)
	status, raw := postExtraDirsRun(t, ts, `{"prompt":"hello","workspace_path":"relative/dir"}`)

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", status, raw)
	}
	if !strings.Contains(raw, "workspace_path") {
		t.Fatalf("error body should name workspace_path, got %s", raw)
	}
}

func TestPostRunWorkspacePathValidAccepted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	body := `{"prompt":"hello","workspace_path":"` + dir + `"}`

	ts := newExtraDirsTestServer(t)
	status, raw := postExtraDirsRun(t, ts, body)

	if status != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", status, raw)
	}
	if !strings.Contains(raw, "run_id") {
		t.Fatalf("expected run_id in create response: %s", raw)
	}
}
