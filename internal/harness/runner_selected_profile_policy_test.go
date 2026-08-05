package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

func writeSelectedProfile(t *testing.T, dir string) {
	t.Helper()
	const profile = `
[meta]
name = "restricted"

[runner]
model = "profile-model"
max_steps = 2
system_prompt = "PROFILE_SYSTEM_PROMPT"
reasoning_effort = "low"

[tools]
allow = ["read"]

[permissions]
allow_bash = false
allow_file_write = false
allow_net_access = false

isolation_mode = "none"
`
	if err := os.WriteFile(filepath.Join(dir, "restricted.toml"), []byte(profile), 0o600); err != nil {
		t.Fatalf("write selected profile: %v", err)
	}
}

// TestStartRun_SelectedProfileAppliesOrdinaryPolicy is the #1188 red-first
// contract. The selected TUI/API profile must compose into an ordinary run,
// rather than only configuring isolation and MCP during preflight.
func TestStartRun_SelectedProfileAppliesOrdinaryPolicy(t *testing.T) {
	t.Parallel()
	profilesDir := t.TempDir()
	writeSelectedProfile(t, profilesDir)
	runner := NewRunner(&fakeProvider{}, NewRegistry(), RunnerConfig{
		DefaultModel: "server-default",
		ProfilesDir:  profilesDir,
	})

	run, err := runner.StartRun(RunRequest{Prompt: "first turn", ProfileName: "restricted"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.Model != "profile-model" {
		t.Fatalf("run.Model = %q, want selected profile model", run.Model)
	}

	runner.mu.RLock()
	state := runner.runs[run.ID]
	runner.mu.RUnlock()
	if state == nil {
		t.Fatal("selected run state missing")
	}
	if state.staticSystemPrompt != "PROFILE_SYSTEM_PROMPT" {
		t.Errorf("system prompt = %q, want profile prompt", state.staticSystemPrompt)
	}
	if state.config.MaxSteps != 0 { // request-level cap is checked below from state execution config
		t.Logf("runner config max steps remains server-scoped: %d", state.config.MaxSteps)
	}
	if !runner.toolAllowedForRun(run.ID, "read") {
		t.Error("profile-permitted read must remain available")
	}
	if runner.toolAllowedForRun(run.ID, "bash") {
		t.Error("profile tool policy must block bash")
	}
	definitions := runner.filteredToolsForRun(run.ID)
	for _, definition := range definitions {
		if definition.Name == "bash" {
			t.Fatal("blocked bash must not be offered to the provider")
		}
	}
}

// TestStartRun_SelectedProfileExplicitRequestCannotBroadenSafety proves the
// documented precedence: explicit non-safety values win, but a request cannot
// widen a selected profile's tool restriction.
func TestStartRun_SelectedProfileExplicitRequestCannotBroadenSafety(t *testing.T) {
	t.Parallel()
	profilesDir := t.TempDir()
	writeSelectedProfile(t, profilesDir)
	runner := NewRunner(&fakeProvider{}, NewRegistry(), RunnerConfig{ProfilesDir: profilesDir})

	run, err := runner.StartRun(RunRequest{
		Prompt:       "second turn",
		ProfileName:  "restricted",
		Model:        "explicit-model",
		AllowedTools: []string{"read", "bash"},
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.Model != "explicit-model" {
		t.Fatalf("explicit model = %q, want explicit-model", run.Model)
	}
	if runner.toolAllowedForRun(run.ID, "bash") {
		t.Error("explicit allowed_tools must not broaden selected profile")
	}
}

// TestStartRun_SelectedProfileNoneIsolationPreservesNoProvisioning locks the
// profile schema's "none" spelling to the runner's empty workspace type.
// Passing it directly to StartRun would incorrectly reject a documented
// non-isolated profile as an unsupported backend.
func TestStartRun_SelectedProfileNoneIsolationPreservesNoProvisioning(t *testing.T) {
	t.Parallel()
	profilesDir := t.TempDir()
	writeSelectedProfile(t, profilesDir)
	runner := NewRunner(&fakeProvider{}, NewRegistry(), RunnerConfig{ProfilesDir: profilesDir})
	if _, err := runner.StartRun(RunRequest{Prompt: "no provision", ProfileName: "restricted"}); err != nil {
		t.Fatalf("StartRun with isolation_mode none: %v", err)
	}
}

// TestContinueRun_SelectedProfileDeniedDownloadBlocksNetworkAndWrite proves
// that selected-profile capability policy survives a continuation. The first
// turn is deliberately harmless; its continuation directly calls all denied
// capabilities and must be blocked before HTTP or file I/O.
func TestContinueRun_SelectedProfileDeniedDownloadBlocksNetworkAndWrite(t *testing.T) {
	profilesDir := t.TempDir()
	const profile = `
[meta]
name = "download-denied"
[tools]
allow = ["fetch", "write", "download"]
[permissions]
allow_file_write = false
allow_net_access = false
`
	if err := os.WriteFile(filepath.Join(profilesDir, "download-denied.toml"), []byte(profile), 0o600); err != nil {
		t.Fatalf("write download profile: %v", err)
	}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	workspace := t.TempDir()
	registry := NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{NetworkAllowlist: []string{parsed.Hostname()}})
	if err := registry.ReplaceByTag("dynamic-profile-test", []htools.Tool{{
		Definition: htools.Definition{
			Name: "fetch", Description: "test outbound fetch", Parameters: map[string]any{"type": "object"}, ParallelSafe: true, Action: htools.ActionFetch,
		},
		Handler: func(ctx context.Context, _ json.RawMessage) (string, error) {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			if err != nil {
				return "", err
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				return "", err
			}
			defer response.Body.Close()
			return "fetched", nil
		},
	}}); err != nil {
		t.Fatalf("replace action-fetch probe: %v", err)
	}
	provider := &stubProvider{turns: []CompletionResult{
		{Content: "first turn completed"},
		{ToolCalls: []ToolCall{{ID: "fetch-denied", Name: "fetch", Arguments: `{"url":"` + server.URL + `"}`}}},
		{ToolCalls: []ToolCall{{ID: "write-denied", Name: "write", Arguments: `{"path":"written.txt","content":"must not write"}`}}},
		{ToolCalls: []ToolCall{{ID: "download-denied", Name: "download", Arguments: `{"url":"` + server.URL + `","file_path":"downloaded.txt"}`}}},
		{Content: "capabilities correctly denied"},
	}}
	runner := NewRunner(provider, registry, RunnerConfig{ProfilesDir: profilesDir, MaxSteps: 4})
	run, err := runner.StartRun(RunRequest{Prompt: "first turn", ProfileName: "download-denied"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatalf("collect first-turn events: %v", err)
	}
	continued, err := runner.ContinueRun(run.ID, "attempt denied download")
	if err != nil {
		t.Fatalf("ContinueRun: %v", err)
	}
	if continued.ConversationID != run.ConversationID {
		t.Fatalf("continued conversation = %q, want %q", continued.ConversationID, run.ConversationID)
	}
	runner.mu.RLock()
	continuedState := runner.runs[continued.ID]
	runner.mu.RUnlock()
	if continuedState == nil || continuedState.profileName != "download-denied" {
		t.Fatalf("continued profile = %q, want download-denied", continuedState.profileName)
	}
	events, err := collectRunEvents(t, runner, continued.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0 after profile denial", got)
	}
	if _, err := os.Stat(filepath.Join(workspace, "downloaded.txt")); !os.IsNotExist(err) {
		t.Fatalf("downloaded file exists or stat failed after profile denial: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "written.txt")); !os.IsNotExist(err) {
		t.Fatalf("written file exists or stat failed after profile denial: %v", err)
	}
	blocked := map[string]bool{}
	for _, event := range events {
		if event.Type == EventToolCallBlocked && event.Payload["reason"] == "profile_capability_denied" {
			if tool, _ := event.Payload["tool"].(string); tool != "" {
				blocked[tool] = true
			}
		}
	}
	for _, tool := range []string{"fetch", "write", "download"} {
		if !blocked[tool] {
			t.Fatalf("expected %s profile capability denial event, got %v", tool, blocked)
		}
	}
}

// TestContinueRun_SelectedProfileAllowlistCannotBeBroadened proves a
// continuation override may narrow a selected profile's tool list but cannot
// replace it. The source turn is harmless; the continuation directly requests
// bash and must be rejected before the handler can perform its side effect.
func TestContinueRun_SelectedProfileAllowlistCannotBeBroadened(t *testing.T) {
	profilesDir := t.TempDir()
	const profile = `
[meta]
name = "read-only"
[tools]
allow = ["read"]
`
	if err := os.WriteFile(filepath.Join(profilesDir, "read-only.toml"), []byte(profile), 0o600); err != nil {
		t.Fatalf("write read-only profile: %v", err)
	}
	var bashCalls atomic.Int64
	registry := NewRegistry()
	if err := registry.RegisterWithOptions(ToolDefinition{Name: "bash", Description: "side-effect probe", Parameters: map[string]any{"type": "object"}}, func(context.Context, json.RawMessage) (string, error) {
		bashCalls.Add(1)
		return "must not execute", nil
	}, RegisterOptions{Action: htools.ActionExecute}); err != nil {
		t.Fatalf("register bash probe: %v", err)
	}
	provider := &stubProvider{turns: []CompletionResult{
		{Content: "source completed"},
		{ToolCalls: []ToolCall{{ID: "bash-bypass", Name: "bash", Arguments: `{"command":"echo side-effect"}`}}},
		{Content: "continuation completed"},
	}}
	runner := NewRunner(provider, registry, RunnerConfig{ProfilesDir: profilesDir, MaxSteps: 2})
	source, err := runner.StartRun(RunRequest{Prompt: "first turn", ProfileName: "read-only"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if _, err := collectRunEvents(t, runner, source.ID); err != nil {
		t.Fatalf("collect source events: %v", err)
	}
	runner.mu.RLock()
	sourceState := runner.runs[source.ID]
	runner.mu.RUnlock()
	if sourceState == nil || len(sourceState.profileAllowedTools) != 1 || sourceState.profileAllowedTools[0] != "read" {
		t.Fatalf("source profile tool bound = %v, want [read]", sourceState.profileAllowedTools)
	}
	widen := []string{"bash"}
	continued, err := runner.ContinueRunWithOptions(source.ID, ContinueRunRequest{Prompt: "try bash", AllowedTools: &widen})
	if err != nil {
		t.Fatalf("ContinueRunWithOptions: %v", err)
	}
	events, err := collectRunEvents(t, runner, continued.ID)
	if err != nil {
		t.Fatalf("collect continuation events: %v", err)
	}
	if got := bashCalls.Load(); got != 0 {
		t.Fatalf("bash calls = %d, want 0 after selected profile allowlist denial", got)
	}
	for _, event := range events {
		if event.Type == EventToolCallBlocked && event.Payload["tool"] == "bash" {
			return
		}
	}
	t.Fatal("expected continuation bash call to be blocked")
}

// TestStartRun_SelectedProfileEmptyIntersectionDeniesAll locks the selected
// profile boundary at admission: both a disjoint request and an explicit empty
// request must deny ordinary tools rather than inheriting the legacy empty =
// unrestricted interpretation.
func TestStartRun_SelectedProfileEmptyIntersectionDeniesAll(t *testing.T) {
	for _, requested := range [][]string{{"bash"}, {}} {
		t.Run("requested="+strings.Join(requested, ","), func(t *testing.T) {
			profilesDir := t.TempDir()
			const profile = "[meta]\nname = \"read-only\"\n[tools]\nallow = [\"read\"]\n"
			if err := os.WriteFile(filepath.Join(profilesDir, "read-only.toml"), []byte(profile), 0o600); err != nil {
				t.Fatalf("write profile: %v", err)
			}
			var calls atomic.Int64
			registry := NewRegistry()
			if err := registry.RegisterWithOptions(ToolDefinition{Name: "read", Description: "probe", Parameters: map[string]any{"type": "object"}}, func(context.Context, json.RawMessage) (string, error) {
				calls.Add(1)
				return "must not execute", nil
			}, RegisterOptions{Action: htools.ActionExecute}); err != nil {
				t.Fatalf("register read: %v", err)
			}
			provider := &stubProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "read", Name: "read", Arguments: `{}`}}}, {Content: "blocked"}}}
			runner := NewRunner(provider, registry, RunnerConfig{ProfilesDir: profilesDir, MaxSteps: 2})
			run, err := runner.StartRun(RunRequest{Prompt: "try bash", ProfileName: "read-only", AllowedTools: requested})
			if err != nil {
				t.Fatalf("StartRun: %v", err)
			}
			events, err := collectRunEvents(t, runner, run.ID)
			if err != nil {
				t.Fatalf("collect events: %v", err)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("read calls = %d, want 0", got)
			}
			for _, event := range events {
				if event.Type == EventToolCallBlocked && event.Payload["tool"] == "read" {
					return
				}
			}
			t.Fatal("expected read call to be blocked")
		})
	}
}

func TestSelectedProfileSkillConstraintCannotBroadenToolBound(t *testing.T) {
	profilesDir := t.TempDir()
	const profile = "[meta]\nname = \"skill-read\"\n[tools]\nallow = [\"skill\", \"read\"]\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "skill-read.toml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(&fakeProvider{}, NewRegistry(), RunnerConfig{ProfilesDir: profilesDir})
	run, err := runner.StartRun(RunRequest{Prompt: "start", ProfileName: "skill-read"})
	if err != nil {
		t.Fatal(err)
	}
	runner.skillConstraints.Activate(run.ID, SkillConstraint{SkillName: "attempted-broaden", AllowedTools: []string{"bash"}})
	if runner.toolAllowedForRun(run.ID, "bash") {
		t.Fatal("skill constraint must not broaden selected profile to bash")
	}
}

// TestStartRun_SelectedProfileRecipeCannotBypassToolBound ensures run_recipe
// cannot turn permission to invoke its outer wrapper into permission to invoke
// arbitrary pre-wired recipe handlers. The fixture includes execution, write,
// and network steps; no inner side effect may occur when a selected profile
// permits only run_recipe.
func TestStartRun_SelectedProfileRecipeCannotBypassToolBound(t *testing.T) {
	profilesDir := t.TempDir()
	const profile = "[meta]\nname = \"recipes-only\"\n[tools]\nallow = [\"run_recipe\"]\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "recipes-only.toml"), []byte(profile), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	workspace := t.TempDir()
	recipesDir := t.TempDir()
	bashPath := filepath.Join(workspace, "bash-pwned")
	writePath := filepath.Join(workspace, "write-pwned")
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	recipeYAML := fmt.Sprintf(`
name: forbidden-indirection
description: "attempt profile-boundary escape"
steps:
  - name: shell
    tool: bash
    args:
      command: "touch '%s'"
  - name: write
    tool: write
    args:
      path: "%s"
      content: "must not write"
  - name: fetch
    tool: fetch
    args:
      url: "%s"
`, bashPath, writePath, server.URL)
	if err := os.WriteFile(filepath.Join(recipesDir, "forbidden.yaml"), []byte(recipeYAML), 0o600); err != nil {
		t.Fatalf("write recipe: %v", err)
	}
	registry := NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{RecipesDir: recipesDir})
	provider := &stubProvider{turns: []CompletionResult{
		{ToolCalls: []ToolCall{{ID: "recipe-bypass", Name: "run_recipe", Arguments: `{"name":"forbidden-indirection"}`}}},
		{Content: "recipe rejected"},
	}}
	runner := NewRunner(provider, registry, RunnerConfig{ProfilesDir: profilesDir, MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "attempt recipe", ProfileName: "recipes-only"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0", got)
	}
	for _, path := range []string{bashPath, writePath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("forbidden recipe side-effect at %s: %v", path, err)
		}
	}
	for _, event := range events {
		if event.Type == EventToolCallCompleted && event.Payload["tool"] == "run_recipe" && strings.Contains(fmt.Sprint(event.Payload["error"]), "bash") {
			return
		}
	}
	t.Fatal("expected run_recipe to reject its disallowed bash member before handlers execute")
}

// TestStartRun_RecipeMemberHonorsActiveSkillConstraint proves the recipe
// wrapper cannot bypass a more restrictive active skill. The profile permits
// both outer and inner tools so the observed denial is specifically the skill
// member gate rather than selected-profile filtering.
func TestStartRun_RecipeMemberHonorsActiveSkillConstraint(t *testing.T) {
	profilesDir := t.TempDir()
	const profile = "[meta]\nname = \"recipe-skill\"\n[tools]\nallow = [\"run_recipe\", \"bash\"]\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "recipe-skill.toml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace, recipesDir := t.TempDir(), t.TempDir()
	pwned := filepath.Join(workspace, "skill-pwned")
	if err := os.WriteFile(filepath.Join(recipesDir, "skill.yaml"), []byte(fmt.Sprintf("name: skill-pwn\ndescription: skill member gate\nsteps:\n  - name: bash\n    tool: bash\n    args:\n      command: \"touch '%s'\"\n", pwned)), 0o600); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	provider := &selectedProfileGateProvider{gate: gate, turns: []CompletionResult{
		{ToolCalls: []ToolCall{{ID: "recipe-skill", Name: "run_recipe", Arguments: `{"name":"skill-pwn"}`}}},
		{Content: "done"},
	}}
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{RecipesDir: recipesDir}), RunnerConfig{ProfilesDir: profilesDir, MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "recipe", ProfileName: "recipe-skill"})
	if err != nil {
		t.Fatal(err)
	}
	runner.skillConstraints.Activate(run.ID, SkillConstraint{SkillName: "outer-only", AllowedTools: []string{"run_recipe"}})
	close(gate)
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pwned); !os.IsNotExist(err) {
		t.Fatalf("skill-constrained recipe created %s: %v", pwned, err)
	}
	for _, event := range events {
		if event.Type == EventToolCallCompleted && event.Payload["tool"] == "run_recipe" && strings.Contains(fmt.Sprint(event.Payload["error"]), "active skill") {
			return
		}
	}
	t.Fatal("expected active skill constraint to reject recipe bash member")
}

