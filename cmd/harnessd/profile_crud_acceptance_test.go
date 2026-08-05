package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-agent-harness/internal/fakeprovider"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/profiles"
	openai "go-agent-harness/internal/provider/openai"
)

// TestHarnessdProfileCRUDUsesIsolatedAbsoluteDirectory is a real daemon
// acceptance test. It owns the listener, never changes HOME, drives every
// HTTP mutation in one runtime, then drives every equivalent agent tool across
// three fake-provider turns. Before #1187 the first HTTP create returns 501
// because production composition never supplied ProfilesDir.
func TestHarnessdProfileCRUDUsesIsolatedAbsoluteDirectory(t *testing.T) {
	workspace := t.TempDir()
	profilesDir := filepath.Join(t.TempDir(), "isolated-profiles")
	projectProfilesDir := filepath.Join(workspace, ".harness", "profiles")
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	env := baseEnv("127.0.0.1:0")
	env["HARNESS_WORKSPACE"] = workspace
	env["HARNESS_PROFILES_DIR"] = profilesDir
	disableCallbacksForUnrelatedHarnessFixture(env)

	provider := fakeprovider.New([]fakeprovider.Turn{
		{ToolCalls: []harness.ToolCall{{
			ID: "create-agent-profile", Name: "create_profile",
			Arguments: `{"name":"agent-managed","description":"created by agent","model":"fake-model","max_steps":2}`,
		}}},
		{Content: "agent profile created"},
		{ToolCalls: []harness.ToolCall{{
			ID: "update-agent-profile", Name: "update_profile",
			Arguments: `{"name":"agent-managed","description":"updated by agent","max_steps":3}`,
		}}},
		{Content: "agent profile updated"},
		{ToolCalls: []harness.ToolCall{{
			ID: "delete-agent-profile", Name: "delete_profile",
			Arguments: `{"name":"agent-managed"}`,
		}}},
		{Content: "agent profile deleted"},
	})

	runHarnessdProfileAcceptance(t, env, provider, func(baseURL string) {
		writeAcceptanceProfile(t, profilesDir, "precedence-profile", "user profile")
		writeAcceptanceProfile(t, projectProfilesDir, "precedence-profile", "project profile")
		assertProfileResponse(t, baseURL, "precedence-profile", "project profile", http.StatusOK)
		assertProfileToolsExposed(t, baseURL)

		// HTTP create -> read -> update -> read -> delete -> not-found is the
		// externally observable API contract in one daemon lifetime.
		profileRequest(t, baseURL, http.MethodPost, "http-managed", `{"description":"created over HTTP","model":"fake-model","max_steps":2}`, http.StatusCreated)
		assertProfileResponse(t, baseURL, "http-managed", "created over HTTP", http.StatusOK)
		profileRequest(t, baseURL, http.MethodPut, "http-managed", `{"description":"updated over HTTP","max_steps":4}`, http.StatusOK)
		assertProfileResponse(t, baseURL, "http-managed", "updated over HTTP", http.StatusOK)
		profileRequest(t, baseURL, http.MethodDelete, "http-managed", "", http.StatusOK)
		assertProfileResponse(t, baseURL, "http-managed", "", http.StatusNotFound)

		for _, prompt := range []string{"create the agent profile", "update the agent profile", "delete the agent profile"} {
			runID := startProfileAcceptanceRun(t, baseURL, prompt)
			terminal := awaitRunTerminalState(t, baseURL, runID, 5*time.Second)
			if terminal["status"] != string(harness.RunStatusCompleted) {
				t.Fatalf("run %s status = %#v", runID, terminal)
			}
		}
		assertProfileResponse(t, baseURL, "agent-managed", "", http.StatusNotFound)

		if _, err := os.Stat(filepath.Join(profilesDir, "http-managed.toml")); !os.IsNotExist(err) {
			t.Fatalf("isolated HTTP profile should have been deleted, stat err=%v", err)
		}
		if _, err := os.Stat(filepath.Join(profilesDir, "agent-managed.toml")); !os.IsNotExist(err) {
			t.Fatalf("isolated agent profile should have been deleted, stat err=%v", err)
		}
		if _, err := os.Stat(filepath.Join(realHome, ".harness", "profiles", "http-managed.toml")); !os.IsNotExist(err) {
			t.Fatalf("real user profile directory was touched, stat err=%v", err)
		}
	})
}

