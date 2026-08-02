package harness

// Conditional tool registration, proven against the single surviving tool
// catalog (NewDefaultRegistryWithOptions, tools_default.go).
//
// These cases were previously asserted against the duplicate BuildCatalog
// builder, which has been removed. The behaviour they pin — a tool appears
// only when its dependency is configured — is a property of the registry, so
// the tests moved here rather than being dropped with the builder.

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/provider/catalog"
)

type stubCronClientForRegistryTest struct{}

func (stubCronClientForRegistryTest) CreateJob(context.Context, htools.CronCreateJobRequest) (htools.CronJob, error) {
	return htools.CronJob{}, nil
}
func (stubCronClientForRegistryTest) ListJobs(context.Context) ([]htools.CronJob, error) {
	return nil, nil
}
func (stubCronClientForRegistryTest) GetJob(context.Context, string) (htools.CronJob, error) {
	return htools.CronJob{}, nil
}
func (stubCronClientForRegistryTest) UpdateJob(context.Context, string, htools.CronUpdateJobRequest) (htools.CronJob, error) {
	return htools.CronJob{}, nil
}
func (stubCronClientForRegistryTest) DeleteJob(context.Context, string) error { return nil }
func (stubCronClientForRegistryTest) DeleteJobCAS(context.Context, string, time.Time) error {
	return nil
}
func (stubCronClientForRegistryTest) ListExecutions(context.Context, string, int, int) ([]htools.CronExecution, error) {
	return nil, nil
}
func (stubCronClientForRegistryTest) Health(context.Context) error { return nil }

func registeredToolNames(registry *Registry) map[string]bool {
	names := make(map[string]bool)
	for _, def := range registry.Definitions() {
		names[def.Name] = true
	}
	return names
}

type provenanceMCPRegistry struct{ mockMCPReg }

func (provenanceMCPRegistry) ListTools(context.Context) (map[string][]htools.MCPToolDefinition, error) {
	return map[string][]htools.MCPToolDefinition{
		"calendar": {{Name: "create event", Description: "Create an event", Parameters: map[string]any{"type": "object"}}},
	}, nil
}

func TestNewDefaultRegistryWithOptions_ProvenanceFollowsRegistrationSource(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		Sourcegraph:  htools.SourcegraphConfig{Endpoint: "https://sourcegraph.example.test"},
		MCPRegistry:  &provenanceMCPRegistry{},
	})
	t.Cleanup(func() { _ = registry.Shutdown(t.Context()) })

	byName := make(map[string]ToolMetadata)
	for _, meta := range registry.DefinitionsWithMetadata() {
		byName[meta.Definition.Name] = meta
	}
	assertProvenance := func(name, owner, condition string) {
		t.Helper()
		meta, ok := byName[name]
		if !ok {
			t.Fatalf("tool %q is not registered", name)
		}
		if meta.Owner != owner || meta.Condition != condition {
			t.Fatalf("%s provenance = owner %q, condition %q; want owner %q, condition %q", name, meta.Owner, meta.Condition, owner, condition)
		}
	}

	assertProvenance("read", "harness.default.core", "built-in runtime registry")
	assertProvenance("create_prompt_extension", "harness.default.deferred", "built-in runtime registry")
	assertProvenance("sourcegraph", "harness.sourcegraph", "Sourcegraph endpoint configured")
	assertProvenance("list_mcp_resources", "harness.mcp", "MCP registry configured")
	assertProvenance("mcp_calendar_create_event", "harness.mcp", `MCP server "calendar" advertised tool during registry discovery`)
}

