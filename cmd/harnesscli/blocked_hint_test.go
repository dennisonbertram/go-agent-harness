package main

// blocked_hint_test.go — Issue #1374: the blocked-run hint printed by
// reportRunBlocked must name a command that actually resolves the block.
// "harnesscli continue <id> <prompt>" only works once a run is already
// completed; it returns 409 run_not_completed for a run still
// waiting_for_user or waiting_for_approval. These tests pin the corrected,
// per-event-type hint and the new "input"/"approve"/"deny" subcommands that
// make the hint actionable.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/internal/harness"
)

// --- BT-001: run.waiting_for_user hints "harnesscli input", not "continue" ---

func TestReportRunBlocked_WaitingForUserHintsInput(t *testing.T) {
	_, errBuf, restore := captureOutput(t)
	defer restore()

	reportRunBlocked("run_abc", harness.EventRunWaitingForUser)

	got := errBuf.String()
	if !strings.Contains(got, "harnesscli input run_abc") {
		t.Errorf("hint must name the working 'input' command for a question-blocked run, got:\n%s", got)
	}
	if strings.Contains(got, "harnesscli continue run_abc") {
		t.Errorf("hint must not suggest 'continue', which returns 409 for a waiting run, got:\n%s", got)
	}
}

// --- BT-002: tool/plan.approval_required hints "harnesscli approve"/"deny" ---

func TestReportRunBlocked_ApprovalHintsApproveDeny(t *testing.T) {
	for _, et := range []harness.EventType{harness.EventToolApprovalRequired, harness.EventPlanApprovalRequired} {
		t.Run(string(et), func(t *testing.T) {
			_, errBuf, restore := captureOutput(t)
			defer restore()

			reportRunBlocked("run_xyz", et)

			got := errBuf.String()
			for _, want := range []string{"harnesscli approve run_xyz", "harnesscli deny run_xyz"} {
				if !strings.Contains(got, want) {
					t.Errorf("hint must name %q for an approval-blocked run, got:\n%s", want, got)
				}
			}
			if strings.Contains(got, "harnesscli continue run_xyz") {
				t.Errorf("hint must not suggest 'continue' for an approval-blocked run, got:\n%s", got)
			}
		})
	}
}

// --- BT-003: "harnesscli input <id> <q>=<a>" POSTs the right JSON to /input ---

func TestRunInput_PostsAnswers(t *testing.T) {
	var capturedPath, capturedMethod string
	var capturedBody map[string]map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode input body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"status":"accepted"}`)
	}))
	defer ts.Close()

	outBuf, errBuf, restore := captureOutput(t)
	defer restore()

	origClient := requestHTTPClient
	requestHTTPClient = ts.Client()
	defer func() { requestHTTPClient = origClient }()

	code := runInput([]string{"-base-url=" + ts.URL, "run_abc", "Proceed with deploy?=yes"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, errBuf.String())
	}
	if capturedMethod != http.MethodPost || capturedPath != "/v1/runs/run_abc/input" {
		t.Fatalf("expected POST /v1/runs/run_abc/input, got %s %s", capturedMethod, capturedPath)
	}
	if capturedBody["answers"]["Proceed with deploy?"] != "yes" {
		t.Fatalf("answers = %v, want {\"Proceed with deploy?\": \"yes\"}", capturedBody["answers"])
	}
	if !strings.Contains(outBuf.String(), "run_abc") {
		t.Errorf("expected confirmation naming the run ID, got:\n%s", outBuf.String())
	}
}

// --- BT-004: "harnesscli input" reports 404 clearly ---

func TestRunInput_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":"not_found","message":"run \"run_missing\" not found"}}`)
	}))
	defer ts.Close()

	_, errBuf, restore := captureOutput(t)
	defer restore()

	origClient := requestHTTPClient
	requestHTTPClient = ts.Client()
	defer func() { requestHTTPClient = origClient }()

	code := runInput([]string{"-base-url=" + ts.URL, "run_missing", "q=a"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "run_missing") || !strings.Contains(errBuf.String(), "not found") {
		t.Errorf("expected clear not-found message, got:\n%s", errBuf.String())
	}
}

