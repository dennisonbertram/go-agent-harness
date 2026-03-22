package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestListProfiles_Success verifies listProfilesCmd prints profiles and returns 0.
func TestListProfiles_Success(t *testing.T) {
	profiles := []map[string]any{
		{
			"name":        "alpha",
			"description": "Alpha profile",
			"model":       "gpt-4",
		},
		{
			"name":        "beta",
			"description": "Beta profile",
			"model":       "claude-opus-4-6",
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/profiles" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"profiles": profiles,
			"count":    len(profiles),
		})
	}))
	defer ts.Close()

	var out strings.Builder
	origStdout := stdout
	stdout = &out
	defer func() { stdout = origStdout }()

	code := listProfilesCmd(requestHTTPClient, ts.URL)
	if code != 0 {
		t.Errorf("expected exit code 0; got %d\noutput: %s", code, out.String())
	}
	output := out.String()
	if !strings.Contains(output, "alpha") {
		t.Errorf("output should contain profile name 'alpha'; got:\n%s", output)
	}
	if !strings.Contains(output, "beta") {
		t.Errorf("output should contain profile name 'beta'; got:\n%s", output)
	}
	if !strings.Contains(output, "gpt-4") {
		t.Errorf("output should contain model 'gpt-4'; got:\n%s", output)
	}
}

// TestListProfiles_EmptyList returns 0 and shows no-profiles message.
func TestListProfiles_EmptyList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"profiles": []any{},
			"count":    0,
		})
	}))
	defer ts.Close()

	var out strings.Builder
	origStdout := stdout
	stdout = &out
	defer func() { stdout = origStdout }()

	code := listProfilesCmd(requestHTTPClient, ts.URL)
	if code != 0 {
		t.Errorf("expected exit code 0; got %d", code)
	}
	output := out.String()
	if !strings.Contains(output, "No profiles") {
		t.Errorf("output should mention 'No profiles'; got:\n%s", output)
	}
}

// TestListProfiles_ServerError returns exit code 1 on non-200 response.
func TestListProfiles_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"server error"}}`))
	}))
	defer ts.Close()

	var out strings.Builder
	origStderr := stderr
	stderr = &out
	defer func() { stderr = origStderr }()

	code := listProfilesCmd(requestHTTPClient, ts.URL)
	if code != 1 {
		t.Errorf("expected exit code 1 on server error; got %d", code)
	}
}

// TestListProfiles_NetworkError returns exit code 1 when server is unreachable.
func TestListProfiles_NetworkError(t *testing.T) {
	var errOut strings.Builder
	origStderr := stderr
	stderr = &errOut
	defer func() { stderr = origStderr }()

	// Use an invalid URL to force a network error.
	code := listProfilesCmd(requestHTTPClient, "http://localhost:0")
	if code != 1 {
		t.Errorf("expected exit code 1 on network error; got %d", code)
	}
}

// TestListProfiles_OutputFormat verifies that output is sorted by name.
func TestListProfiles_OutputFormat(t *testing.T) {
	profiles := []map[string]any{
		{"name": "zebra", "description": "Zebra", "model": "gpt-4"},
		{"name": "apple", "description": "Apple", "model": "gpt-4"},
		{"name": "mango", "description": "Mango", "model": "gpt-4"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"profiles": profiles,
			"count":    len(profiles),
		})
	}))
	defer ts.Close()

	var out strings.Builder
	origStdout := stdout
	stdout = &out
	defer func() { stdout = origStdout }()

	code := listProfilesCmd(requestHTTPClient, ts.URL)
	if code != 0 {
		t.Errorf("expected exit code 0; got %d", code)
	}
	output := out.String()

	// Verify sorted order: apple < mango < zebra.
	applePos := strings.Index(output, "apple")
	mangoPos := strings.Index(output, "mango")
	zebraPos := strings.Index(output, "zebra")
	if applePos < 0 || mangoPos < 0 || zebraPos < 0 {
		t.Fatalf("output missing expected names:\n%s", output)
	}
	if !(applePos < mangoPos && mangoPos < zebraPos) {
		t.Errorf("output not sorted: apple=%d mango=%d zebra=%d\n%s", applePos, mangoPos, zebraPos, output)
	}
}