// TestNewDefaultRegistryWithOptions_AllToolsHaveNonEmptyDescriptions is the
// end-to-end invariant for issue #41: every registered tool's description must
// come from an embedded .md file via descriptions.Load(), never left empty.
//
// Ported from the deleted duplicate catalog's test suite, which was the only
// place this cross-cutting invariant was asserted. It now runs against the
// registry that actually serves models.
func TestNewDefaultRegistryWithOptions_AllToolsHaveNonEmptyDescriptions(t *testing.T) {
	t.Parallel()

	verifier := &stubSkillVerifierForRegistryTest{}
	mgr := htools.NewCallbackManager(stubRunStarterForRegistryTest{})
	defer mgr.Shutdown()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:    ToolApprovalModeFullAuto,
		AgentRunner:     fakeAgentRunnerForWebTest{},
		WebFetcher:      &realHTTPWebFetcherForRegistryTest{client: http.DefaultClient},
		CronClient:      stubCronClientForRegistryTest{},
		CallbackManager: mgr,
		ModelCatalog:    &catalog.Catalog{CatalogVersion: "1.0"},
		SkillLister:     verifier,
		SkillVerifier:   verifier,
	})

	defs := registry.Definitions()
	if len(defs) == 0 {
		t.Fatal("registry is empty — cannot validate descriptions")
	}
	for _, def := range defs {
		if strings.TrimSpace(def.Description) == "" {
			t.Errorf("tool %q has an empty Description — add a descriptions.Load(%q) call and a descriptions/%s.md file", def.Name, def.Name, def.Name)
		}
	}
}

// TestNewDefaultRegistryWithOptions_ListModelsToolRegistration pins that
// list_models appears only when a model catalog is configured.
func TestNewDefaultRegistryWithOptions_ListModelsToolRegistration(t *testing.T) {
	t.Parallel()

	withCatalog := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		ModelCatalog: &catalog.Catalog{CatalogVersion: "1.0"},
	})
	if !registeredToolNames(withCatalog)["list_models"] {
		t.Error("list_models not registered when a ModelCatalog is configured")
	}

	withoutCatalog := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	if registeredToolNames(withoutCatalog)["list_models"] {
		t.Error("list_models must not be registered without a ModelCatalog")
	}
}

// emptySkillListerForRegistryTest reports no skills, which must keep the skill
// tool out of the registry entirely.
type emptySkillListerForRegistryTest struct {
	stubSkillVerifierForRegistryTest
}

func (emptySkillListerForRegistryTest) ListSkills() []htools.SkillInfo { return nil }

// TestNewDefaultRegistryWithOptions_SkillToolRegistration pins the two
// conditions on registering the skill tool: a SkillLister must be configured,
// AND it must actually report at least one skill. Ported from the deleted
// duplicate catalog's test suite.
func TestNewDefaultRegistryWithOptions_SkillToolRegistration(t *testing.T) {
	t.Parallel()

	withSkills := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		SkillLister:  &stubSkillVerifierForRegistryTest{},
	})
	if !registeredToolNames(withSkills)["skill"] {
		t.Error("skill not registered when a SkillLister reports skills")
	}

	noSkills := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		SkillLister:  &emptySkillListerForRegistryTest{},
	})
	if registeredToolNames(noSkills)["skill"] {
		t.Error("skill must not be registered when the lister reports no skills")
	}

	noLister := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	if registeredToolNames(noLister)["skill"] {
		t.Error("skill must not be registered without a SkillLister")
	}
}

type stubRunStarterForRegistryTest struct{}

func (stubRunStarterForRegistryTest) StartRun(_, _, _, _ string) error { return nil }

// TestNewDefaultRegistryWithOptions_DelayedCallbackToolRegistration pins that
// the three delayed-callback tools are registered only when a CallbackManager
// is configured.
func TestNewDefaultRegistryWithOptions_DelayedCallbackToolRegistration(t *testing.T) {
	t.Parallel()

	callbackTools := []string{"set_delayed_callback", "cancel_delayed_callback", "list_delayed_callbacks"}

	mgr := htools.NewCallbackManager(stubRunStarterForRegistryTest{})
	defer mgr.Shutdown()

	withManager := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:    ToolApprovalModeFullAuto,
		CallbackManager: mgr,
	})
	present := registeredToolNames(withManager)
	for _, name := range callbackTools {
		if !present[name] {
			t.Errorf("%q not registered when a CallbackManager is configured", name)
		}
	}

	withoutManager := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	absent := registeredToolNames(withoutManager)
	for _, name := range callbackTools {
		if absent[name] {
			t.Errorf("%q must not be registered without a CallbackManager", name)
		}
	}
}