func writeAcceptanceProfile(t *testing.T, dir, name, description string) {
	t.Helper()
	if err := profiles.SaveProfileToDir(&profiles.Profile{
		Meta:   profiles.ProfileMeta{Name: name, Description: description, Version: 1, CreatedBy: "test"},
		Runner: profiles.ProfileRunner{Model: "fake-model", MaxSteps: 2},
	}, dir); err != nil {
		t.Fatalf("write %s profile to %s: %v", name, dir, err)
	}
}

func runHarnessdProfileAcceptance(t *testing.T, env map[string]string, provider harness.Provider, check func(baseURL string)) {
	t.Helper()
	sig := make(chan os.Signal, 1)
	done := make(chan error, 1)
	listenerAddr := make(chan string, 1)
	deps := runDeps{listen: func(network, address string) (net.Listener, error) {
		listener, err := net.Listen(network, address)
		if err == nil {
			listenerAddr <- listener.Addr().String()
		}
		return listener, err
	}}
	getenv := func(key string) string { return env[key] }
	go func() {
		done <- runWithSignalsWithDeps(sig, getenv, func(openai.Config) (harness.Provider, error) { return provider, nil }, "", deps)
	}()

	var addr string
	select {
	case addr = <-listenerAddr:
	case err := <-done:
		t.Fatalf("harnessd returned before listener: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for harnessd listener")
	}
	awaitHealthyOrRunFailure(t, addr, done, 10*time.Second)
	check("http://" + addr)
	sig <- os.Interrupt
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("harnessd shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for harnessd shutdown")
	}
}

func assertProfileToolsExposed(t *testing.T, baseURL string) {
	t.Helper()
	response, err := http.Get(baseURL + "/v1/tools")
	if err != nil {
		t.Fatalf("GET /v1/tools: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("GET /v1/tools = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode /v1/tools: %v", err)
	}
	found := map[string]bool{}
	for _, tool := range payload.Tools {
		found[tool.Name] = true
	}
	for _, name := range []string{"create_profile", "update_profile", "delete_profile"} {
		if !found[name] {
			t.Fatalf("configured profile mutation tool %q missing from %#v", name, payload.Tools)
		}
	}
}

func profileRequest(t *testing.T, baseURL, method, name, body string, want int) {
	t.Helper()
	request, err := http.NewRequest(method, baseURL+"/v1/profiles/"+name, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new %s profile request: %v", method, err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s profile %s: %v", method, name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("%s profile %s = %d, want %d: %s", method, name, response.StatusCode, want, payload)
	}
}

func assertProfileResponse(t *testing.T, baseURL, name, wantDescription string, wantStatus int) {
	t.Helper()
	response, err := http.Get(baseURL + "/v1/profiles/" + name)
	if err != nil {
		t.Fatalf("GET profile %s: %v", name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("GET profile %s = %d, want %d: %s", name, response.StatusCode, wantStatus, payload)
	}
	if wantStatus != http.StatusOK {
		return
	}
	var profile struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile %s: %v", name, err)
	}
	if profile.Description != wantDescription {
		t.Fatalf("profile %s description = %q, want %q", name, profile.Description, wantDescription)
	}
}

func startProfileAcceptanceRun(t *testing.T, baseURL, prompt string) string {
	t.Helper()
	response, err := http.Post(baseURL+"/v1/runs", "application/json", bytes.NewBufferString(`{"prompt":`+mustJSON(t, prompt)+`}`))
	if err != nil {
		t.Fatalf("POST run %q: %v", prompt, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("POST run %q = %d: %s", prompt, response.StatusCode, payload)
	}
	var payload struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil || payload.RunID == "" {
		t.Fatalf("decode started profile run: id=%q err=%v", payload.RunID, err)
	}
	return payload.RunID
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
