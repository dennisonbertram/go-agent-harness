//go:build unix

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	openai "go-agent-harness/internal/provider/openai"
)

// This file exercises the actual daemon binary's shutdown wiring end to end
// (real HTTP server, real signal channel, real bash subprocess, real SQLite
// run store) as a regression guard for issue #1373, distinct from the
// runner-level unit test in internal/harness/runner_shutdown_cancel_test.go:
// that test proves Runner.CancelAllActiveRuns itself works; this one proves
// cmd/harnessd/main.go actually calls it during shutdown. Reverting the
// integration in main.go (while leaving the Runner method intact) would
// leave the bash child alive and the run's persisted status "running" here,
// even though the unit test above would still pass.

// waitForFile polls until path exists and holds non-empty content, returning
// the trimmed content.
func waitForFileContent(t *testing.T, path string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return strings.TrimSpace(string(data))
		}
		if time.Now().After(deadline) {
			t.Fatalf("file %q not readable within %v", path, timeout)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func regressionProcessGone(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return true
	}
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true
	}
	if idx := strings.LastIndexByte(string(data), ')'); idx >= 0 && idx+2 < len(data) {
		return data[idx+2] == 'Z'
	}
	return false
}

// TestShutdownCancelsInflightRunAndKillsBashChild reproduces issue #1373's
// live repro end to end: a run blocked in a foreground bash tool call must
// be killed and marked cancelled — durably, in the run store — when the
// daemon receives a shutdown signal, and no bash child may survive it.
func TestShutdownCancelsInflightRunAndKillsBashChild(t *testing.T) {
	// authDisabledFromEnv and explicitAuthOptOut read the real process
	// environment (not the getenv closure below), so the opt-out has to be
	// set this way for the test server to accept unauthenticated requests.
	t.Setenv("HARNESS_AUTH_DISABLED", "true")

	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "child.pid")
	runDBPath := filepath.Join(tmpDir, "runs.db")

	provider := &scriptedHarnessdProvider{
		turns: []harness.CompletionResult{
			{
				ToolCalls: []harness.ToolCall{{
					ID:   "call-bash-1",
					Name: "bash",
					Arguments: mustMarshalJSON(t, map[string]any{
						"command":         fmt.Sprintf("echo $$ > %s; sleep 30", pidFile),
						"timeout_seconds": 60,
					}),
				}},
			},
			{Content: "done"},
		},
	}

	addr := freeLocalAddr(t)
	env := map[string]string{
		"OPENAI_API_KEY":        "test-key",
		"HARNESS_ADDR":          addr,
		"HARNESS_AUTH_DISABLED": "true",
		"HARNESS_MEMORY_MODE":   "off",
		"HARNESS_RUN_DB":        runDBPath,
		"HARNESS_WORKSPACE":     tmpDir,
	}

	baseURL, shutdown := startHarnessdTestServer(t, env, func(openai.Config) (harness.Provider, error) {
		return provider, nil
	}, "")

	createResp, err := http.Post(baseURL+"/v1/runs", "application/json", strings.NewReader(`{"prompt":"run the blocking command"}`))
	if err != nil {
		shutdown()
		t.Fatalf("POST /v1/runs: %v", err)
	}
	bodyBytes, _ := io.ReadAll(createResp.Body)
	createResp.Body.Close()
	var created struct {
		RunID string `json:"run_id"`
	}
	decodeErr := json.Unmarshal(bodyBytes, &created)
	if decodeErr != nil || created.RunID == "" {
		shutdown()
		t.Fatalf("decode create response: status=%d body=%s err=%v id=%q", createResp.StatusCode, bodyBytes, decodeErr, created.RunID)
	}

	// Wait for the bash child to actually be running before triggering
	// shutdown, so this proves cancellation interrupts a live tool call.
	pidStr := waitForFileContent(t, pidFile, 5*time.Second)
	pid, convErr := strconv.Atoi(pidStr)
	if convErr != nil || pid <= 0 {
		shutdown()
		t.Fatalf("unexpected pid file content %q: %v", pidStr, convErr)
	}

	// shutdown() sends the interrupt signal and waits up to 5s for
	// runWithSignals to return. Before this fix, nothing cancelled the run,
	// so the bash child outlived the daemon and the run stayed "running".
	shutdown()

	if !regressionProcessGone(pid) {
		t.Fatalf("bash child pid %d still alive after daemon shutdown", pid)
	}

	// Restart against the same run store and confirm the run was durably
	// persisted as cancelled — not left "running" forever, per issue #1373.
	restartEnv := map[string]string{
		"OPENAI_API_KEY":        "test-key",
		"HARNESS_ADDR":          freeLocalAddr(t),
		"HARNESS_AUTH_DISABLED": "true",
		"HARNESS_MEMORY_MODE":   "off",
		"HARNESS_RUN_DB":        runDBPath,
		"HARNESS_WORKSPACE":     tmpDir,
	}
	restartBaseURL, restartShutdown := startHarnessdTestServer(t, restartEnv, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	}, "")
	defer restartShutdown()

	persistedResp, err := http.Get(restartBaseURL + "/v1/runs/" + created.RunID)
	if err != nil {
		t.Fatalf("GET persisted run: %v", err)
	}
	defer persistedResp.Body.Close()
	var persisted map[string]any
	if err := json.NewDecoder(persistedResp.Body).Decode(&persisted); err != nil {
		t.Fatalf("decode persisted run: %v", err)
	}
	if got := persisted["status"]; got != string(harness.RunStatusCancelled) {
		t.Fatalf("persisted run status = %v, want %q", got, harness.RunStatusCancelled)
	}
}

func mustMarshalJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}