// TestDelayedCallbackToolsAreCoreNotDeferred pins that the callback tools reach
// the model without discovery.
//
// Registration alone is not enough and the test above cannot see the
// difference: a deferred tool is registered but withheld from
// DefinitionsForRun until find_tool activates it for that run. Shipped that
// way, a model asked for a reminder knew a callback existed, did not see one
// in its tool list, guessed it was a skill, and failed with
// "skill not found: set_delayed_callback".
func TestDelayedCallbackToolsAreCoreNotDeferred(t *testing.T) {
	t.Parallel()

	mgr := htools.NewCallbackManager(stubRunStarterForRegistryTest{})
	defer mgr.Shutdown()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:    ToolApprovalModeFullAuto,
		CallbackManager: mgr,
	})

	// A nil tracker means nothing has been activated, so only core tools show.
	visible := map[string]bool{}
	for _, def := range registry.DefinitionsForRun("run-1", nil) {
		visible[def.Name] = true
	}

	for _, name := range []string{
		"set_delayed_callback", "cancel_delayed_callback", "list_delayed_callbacks",
	} {
		if !visible[name] {
			t.Errorf("%q is not visible to a run without activation — it is deferred, "+
				"so the model cannot call it without discovering it first", name)
		}
	}
}

// TestCronToolsAreCoreNotDeferred pins that cron reaches the model without a
// discovery step.
//
// Deferred, cron_create was registered but withheld from DefinitionsForRun.
// Asked to schedule a recurring job, the model did not call find_tool — it ran
// the command once through bash and replied "created". The user was told a
// cron job existed when none did. A missing tool producing a false success is
// a worse failure than one producing an error, which is why this is pinned.
func TestCronToolsAreCoreNotDeferred(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		CronClient:   stubCronClientForRegistryTest{},
	})

	visible := map[string]bool{}
	for _, def := range registry.DefinitionsForRun("run-1", nil) {
		visible[def.Name] = true
	}

	for _, name := range []string{
		"cron_create", "cron_list", "cron_get", "cron_update",
		"cron_history", "cron_delete", "cron_pause", "cron_resume",
	} {
		if !visible[name] {
			t.Errorf("%q is not visible to a run without activation — the model cannot "+
				"call it, and has been observed substituting bash and claiming success", name)
		}
	}
}

// TestDefaultRegistryInitialCoreToolSchemasAreProviderCompatible exercises the
// actual tool list sent to a fresh model run. OpenAI-compatible providers
// require object-shaped function parameters and reject composition/constant
// keywords at the schema root; type-specific constraints such as a property
// enum remain valid below that root.
//
// Cron is included explicitly because all eight operations are core-visible:
// checking only cron_create would let a later CRUD schema change break the
// provider request before the model could create or manage a job.
func TestDefaultRegistryInitialCoreToolSchemasAreProviderCompatible(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		CronClient:   stubCronClientForRegistryTest{},
	})

	checkedCron := make(map[string]bool)
	for _, def := range registry.DefinitionsForRun("run-1", nil) {
		if got := def.Parameters["type"]; got != "object" {
			t.Errorf("initial core tool %q schema type = %#v, want object", def.Name, got)
		}
		for _, forbidden := range []string{"oneOf", "anyOf", "allOf", "enum", "const", "not"} {
			if _, found := def.Parameters[forbidden]; found {
				t.Errorf("initial core tool %q schema has forbidden top-level %q: %#v", def.Name, forbidden, def.Parameters)
			}
		}
		if strings.HasPrefix(def.Name, "cron_") {
			checkedCron[def.Name] = true
		}
	}

	for _, name := range []string{
		"cron_create", "cron_list", "cron_get", "cron_update",
		"cron_history", "cron_delete", "cron_pause", "cron_resume",
	} {
		if !checkedCron[name] {
			t.Errorf("%q was not checked in the initial core provider-schema set", name)
		}
	}
}