// TestStartRun_RecipeMemberHonorsPermissionRules verifies both terminal
// permission-rule outcomes at the member boundary: deny prevents the real
// bash side effect, while ask exposes the bash member to the approval broker
// and runs it only after the operator approves.
func TestStartRun_RecipeMemberHonorsPermissionRules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		effect   PermissionEffect
		approve  bool
		wantFile bool
	}{
		{name: "deny", effect: PermissionEffectDeny},
		{name: "ask approved", effect: PermissionEffectAsk, approve: true, wantFile: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profilesDir := t.TempDir()
			const profile = "[meta]\nname = \"recipe-rules\"\n[tools]\nallow = [\"run_recipe\", \"bash\"]\n"
			if err := os.WriteFile(filepath.Join(profilesDir, "recipe-rules.toml"), []byte(profile), 0o600); err != nil {
				t.Fatal(err)
			}
			workspace, recipesDir := t.TempDir(), t.TempDir()
			output := filepath.Join(workspace, "rule-pwned")
			if err := os.WriteFile(filepath.Join(recipesDir, "rule.yaml"), []byte(fmt.Sprintf("name: rule-pwn\ndescription: permission member gate\nsteps:\n  - name: bash\n    tool: bash\n    args:\n      command: \"touch '%s'\"\n", output)), 0o600); err != nil {
				t.Fatal(err)
			}
			broker := NewInMemoryApprovalBroker()
			provider := &stubProvider{turns: []CompletionResult{
				{ToolCalls: []ToolCall{{ID: "recipe-rule", Name: "run_recipe", Arguments: `{"name":"rule-pwn"}`}}},
				{Content: "done"},
			}}
			runner := NewRunner(provider, NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{RecipesDir: recipesDir}), RunnerConfig{ProfilesDir: profilesDir, ApprovalBroker: broker, MaxSteps: 2})
			run, err := runner.StartRun(RunRequest{Prompt: "recipe", ProfileName: "recipe-rules", Permissions: &PermissionConfig{Rules: permissionRuleSet(permissionRule("bash", tc.effect))}})
			if err != nil {
				t.Fatal(err)
			}
			if tc.approve {
				deadline := time.Now().Add(5 * time.Second)
				for {
					pending, ok := broker.Pending(run.ID)
					if ok {
						if pending.Tool != "bash" || !strings.Contains(pending.CallID, ":recipe:0:bash") {
							t.Fatalf("approval targeted %+v, want recipe bash member", pending)
						}
						if err := broker.Approve(run.ID); err != nil {
							t.Fatal(err)
						}
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("timed out waiting for recipe member approval")
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			if _, err := collectRunEvents(t, runner, run.ID); err != nil {
				t.Fatal(err)
			}
			_, statErr := os.Stat(output)
			if tc.wantFile && statErr != nil {
				t.Fatalf("approved recipe member did not create %s: %v", output, statErr)
			}
			if !tc.wantFile && !os.IsNotExist(statErr) {
				t.Fatalf("denied recipe member created %s: %v", output, statErr)
			}
		})
	}
}

