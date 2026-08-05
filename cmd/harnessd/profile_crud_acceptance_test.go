package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// TestHarnessdSkillsDirOverrideCreatesVerifiesAndServesIsolatedSkill is the
// real fake-provider daemon acceptance for #1199. It drives an agent-created
// skill, its SSE stream, immediate catalog/GET/verify/activation lifecycle,
// restart reload, and same-conversation turns while proving neither the configured legacy
// global root nor the user's default global root receives the authored file.
func TestHarnessdSkillsDirOverrideCreatesVerifiesAndServesIsolatedSkill(t *testing.T) {
	workspace := t.TempDir()
	globalDir := t.TempDir()
	skillsDir := t.TempDir()
	const skillName = "isolated-agent-skill"
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	// Seed one ordinary skill so the startup catalog exposes the core `skill`
	// executor; the authored skill below is still created after startup.
	seedDir := filepath.Join(skillsDir, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "SKILL.md"), []byte("---\nname: seed\ndescription: \"seed skill Trigger: seed\"\nversion: 1\n---\nSeed instructions.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env := baseEnv("127.0.0.1:0")
	env["HARNESS_WORKSPACE"] = workspace
	env["HARNESS_GLOBAL_DIR"] = globalDir
	env["HARNESS_SKILLS_DIR"] = skillsDir
	env["HARNESS_SKILLS_ENABLED"] = "true"
	// Immediate lifecycle operations must not rely on the asynchronous watcher.
	env["HARNESS_WATCH_ENABLED"] = "false"
	disableCallbacksForUnrelatedHarnessFixture(env)

	provider := fakeprovider.New([]fakeprovider.Turn{
		{ToolCalls: []harness.ToolCall{{
			ID: "create-isolated-skill", Name: "create_skill",
			Arguments: `{"name":"isolated-agent-skill","description":"isolated agent acceptance","trigger":"when isolated acceptance is requested","content":"Respond with the isolated skill confirmation, explain why the result is durable, and include the requested acceptance details for the operator."}`,
		}}},
		{Content: "isolated skill created"},
		{ToolCalls: []harness.ToolCall{{ID: "verify-isolated-skill", Name: "verify_skill", Arguments: `{"name":"isolated-agent-skill"}`}}},
		{Content: "isolated skill verified"},
		{ToolCalls: []harness.ToolCall{{ID: "activate-isolated-skill", Name: "skill", Arguments: `{"command":"isolated-agent-skill"}`}}},
		{Content: "isolated skill activated"},
	})
	isolationFile := filepath.Join(skillsDir, skillName, "SKILL.md")

	runHarnessdProfileAcceptance(t, env, provider, func(baseURL string) {
		assertSkillToolsExposed(t, baseURL)
		firstRunID := startProfileAcceptanceRun(t, baseURL, "create the isolated skill")
		first := awaitRunTerminalState(t, baseURL, firstRunID, 5*time.Second)
		if first["status"] != string(harness.RunStatusCompleted) || first["output"] != "isolated skill created" {
			t.Fatalf("create run terminal state = %#v", first)
		}
		conversationID, _ := first["conversation_id"].(string)
		if conversationID == "" {
			t.Fatalf("create run missing conversation id: %#v", first)
		}

		isolatedFile := isolationFile
		if _, err := os.Stat(isolatedFile); err != nil {
			t.Fatalf("agent-created isolated skill missing at %s: %v", isolatedFile, err)
		}
		for _, forbidden := range []string{
			filepath.Join(globalDir, "skills", skillName, "SKILL.md"),
			filepath.Join(home, ".go-harness", "skills", skillName, "SKILL.md"),
		} {
			if _, err := os.Stat(forbidden); !os.IsNotExist(err) {
				t.Fatalf("isolated create touched forbidden global root %s: %v", forbidden, err)
			}
		}

		eventsResp, err := http.Get(baseURL + "/v1/runs/" + firstRunID + "/events")
		if err != nil {
			t.Fatalf("GET events: %v", err)
		}
		eventsBody, _ := io.ReadAll(eventsResp.Body)
		eventsResp.Body.Close()
		if !strings.Contains(string(eventsBody), "event: tool.call.completed") || !strings.Contains(string(eventsBody), "create_skill") {
			t.Fatalf("create_skill execution not visible through SSE: %s", eventsBody)
		}

		getResp, err := http.Get(baseURL + "/v1/skills/" + skillName)
		if err != nil {
			t.Fatalf("GET created skill: %v", err)
		}
		getBody, _ := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		if getResp.StatusCode != http.StatusOK || !strings.Contains(string(getBody), isolatedFile) {
			t.Fatalf("created skill catalog/GET mismatch: status=%d body=%s", getResp.StatusCode, getBody)
		}

		body, err := json.Marshal(map[string]string{"prompt": "verify the isolated skill", "conversation_id": conversationID})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(baseURL+"/v1/runs", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST continuation: %v", err)
		}
		var continuation struct {
			RunID string `json:"run_id"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&continuation)
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted || decodeErr != nil || continuation.RunID == "" {
			t.Fatalf("continuation start status=%d run=%q err=%v", resp.StatusCode, continuation.RunID, decodeErr)
		}
		second := awaitRunTerminalState(t, baseURL, continuation.RunID, 5*time.Second)
		if second["status"] != string(harness.RunStatusCompleted) || second["conversation_id"] != conversationID || second["output"] != "isolated skill verified" {
			t.Fatalf("immediate verify continuation failed: %#v", second)
		}
		file, err := os.ReadFile(isolatedFile)
		if err != nil || !strings.Contains(string(file), "verified: true") {
			t.Fatalf("verification was not durable: err=%v content=%s", err, file)
		}
		verifyGet, err := http.Get(baseURL + "/v1/skills/" + skillName)
		if err != nil {
			t.Fatal(err)
		}
		verifyGetBody, _ := io.ReadAll(verifyGet.Body)
		verifyGet.Body.Close()
		if verifyGet.StatusCode != http.StatusOK || !strings.Contains(string(verifyGetBody), `"verified":true`) {
			t.Fatalf("registry verification not visible: %d %s", verifyGet.StatusCode, verifyGetBody)
		}
		body, err = json.Marshal(map[string]string{"prompt": "activate the isolated skill", "conversation_id": conversationID})
		if err != nil {
			t.Fatal(err)
		}
		resp, err = http.Post(baseURL+"/v1/runs", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST activation continuation: %v", err)
		}
		var activation struct {
			RunID string `json:"run_id"`
		}
		decodeErr = json.NewDecoder(resp.Body).Decode(&activation)
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted || decodeErr != nil || activation.RunID == "" {
			t.Fatalf("activation continuation status=%d run=%q err=%v", resp.StatusCode, activation.RunID, decodeErr)
		}
		third := awaitRunTerminalState(t, baseURL, activation.RunID, 5*time.Second)
		if third["status"] != string(harness.RunStatusCompleted) || third["conversation_id"] != conversationID || third["output"] != "isolated skill activated" {
			t.Fatalf("same-conversation skill activation failed: %#v", third)
		}
		activationEvents, err := http.Get(baseURL + "/v1/runs/" + activation.RunID + "/events")
		if err != nil {
			t.Fatal(err)
		}
		activationBody, _ := io.ReadAll(activationEvents.Body)
		activationEvents.Body.Close()
		if !strings.Contains(string(activationBody), "event: tool.call.completed") || !strings.Contains(string(activationBody), "activate-isolated-skill") || !strings.Contains(string(activationBody), "event: meta.message.injected") {
			t.Fatalf("activation tool result/meta-message missing from SSE: %s", activationBody)
		}
	})

	// A new daemon must reload the persisted verification from the same isolated
	// directory, independent of the original in-memory registry and watcher.
	restartProvider := fakeprovider.New([]fakeprovider.Turn{
		{ToolCalls: []harness.ToolCall{{ID: "use-restarted-skill", Name: "skill", Arguments: `{"command":"isolated-agent-skill"}`}}},
		{Content: "restarted skill activated"},
	})
	runHarnessdProfileAcceptance(t, env, restartProvider, func(baseURL string) {
		response, err := http.Get(baseURL + "/v1/skills/" + skillName)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"verified":true`) {
			t.Fatalf("restart did not reload verified skill: %d %s", response.StatusCode, body)
		}
		if file, err := os.ReadFile(isolationFile); err != nil || !strings.Contains(string(file), "verified: true") {
			t.Fatalf("restart disk proof failed: err=%v content=%s", err, file)
		}
		runID := startProfileAcceptanceRun(t, baseURL, "use the restarted isolated skill")
		terminal := awaitRunTerminalState(t, baseURL, runID, 5*time.Second)
		if terminal["status"] != string(harness.RunStatusCompleted) || terminal["output"] != "restarted skill activated" {
			t.Fatalf("restart skill use failed: %#v", terminal)
		}
	})
}

func assertSkillToolsExposed(t *testing.T, baseURL string) {
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
	for _, name := range []string{"create_skill", "verify_skill"} {
		if !found[name] {
			t.Fatalf("configured skill tool %q missing from catalog %#v", name, payload.Tools)
		}
	}
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