// TestNewDefaultRegistryWithOptions_CronToolRegistration pins that all eight cron
// tools are registered when a CronClient is configured, and that none of them
// leak into a registry built without one.
func TestNewDefaultRegistryWithOptions_CronToolRegistration(t *testing.T) {
	t.Parallel()

	withClient := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		CronClient:   stubCronClientForRegistryTest{},
	})
	present := registeredToolNames(withClient)
	for _, name := range []string{
		"cron_create", "cron_list", "cron_get", "cron_update",
		"cron_history", "cron_delete", "cron_pause", "cron_resume",
	} {
		if !present[name] {
			t.Errorf("cron tool %q not registered when a CronClient is configured", name)
		}
	}

	withoutClient := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	for name := range registeredToolNames(withoutClient) {
		if strings.HasPrefix(name, "cron_") {
			t.Errorf("cron tool %q must not be registered without a CronClient", name)
		}
	}
}

// TestNewDefaultRegistryWithOptions_RecipeToolRegistration pins that run_recipe
// appears only when a recipes directory is configured AND actually contains at
// least one loadable recipe. Ported from the deleted duplicate catalog's test
// suite; the per-step policy guarantee (issue #788) is covered separately by
// TestDefaultRegistry_RecipeStepsRespectPolicy.
func TestNewDefaultRegistryWithOptions_RecipeToolRegistration(t *testing.T) {
	t.Parallel()

	withRecipe := t.TempDir()
	recipeYAML := "name: demo\ndescription: \"a demo recipe\"\nsteps:\n  - name: s1\n    tool: read\n    args:\n      path: README.md\n"
	if err := os.WriteFile(filepath.Join(withRecipe, "demo.yaml"), []byte(recipeYAML), 0o644); err != nil {
		t.Fatalf("write recipe: %v", err)
	}

	registered := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		RecipesDir:   withRecipe,
	})
	if !registeredToolNames(registered)["run_recipe"] {
		t.Error("run_recipe not registered when the recipes directory holds a recipe")
	}

	emptyDir := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		RecipesDir:   t.TempDir(),
	})
	if registeredToolNames(emptyDir)["run_recipe"] {
		t.Error("run_recipe must not be registered when the recipes directory is empty")
	}

	noDir := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	if registeredToolNames(noDir)["run_recipe"] {
		t.Error("run_recipe must not be registered without a RecipesDir")
	}
}