// TestStartRun_RecipeMemberHonorsPreToolUseHooks keeps recipe members on the
// same hook path as direct calls: a deny has no side effect and a mutation is
// the argument actually passed to the bash handler.
func TestStartRun_RecipeMemberHonorsPreToolUseHooks(t *testing.T) {
	for _, tc := range []struct {
		name       string
		deny       bool
		hookError  bool
		wantMutate bool
	}{
		{name: "deny", deny: true},
		{name: "hook error", hookError: true},
		{name: "mutation", wantMutate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profilesDir, workspace, recipesDir := t.TempDir(), t.TempDir(), t.TempDir()
			const profile = "[meta]\nname = \"recipe-hooks\"\n[tools]\nallow = [\"run_recipe\", \"bash\"]\n"
			if err := os.WriteFile(filepath.Join(profilesDir, "recipe-hooks.toml"), []byte(profile), 0o600); err != nil {
				t.Fatal(err)
			}
			original, modified := filepath.Join(workspace, "original"), filepath.Join(workspace, "modified")
			recipeYAML := fmt.Sprintf("name: hook-pwn\ndescription: hook parity\nsteps:\n  - name: bash\n    tool: bash\n    args:\n      command: \"touch '%s'\"\n", original)
			if err := os.WriteFile(filepath.Join(recipesDir, "hook.yaml"), []byte(recipeYAML), 0o600); err != nil {
				t.Fatal(err)
			}
			var bashHookCalls atomic.Int64
			hook := preToolHookFunc{name: "recipe-member-hook", fn: func(_ context.Context, ev PreToolUseEvent) (*PreToolUseResult, error) {
				if ev.ToolName != "bash" {
					return &PreToolUseResult{Decision: ToolHookAllow}, nil
				}
				bashHookCalls.Add(1)
				if tc.deny {
					return &PreToolUseResult{Decision: ToolHookDeny, Reason: "deny recipe bash"}, nil
				}
				if tc.hookError {
					return nil, fmt.Errorf("recipe hook error")
				}
				return &PreToolUseResult{Decision: ToolHookAllow, ModifiedArgs: json.RawMessage(fmt.Sprintf(`{"command":"touch '%s'"}`, modified))}, nil
			}}
			provider := &stubProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "recipe-hook", Name: "run_recipe", Arguments: `{"name":"hook-pwn"}`}}}, {Content: "done"}}}
			runner := NewRunner(provider, NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{RecipesDir: recipesDir}), RunnerConfig{ProfilesDir: profilesDir, PreToolUseHooks: []PreToolUseHook{hook}, MaxSteps: 2})
			run, err := runner.StartRun(RunRequest{Prompt: "recipe", ProfileName: "recipe-hooks"})
			if err != nil {
				t.Fatal(err)
			}
			events, err := collectRunEvents(t, runner, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got := bashHookCalls.Load(); got != 1 {
				t.Fatalf("bash hook calls = %d, want 1", got)
			}
			if _, err := os.Stat(original); !os.IsNotExist(err) {
				t.Fatalf("original recipe command executed despite hook policy: %v", err)
			}
			_, modifiedErr := os.Stat(modified)
			if tc.wantMutate && modifiedErr != nil {
				t.Fatalf("hook-modified recipe command did not execute: %v", modifiedErr)
			}
			if !tc.wantMutate && !os.IsNotExist(modifiedErr) {
				t.Fatalf("denied recipe command unexpectedly created modified output: %v", modifiedErr)
			}
			if tc.hookError {
				for _, event := range events {
					if event.Type == EventToolHookFailed && event.Payload["tool"] == "bash" {
						return
					}
				}
				t.Fatal("expected fail-closed recipe hook error event")
			}
		})
	}
}

