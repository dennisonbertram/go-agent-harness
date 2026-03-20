package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		ProfilesDir:     userDir,
	})
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return ts
}

// profilesTestServerWithDir creates a test HTTP server with a single profiles directory.
func profilesTestServerWithDir(t *testing.T, profilesDir string) *httptest.Server {
	t.Helper()
	return profilesTestServer(t, "", profilesDir)
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

// TestCreateProfileHandler_CreatesProfile verifies POST /v1/profiles/{name} creates a profile.
func TestCreateProfileHandler_CreatesProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ts := profilesTestServerWithDir(t, dir)

	body := `{
		"description": "A test profile",
		"model": "gpt-4.1-mini",
		"max_steps": 10
	}`

	res, err := http.Post(ts.URL+"/v1/profiles/new-profile", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 201, got %d: %s", res.StatusCode, string(b))
	}

	// Verify file was created.
	path := filepath.Join(dir, "new-profile.toml")
	_, statErr := os.Stat(path)
	require.NoError(t, statErr, "expected profile file at %s", path)
}

// TestCreateProfileHandler_RejectsBuiltin verifies POST /v1/profiles/{builtin} returns 409.
func TestCreateProfileHandler_RejectsBuiltin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ts := profilesTestServerWithDir(t, dir)

	body := `{"description": "Shadow built-in", "model": "gpt-4.1-mini", "max_steps": 5}`

	res, err := http.Post(ts.URL+"/v1/profiles/github", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusConflict, res.StatusCode)
}

// TestUpdateProfileHandler_UpdatesProfile verifies PUT /v1/profiles/{name} updates a profile.
func TestUpdateProfileHandler_UpdatesProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Pre-create a profile file.
	content := `[meta]
name = "update-me"
description = "Original"
version = 1
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 5
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "update-me.toml"), []byte(content), 0644))

	ts := profilesTestServerWithDir(t, dir)

	body := `{"description": "Updated", "model": "gpt-4.1", "max_steps": 20}`
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/v1/profiles/update-me", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(b))
	}

	var resp map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&resp))
	assert.Equal(t, "updated", resp["status"])
}

// TestUpdateProfileHandler_RejectsBuiltin verifies PUT /v1/profiles/{builtin} returns 403.
func TestUpdateProfileHandler_RejectsBuiltin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ts := profilesTestServerWithDir(t, dir)

	body := `{"description": "Attempt to modify built-in", "model": "gpt-4.1"}`
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/v1/profiles/full", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusForbidden, res.StatusCode)
}

// TestDeleteProfileHandler_DeletesUserProfile verifies DELETE /v1/profiles/{name} deletes a user profile.
func TestDeleteProfileHandler_DeletesUserProfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Pre-create a profile to delete.
	content := `[meta]
name = "delete-me"
description = "To be deleted"
version = 1
created_by = "user"

[runner]
model = "gpt-4.1-mini"
max_steps = 5
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "delete-me.toml"), []byte(content), 0644))

	ts := profilesTestServerWithDir(t, dir)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/profiles/delete-me", nil)
	require.NoError(t, err)

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(b))
	}

	// File should be gone.
	_, statErr := os.Stat(filepath.Join(dir, "delete-me.toml"))
	require.True(t, os.IsNotExist(statErr), "deleted profile file should not exist")
}

// TestDeleteProfileHandler_BuiltinProtected verifies DELETE /v1/profiles/{builtin} returns 403.
func TestDeleteProfileHandler_BuiltinProtected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ts := profilesTestServerWithDir(t, dir)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/profiles/github", nil)
	require.NoError(t, err)

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusForbidden, res.StatusCode)
}

// TestDeleteProfileHandler_NotFound verifies DELETE /v1/profiles/{nonexistent} returns 404.
func TestDeleteProfileHandler_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ts := profilesTestServerWithDir(t, dir)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/profiles/does-not-exist", nil)
	require.NoError(t, err)

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
}

// TestProfilesHandler_NotConfigured verifies 501 when profiles dir is empty.
func TestProfilesHandler_NotConfigured(t *testing.T) {
	t.Parallel()

	ts := profilesTestServerWithDir(t, "") // no profiles dir

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/profiles/new-profile"},
		{http.MethodPut, "/v1/profiles/some-profile"},
		{http.MethodDelete, "/v1/profiles/some-profile"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body io.Reader
			if tc.method != http.MethodDelete {
				body = bytes.NewBufferString(`{"description":"x","model":"gpt-4.1-mini"}`)
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, body)
			require.NoError(t, err)
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, http.StatusNotImplemented, res.StatusCode)
		})
	}
}