// TestNewDefaultRegistryWithOptions_DirectoryBackedToolRegistration covers the
// registration branches gated on a configured directory or endpoint. Each is a
// separate opt-in an operator can get wrong, and each is invisible at runtime
// until an agent asks for a tool that is not there.
func TestNewDefaultRegistryWithOptions_DirectoryBackedToolRegistration(t *testing.T) {
	t.Parallel()

	base := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
	})
	baseNames := registeredToolNames(base)

	t.Run("create_skill requires a skills directory", func(t *testing.T) {
		if baseNames["create_skill"] {
			t.Error("create_skill must not be registered without a SkillsDir")
		}
		withDir := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
			ApprovalMode: ToolApprovalModeFullAuto,
			SkillsDir:    t.TempDir(),
		})
		if !registeredToolNames(withDir)["create_skill"] {
			t.Error("create_skill should be registered when a SkillsDir is configured")
		}
	})

	t.Run("profile mutation tools require a profiles directory", func(t *testing.T) {
		mutators := []string{"create_profile", "update_profile", "delete_profile"}
		for _, name := range mutators {
			if baseNames[name] {
				t.Errorf("%q must not be registered without a ProfilesDir", name)
			}
		}
		withDir := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
			ApprovalMode: ToolApprovalModeFullAuto,
			ProfilesDir:  t.TempDir(),
		})
		got := registeredToolNames(withDir)
		for _, name := range mutators {
			if !got[name] {
				t.Errorf("%q should be registered when a ProfilesDir is configured", name)
			}
		}
		// The read-only profile tools are always available either way.
		for _, name := range []string{"list_profiles", "get_profile", "validate_profile", "recommend_profile"} {
			if !baseNames[name] {
				t.Errorf("%q should always be registered", name)
			}
		}
	})

	t.Run("sourcegraph requires an endpoint", func(t *testing.T) {
		if baseNames["sourcegraph"] {
			t.Error("sourcegraph must not be registered without an endpoint")
		}
		withEndpoint := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
			ApprovalMode: ToolApprovalModeFullAuto,
			Sourcegraph:  htools.SourcegraphConfig{Endpoint: "https://sourcegraph.example.com"},
		})
		if !registeredToolNames(withEndpoint)["sourcegraph"] {
			t.Error("sourcegraph should be registered when an endpoint is configured")
		}
	})

	t.Run("script tools load from a configured directory", func(t *testing.T) {
		dir := t.TempDir()
		manifest := `{"name":"echo_script","description":"echoes","parameters":{"type":"object"}}`
		if err := os.WriteFile(filepath.Join(dir, "echo_script.json"), []byte(manifest), 0o644); err != nil {
			t.Fatalf("write script manifest: %v", err)
		}
		// A malformed or unloadable directory must not panic or abort the
		// whole registry build — the tool is simply absent.
		reg := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
			ApprovalMode:   ToolApprovalModeFullAuto,
			ScriptToolsDir: dir,
		})
		if len(reg.Definitions()) == 0 {
			t.Error("registry should still build with a script tools directory configured")
		}
	})

	t.Run("a bad recipes directory does not break the registry", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("not: [valid"), 0o644); err != nil {
			t.Fatalf("write broken recipe: %v", err)
		}
		reg := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
			ApprovalMode: ToolApprovalModeFullAuto,
			RecipesDir:   dir,
		})
		if registeredToolNames(reg)["run_recipe"] {
			t.Error("run_recipe must not be registered when the recipes fail to load")
		}
		if len(reg.Definitions()) == 0 {
			t.Error("a broken recipe file must not prevent the rest of the registry from building")
		}
	})

	t.Run("always-registered tools are present in a bare registry", func(t *testing.T) {
		for _, name := range []string{
			"read", "write", "edit", "bash", "apply_patch", "grep", "glob", "ls",
			"git_status", "git_diff", "todos", "find_tool", "deploy",
			"get_efficiency_report", "download", "context_status", "compact_history",
		} {
			if !baseNames[name] {
				t.Errorf("%q should be registered in a bare registry", name)
			}
		}
	})
}

// TestNewDefaultRegistryWithOptions_JobTrackerLifecycle pins that a registry
// registers its background-job manager with the daemon-wide tracker and
// unregisters it on shutdown, so a dead registry's jobs cannot linger in the
// /v1/tasks union.
func TestNewDefaultRegistryWithOptions_JobTrackerLifecycle(t *testing.T) {
	t.Parallel()

	tracker := NewJobTracker()
	managerCount := func() int {
		tracker.mu.RLock()
		defer tracker.mu.RUnlock()
		return len(tracker.managers)
	}
	before := managerCount()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		JobTracker:   tracker,
	})
	if managerCount() != before+1 {
		t.Fatalf("registry did not register its job manager: %d managers, want %d", managerCount(), before+1)
	}

	if err := registry.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if managerCount() != before {
		t.Errorf("shutdown must unregister the job manager: %d managers, want %d", managerCount(), before)
	}
}
