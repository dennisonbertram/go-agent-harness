package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
)

// profilesTestServer builds a test server with profiles configured to use empty dirs,
// so only built-in profiles are visible. Auth is disabled.
func profilesTestServer(t *testing.T, projectDir, userDir string) *httptest.Server {
	t.Helper()
	registry := harness.NewRegistry()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
	s := NewWithOptions(ServerOptions{
		Runner:          runner,
		AuthDisabled:    true,
		ProfilesProject: projectDir,
		ProfilesUser:    userDir,
	})
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts
}

// TestListProfilesHandler_ReturnsJSON verifies GET /v1/profiles returns a JSON array.
func TestListProfilesHandler_ReturnsJSON(t *testing.T) {
	t.Parallel()

	ts := profilesTestServer(t, "", "")

	res, err := http.Get(ts.URL + "/v1/profiles")
	if err != nil {
		t.Fatalf("GET /v1/profiles: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var resp struct {
		Profiles []map[string]any `json:"profiles"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Built-in profiles must always be present.
	if len(resp.Profiles) == 0 {
		t.Fatal("expected at least one profile in response")
	}

	// Each profile must have source_tier.
	for i, p := range resp.Profiles {
		if _, ok := p["source_tier"]; !ok {
			t.Errorf("profile[%d] missing source_tier", i)
		}
	}
}

// TestGetProfileHandler_ReturnsProfile verifies GET /v1/profiles/{name} returns a specific profile.
func TestGetProfileHandler_ReturnsProfile(t *testing.T) {
	t.Parallel()

	ts := profilesTestServer(t, "", "")

	res, err := http.Get(ts.URL + "/v1/profiles/full")
	if err != nil {
		t.Fatalf("GET /v1/profiles/full: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var profile map[string]any
	if err := json.NewDecoder(res.Body).Decode(&profile); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if profile["name"] != "full" {
		t.Errorf("expected name 'full', got %v", profile["name"])
	}
	if _, ok := profile["source_tier"]; !ok {
		t.Error("expected source_tier in profile response")
	}
}

// TestGetProfileHandler_NotFound verifies GET /v1/profiles/nonexistent returns 404.
func TestGetProfileHandler_NotFound(t *testing.T) {
	t.Parallel()

	ts := profilesTestServer(t, "", "")

	res, err := http.Get(ts.URL + "/v1/profiles/no-such-profile-xyz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Errorf("expected 404, got %d: %s", res.StatusCode, string(body))
	}
}

// TestListProfilesHandler_MethodNotAllowed verifies POST /v1/profiles returns 405.
func TestListProfilesHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	ts := profilesTestServer(t, "", "")

	res, err := http.Post(ts.URL+"/v1/profiles", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /v1/profiles: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}
