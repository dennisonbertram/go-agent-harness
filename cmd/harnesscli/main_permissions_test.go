package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRunParsesSandboxAndNetworkFlagsIntoPermissions verifies that --sandbox
// and --network populate a "permissions" object on the run-create request
// body (issue #1397). When neither flag is set, "permissions" must be
// entirely absent so the server falls back to its own defaults.
func TestRunParsesSandboxAndNetworkFlagsIntoPermissions(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"run_id":"run_perm","status":"queued"}`)
	})
	mux.HandleFunc("/v1/runs/run_perm/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = io.WriteString(w, "event: run.completed\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"e1\",\"run_id\":\"run_perm\",\"type\":\"run.completed\"}\n\n")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	origRequestClient := requestHTTPClient
	origStreamClient := streamHTTPClient
	origStdout := stdout
	origStderr := stderr
	defer func() {
		requestHTTPClient = origRequestClient
		streamHTTPClient = origStreamClient
		stdout = origStdout
		stderr = origStderr
	}()

	requestHTTPClient = ts.Client()
	streamHTTPClient = ts.Client()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	code := run([]string{
		"-base-url=" + ts.URL,
		"-prompt=do work",
		"-sandbox=local",
		"-network=deny",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		t.Fatalf("decode captured request body: %v; raw=%s", err, rawBody)
	}
	permsRaw, ok := body["permissions"]
	if !ok {
		t.Fatalf("expected \"permissions\" key in request body, got: %s", rawBody)
	}
	perms, ok := permsRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected \"permissions\" to be an object, got: %T (%v)", permsRaw, permsRaw)
	}
	if perms["sandbox"] != "local" {
		t.Errorf("expected permissions.sandbox=%q, got %v", "local", perms["sandbox"])
	}
	if perms["network"] != "deny" {
		t.Errorf("expected permissions.network=%q, got %v", "deny", perms["network"])
	}
	if approval, ok := perms["approval"]; !ok || approval != "" {
		t.Errorf("expected permissions.approval to be present and empty (matching the struct's json tag), got %v (present=%v)", approval, ok)
	}
}

// TestRunOmitsPermissionsWhenNoSandboxOrNetworkFlagSet verifies that a plain
// run request (no --sandbox, no --network) sends no "permissions" field at
// all, leaving the server's own defaults in effect (issue #1397).
func TestRunOmitsPermissionsWhenNoSandboxOrNetworkFlagSet(t *testing.T) {
	var rawBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		rawBody = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"run_id":"run_noperm","status":"queued"}`)
	})
	mux.HandleFunc("/v1/runs/run_noperm/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = io.WriteString(w, "event: run.completed\n")
		_, _ = io.WriteString(w, "data: {\"id\":\"e1\",\"run_id\":\"run_noperm\",\"type\":\"run.completed\"}\n\n")
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	origRequestClient := requestHTTPClient
	origStreamClient := streamHTTPClient
	origStdout := stdout
	origStderr := stderr
	defer func() {
		requestHTTPClient = origRequestClient
		streamHTTPClient = origStreamClient
		stdout = origStdout
		stderr = origStderr
	}()

	requestHTTPClient = ts.Client()
	streamHTTPClient = ts.Client()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}

	code := run([]string{
		"-base-url=" + ts.URL,
		"-prompt=do work",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		t.Fatalf("decode captured request body: %v; raw=%s", err, rawBody)
	}
	if _, ok := body["permissions"]; ok {
		t.Errorf("expected no \"permissions\" key when neither flag is set, got: %s", rawBody)
	}
}