// --- BT-005: "harnesscli input" reports 409 (not waiting for input) clearly ---

func TestRunInput_Conflict(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"error":{"code":"no_pending_input","message":"run is not waiting for user input"}}`)
	}))
	defer ts.Close()

	_, errBuf, restore := captureOutput(t)
	defer restore()

	origClient := requestHTTPClient
	requestHTTPClient = ts.Client()
	defer func() { requestHTTPClient = origClient }()

	code := runInput([]string{"-base-url=" + ts.URL, "run_done", "q=a"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "not waiting for") {
		t.Errorf("expected a message explaining the run is not waiting for input, got:\n%s", errBuf.String())
	}
}

// --- BT-006: "harnesscli approve <id>" POSTs /approve ---

func TestRunApprove_Success(t *testing.T) {
	var capturedPath, capturedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"approved"}`)
	}))
	defer ts.Close()

	outBuf, errBuf, restore := captureOutput(t)
	defer restore()

	origClient := requestHTTPClient
	requestHTTPClient = ts.Client()
	defer func() { requestHTTPClient = origClient }()

	code := runApprove([]string{"-base-url=" + ts.URL, "run_abc"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, errBuf.String())
	}
	if capturedMethod != http.MethodPost || capturedPath != "/v1/runs/run_abc/approve" {
		t.Fatalf("expected POST /v1/runs/run_abc/approve, got %s %s", capturedMethod, capturedPath)
	}
	if !strings.Contains(outBuf.String(), "run_abc") {
		t.Errorf("expected confirmation naming the run ID, got:\n%s", outBuf.String())
	}
}

// --- BT-007: "harnesscli deny <id>" POSTs /deny ---

func TestRunDeny_Success(t *testing.T) {
	var capturedPath, capturedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"denied"}`)
	}))
	defer ts.Close()

	outBuf, errBuf, restore := captureOutput(t)
	defer restore()

	origClient := requestHTTPClient
	requestHTTPClient = ts.Client()
	defer func() { requestHTTPClient = origClient }()

	code := runDeny([]string{"-base-url=" + ts.URL, "run_abc"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%s)", code, errBuf.String())
	}
	if capturedMethod != http.MethodPost || capturedPath != "/v1/runs/run_abc/deny" {
		t.Fatalf("expected POST /v1/runs/run_abc/deny, got %s %s", capturedMethod, capturedPath)
	}
	if !strings.Contains(outBuf.String(), "run_abc") {
		t.Errorf("expected confirmation naming the run ID, got:\n%s", outBuf.String())
	}
}

// --- BT-008: dispatch routes "input"/"approve"/"deny" instead of falling through to run() ---

func TestDispatch_InputApproveDenyRoutedNoID(t *testing.T) {
	for _, cmd := range []string{"input", "approve", "deny"} {
		t.Run(cmd, func(t *testing.T) {
			_, errBuf, restore := captureOutput(t)
			defer restore()

			code := dispatch([]string{cmd})
			if code != 1 {
				t.Fatalf("dispatch(%q) with no ID should return 1; got %d", cmd, code)
			}
			if strings.Contains(errBuf.String(), "prompt is required") {
				t.Errorf("dispatch(%q) should not fall through to run(); got: %s", cmd, errBuf.String())
			}
		})
	}
}

// --- BT-009 (regression): runContinue's 409 for a waiting run names the
// right next command instead of repeating "continue", which just failed. ---

func TestRunContinue_ConflictHintsWaitingForUserCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/runs/run_prev/continue":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"error":{"code":"run_not_completed","message":"run is not completed"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/runs/run_prev":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"run_prev","status":"waiting_for_user"}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	_, errBuf, restore := captureOutput(t)
	defer restore()

	origClient := requestHTTPClient
	requestHTTPClient = ts.Client()
	defer func() { requestHTTPClient = origClient }()

	code := runContinue([]string{"-base-url=" + ts.URL, "run_prev", "follow up"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	got := errBuf.String()
	if !strings.Contains(got, "harnesscli input run_prev") {
		t.Errorf("expected the 409 to hint the working 'input' command, got:\n%s", got)
	}
}