// TestStartRun_BlockedRecipeMemberSkipsPreToolUseHooks preserves direct-call
// ordering: profile/allowlist/skill rejection happens before a hook observes
// or can mutate the blocked member.
func TestStartRun_BlockedRecipeMemberSkipsPreToolUseHooks(t *testing.T) {
	profilesDir, workspace, recipesDir := t.TempDir(), t.TempDir(), t.TempDir()
	const profile = "[meta]\nname = \"recipe-outer-only\"\n[tools]\nallow = [\"run_recipe\"]\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "recipe-outer-only.toml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	pwned := filepath.Join(workspace, "blocked-member")
	if err := os.WriteFile(filepath.Join(recipesDir, "blocked.yaml"), []byte(fmt.Sprintf("name: blocked-member\ndescription: blocked hook order\nsteps:\n  - name: bash\n    tool: bash\n    args:\n      command: \"touch '%s'\"\n", pwned)), 0o600); err != nil {
		t.Fatal(err)
	}
	var hookCalls atomic.Int64
	hook := preToolHookFunc{name: "must-not-observe", fn: func(_ context.Context, ev PreToolUseEvent) (*PreToolUseResult, error) {
		if ev.ToolName == "bash" {
			hookCalls.Add(1)
		}
		return &PreToolUseResult{Decision: ToolHookAllow}, nil
	}}
	provider := &stubProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "recipe-blocked", Name: "run_recipe", Arguments: `{"name":"blocked-member"}`}}}, {Content: "done"}}}
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{RecipesDir: recipesDir}), RunnerConfig{ProfilesDir: profilesDir, PreToolUseHooks: []PreToolUseHook{hook}, MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "recipe", ProfileName: "recipe-outer-only"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatal(err)
	}
	if got := hookCalls.Load(); got != 0 {
		t.Fatalf("blocked bash hook calls = %d, want 0", got)
	}
	if _, err := os.Stat(pwned); !os.IsNotExist(err) {
		t.Fatalf("blocked recipe member created %s: %v", pwned, err)
	}
}

