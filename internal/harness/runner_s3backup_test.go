package harness

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-agent-harness/internal/store"
	"go-agent-harness/internal/store/s3backup"
)

// --- fake S3 server helper ---

type s3Capture struct {
	count    atomic.Int64
	lastKey  string
	lastBody []byte
}

func makeTestS3Server(t *testing.T, capture *s3Capture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capture.lastKey = r.URL.Path
		capture.lastBody = body
		capture.count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// waitForTerminal polls GetRun until the run reaches a terminal status or t
// times out. It returns the final status.
func waitForTerminal(t *testing.T, runner *Runner, runID string) RunStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, ok := runner.GetRun(runID)
		if !ok {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if r.Status == RunStatusCompleted || r.Status == RunStatusFailed || r.Status == RunStatusCancelled {
			return r.Status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for terminal run status")
	return ""
}

// waitForS3Upload polls capture until at least one S3 PUT has been recorded or
// 2 seconds elapse.
func waitForS3Upload(t *testing.T, capture *s3Capture) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if capture.count.Load() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout: S3 PUT never fired")
}

// --- tests ---

// TestRunner_S3Backup_OnCompletion verifies that a successful run triggers
// one S3 PUT with a valid JSONL body.
func TestRunner_S3Backup_OnCompletion(t *testing.T) {
	t.Parallel()

	capture := &s3Capture{}
	srv := makeTestS3Server(t, capture)

	memStore := store.NewMemoryStore()
	cfg := s3backup.Config{
		Bucket:          "test-bucket",
		KeyPrefix:       "backup",
		Region:          "us-east-1",
		AccessKeyID:     "AKIATEST",
		SecretAccessKey: "testsecret",
		EndpointURL:     srv.URL,
	}
	uploader := s3backup.NewUploader(cfg)

	runner := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "done"}}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel: "gpt-4.1-mini",
			MaxSteps:     1,
			Store:        memStore,
			S3Uploader:   uploader,
		},
	)

	run, startErr := runner.StartRun(RunRequest{Prompt: "hello", ConversationID: "conv-s3-1"})
	if startErr != nil {
		t.Fatalf("StartRun: %v", startErr)
	}

	waitForTerminal(t, runner, run.ID)
	waitForS3Upload(t, capture)

	// Verify the S3 key format: /bucket/prefix/conv/run.jsonl
	if !strings.Contains(capture.lastKey, "conv-s3-1") {
		t.Errorf("S3 key %q does not contain conversation ID", capture.lastKey)
	}
	if !strings.Contains(capture.lastKey, run.ID) {
		t.Errorf("S3 key %q does not contain run ID", capture.lastKey)
	}
	if !strings.Contains(capture.lastKey, ".jsonl") {
		t.Errorf("S3 key %q does not end with .jsonl", capture.lastKey)
	}

	// Verify the body is valid JSONL.
	lines := strings.Split(strings.TrimSpace(string(capture.lastBody)), "\n")
	if len(lines) == 0 {
		t.Fatal("S3 body is empty")
	}
	for i, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}

	// First line should be the run header.
	var hdr map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &hdr); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if hdr["type"] != "run" {
		t.Errorf("first line type: got %v, want run", hdr["type"])
	}
}

// TestRunner_S3Backup_NoopWhenUnconfigured verifies that omitting S3Uploader
// causes zero S3 calls (no panic, no error).
func TestRunner_S3Backup_NoopWhenUnconfigured(t *testing.T) {
	t.Parallel()

	memStore := store.NewMemoryStore()
	runner := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "done"}}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel: "gpt-4.1-mini",
			MaxSteps:     1,
			Store:        memStore,
			// S3Uploader intentionally nil — should be a no-op.
		},
	)

	run, startErr := runner.StartRun(RunRequest{Prompt: "hello"})
	if startErr != nil {
		t.Fatalf("StartRun: %v", startErr)
	}

	// Run to completion — should not panic.
	waitForTerminal(t, runner, run.ID)
	// Success: reaching here without panic means the no-op path works.
}

// TestRunner_S3Backup_NoopUploaderNoPanic verifies that using NewNoOpUploader
// explicitly results in zero network calls and no panics.
func TestRunner_S3Backup_NoopUploaderNoPanic(t *testing.T) {
	t.Parallel()

	memStore := store.NewMemoryStore()
	runner := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "done"}}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel: "gpt-4.1-mini",
			MaxSteps:     1,
			Store:        memStore,
			S3Uploader:   s3backup.NewNoOpUploader(),
		},
	)

	run, startErr := runner.StartRun(RunRequest{Prompt: "test"})
	if startErr != nil {
		t.Fatalf("StartRun: %v", startErr)
	}

	waitForTerminal(t, runner, run.ID)
}

// TestRunner_S3Backup_OnFailure verifies that a failing run also triggers
// the S3 backup.
func TestRunner_S3Backup_OnFailure(t *testing.T) {
	t.Parallel()

	capture := &s3Capture{}
	srv := makeTestS3Server(t, capture)

	memStore := store.NewMemoryStore()
	cfg := s3backup.Config{
		Bucket:          "fail-bucket",
		KeyPrefix:       "",
		Region:          "us-east-1",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		EndpointURL:     srv.URL,
	}
	uploader := s3backup.NewUploader(cfg)

	runner := NewRunner(
		&errorProvider{err: ErrRunNotFound}, // provider always returns an error
		NewRegistry(),
		RunnerConfig{
			DefaultModel: "gpt-4.1-mini",
			MaxSteps:     1,
			Store:        memStore,
			S3Uploader:   uploader,
		},
	)

	run, startErr := runner.StartRun(RunRequest{Prompt: "fail me"})
	if startErr != nil {
		t.Fatalf("StartRun: %v", startErr)
	}

	waitForTerminal(t, runner, run.ID)
	waitForS3Upload(t, capture)

	if capture.count.Load() == 0 {
		t.Fatal("expected S3 PUT on run failure, but it was not called")
	}
}

// TestRunner_S3Backup_KeyContainsBucket verifies the PUT URL includes the bucket.
func TestRunner_S3Backup_KeyContainsBucket(t *testing.T) {
	t.Parallel()

	capture := &s3Capture{}
	srv := makeTestS3Server(t, capture)

	memStore := store.NewMemoryStore()
	cfg := s3backup.Config{
		Bucket:          "my-special-bucket",
		KeyPrefix:       "prefix",
		Region:          "eu-west-1",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		EndpointURL:     srv.URL,
	}
	uploader := s3backup.NewUploader(cfg)

	runner := NewRunner(
		&stubProvider{turns: []CompletionResult{{Content: "ok"}}},
		NewRegistry(),
		RunnerConfig{
			DefaultModel: "gpt-4.1-mini",
			MaxSteps:     1,
			Store:        memStore,
			S3Uploader:   uploader,
		},
	)

	run, startErr := runner.StartRun(RunRequest{Prompt: "check bucket"})
	if startErr != nil {
		t.Fatalf("StartRun: %v", startErr)
	}

	waitForTerminal(t, runner, run.ID)
	waitForS3Upload(t, capture)

	if !strings.Contains(capture.lastKey, "my-special-bucket") {
		t.Errorf("S3 path %q does not contain bucket name", capture.lastKey)
	}
}