// TestStartRun_RecipeMemberApprovalIDsAreUnique makes duplicate or optional
// step names safe for the single-pending-entry approval broker.
func TestStartRun_RecipeMemberApprovalIDsAreUnique(t *testing.T) {
	profilesDir, workspace, recipesDir := t.TempDir(), t.TempDir(), t.TempDir()
	const profile = "[meta]\nname = \"recipe-ids\"\n[tools]\nallow = [\"run_recipe\", \"bash\"]\n"
	if err := os.WriteFile(filepath.Join(profilesDir, "recipe-ids.toml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	first, second := filepath.Join(workspace, "first"), filepath.Join(workspace, "second")
	recipeYAML := fmt.Sprintf("name: duplicate-names\ndescription: unique approvals\nsteps:\n  - tool: bash\n    args:\n      command: \"touch '%s'\"\n  - tool: bash\n    args:\n      command: \"touch '%s'\"\n", first, second)
	if err := os.WriteFile(filepath.Join(recipesDir, "ids.yaml"), []byte(recipeYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	broker := NewInMemoryApprovalBroker()
	provider := &stubProvider{turns: []CompletionResult{{ToolCalls: []ToolCall{{ID: "recipe-ids", Name: "run_recipe", Arguments: `{"name":"duplicate-names"}`}}}, {Content: "done"}}}
	runner := NewRunner(provider, NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{RecipesDir: recipesDir}), RunnerConfig{ProfilesDir: profilesDir, ApprovalBroker: broker, MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "recipe", ProfileName: "recipe-ids", Permissions: &PermissionConfig{Rules: permissionRuleSet(permissionRule("bash", PermissionEffectAsk))}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for len(seen) < 2 {
		deadline := time.Now().Add(5 * time.Second)
		for {
			pending, ok := broker.Pending(run.ID)
			if ok {
				seen[pending.CallID] = true
				if err := broker.Approve(run.ID); err != nil {
					t.Fatal(err)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for approval %d", len(seen)+1)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if _, err := collectRunEvents(t, runner, run.ID); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || !seen["recipe-ids:recipe:0:"] || !seen["recipe-ids:recipe:1:"] {
		t.Fatalf("member approval IDs = %v, want indexed unique IDs", seen)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("approved recipe step did not create %s: %v", path, err)
		}
	}
}

type selectedProfileGateProvider struct {
	gate  <-chan struct{}
	turns []CompletionResult
	calls atomic.Int64
}

func (p *selectedProfileGateProvider) Complete(context.Context, CompletionRequest) (CompletionResult, error) {
	<-p.gate
	idx := int(p.calls.Add(1) - 1)
	if idx >= len(p.turns) {
		return CompletionResult{Content: "done"}, nil
	}
	return p.turns[idx], nil
}

type profileMCPNetworkProbe struct{ calls atomic.Int64 }

func (p *profileMCPNetworkProbe) ListResources(context.Context, string) ([]htools.MCPResource, error) {
	return nil, nil
}
func (p *profileMCPNetworkProbe) ReadResource(context.Context, string, string) (string, error) {
	return "", nil
}
func (p *profileMCPNetworkProbe) ListTools(context.Context) (map[string][]htools.MCPToolDefinition, error) {
	return nil, nil
}
func (p *profileMCPNetworkProbe) CallTool(context.Context, string, string, json.RawMessage) (string, error) {
	p.calls.Add(1)
	return "external call", nil
}

type profileMCPConnectProbe struct{ calls atomic.Int64 }

func (p *profileMCPConnectProbe) Connect(context.Context, string, string) (htools.MCPRegistry, error) {
	p.calls.Add(1)
	return nil, nil
}

// TestStartRun_SelectedProfileDeniedNetworkBlocksMCPCall keeps the external
// RPC boundary fail-closed: an MCP tool is an outbound capability even though
// the server did not declare an internal harness action category.
func TestStartRun_SelectedProfileDeniedNetworkBlocksMCPCall(t *testing.T) {
	profilesDir := t.TempDir()
	const profile = `
[meta]
name = "mcp-net-denied"
[tools]
allow = ["mcp_remote_probe"]
[permissions]
allow_net_access = false
`
	if err := os.WriteFile(filepath.Join(profilesDir, "mcp-net-denied.toml"), []byte(profile), 0o600); err != nil {
		t.Fatalf("write MCP profile: %v", err)
	}
	probe := &profileMCPNetworkProbe{}
	registry := NewRegistry()
	if _, err := registry.RegisterMCPTools("remote", []htools.MCPToolDefinition{{Name: "probe", Description: "external probe", Parameters: map[string]any{}}}, probe); err != nil {
		t.Fatalf("RegisterMCPTools: %v", err)
	}
	provider := &stubProvider{turns: []CompletionResult{
		{ToolCalls: []ToolCall{{ID: "mcp-denied", Name: "mcp_remote_probe", Arguments: `{}`}}},
		{Content: "MCP call blocked"},
	}}
	runner := NewRunner(provider, registry, RunnerConfig{ProfilesDir: profilesDir, MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "do not call external MCP", ProfileName: "mcp-net-denied"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	if got := probe.calls.Load(); got != 0 {
		t.Fatalf("MCP calls = %d, want 0 after profile network denial", got)
	}
	for _, event := range events {
		if event.Type == EventToolCallBlocked && event.Payload["tool"] == "mcp_remote_probe" && event.Payload["reason"] == "profile_capability_denied" {
			return
		}
	}
	t.Fatal("expected profile capability denial event for MCP tool")
}

// TestStartRun_SelectedProfileDeniedNetworkBlocksMCPConnect proves that the
// connection/setup tool is also an outbound boundary, before it can connect or
// enumerate an external MCP server.
func TestStartRun_SelectedProfileDeniedNetworkBlocksMCPConnect(t *testing.T) {
	profilesDir := t.TempDir()
	const profile = `
[meta]
name = "mcp-connect-net-denied"
[tools]
allow = ["connect_mcp"]
[permissions]
allow_net_access = false
`
	if err := os.WriteFile(filepath.Join(profilesDir, "mcp-connect-net-denied.toml"), []byte(profile), 0o600); err != nil {
		t.Fatalf("write MCP-connect profile: %v", err)
	}
	probe := &profileMCPConnectProbe{}
	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{MCPConnector: probe})
	provider := &stubProvider{turns: []CompletionResult{
		{ToolCalls: []ToolCall{{ID: "mcp-connect-denied", Name: "connect_mcp", Arguments: `{"url":"https://mcp.example.test/sse"}`}}},
		{Content: "MCP connection blocked"},
	}}
	runner := NewRunner(provider, registry, RunnerConfig{ProfilesDir: profilesDir, MaxSteps: 2})
	run, err := runner.StartRun(RunRequest{Prompt: "do not connect external MCP", ProfileName: "mcp-connect-net-denied"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	events, err := collectRunEvents(t, runner, run.ID)
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	if got := probe.calls.Load(); got != 0 {
		t.Fatalf("MCP connector calls = %d, want 0 after profile network denial", got)
	}
	for _, event := range events {
		if event.Type == EventToolCallBlocked && event.Payload["tool"] == "connect_mcp" && event.Payload["reason"] == "profile_capability_denied" {
			return
		}
	}
	t.Fatal("expected profile capability denial event for connect_mcp")
}
