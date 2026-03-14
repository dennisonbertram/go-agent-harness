package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go-agent-harness/internal/config"
	"go-agent-harness/internal/cron"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/mcp"
	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
	anthropic "go-agent-harness/internal/provider/anthropic"
	openai "go-agent-harness/internal/provider/openai"
	"go-agent-harness/internal/provider/pricing"
	"go-agent-harness/internal/server"
	"go-agent-harness/internal/skills"
	"go-agent-harness/internal/systemprompt"
	"go-agent-harness/internal/watcher"
)

// callbackRunStarter is a lazy adapter that bridges the CallbackManager's
// RunStarter interface to the harness Runner. It uses a mutex-guarded pointer
// so the CallbackManager can be created before the Runner exists.
type callbackRunStarter struct {
	mu     sync.Mutex
	runner *harness.Runner
}

func (a *callbackRunStarter) StartRun(prompt, conversationID string) error {
	a.mu.Lock()
	r := a.runner
	a.mu.Unlock()
	if r == nil {
		return fmt.Errorf("runner not yet initialized")
	}
	_, err := r.StartRun(harness.RunRequest{
		Prompt:         prompt,
		ConversationID: conversationID,
	})
	return err
}

type providerFactory func(cfg openai.Config) (harness.Provider, error)

// profileFlag is the --profile CLI flag. Registered at package level so it
// integrates cleanly with Go's test infrastructure flags.
var profileFlag = flag.String("profile", "", "named profile to load from ~/.harness/profiles/<name>.toml")

var (
	runMain            = run
	exitFunc           = os.Exit
	runWithSignalsFunc = runWithSignals
)

func main() {
	flag.Parse()
	if err := runMain(); err != nil {
		log.Printf("fatal: %v", err)
		exitFunc(1)
	}
}

func run() error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	return runWithSignalsFunc(sig, os.Getenv, func(cfg openai.Config) (harness.Provider, error) {
		return openai.NewClient(cfg)
	}, *profileFlag)
}

func runWithSignals(sig <-chan os.Signal, getenv func(string) string, newProvider providerFactory, profileName string) error {
	if sig == nil {
		return fmt.Errorf("signal channel is required")
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if newProvider == nil {
		newProvider = func(config openai.Config) (harness.Provider, error) {
			return openai.NewClient(config)
		}
	}

	// Local helpers that use the injected getenv instead of os.Getenv,
	// so tests can override environment values without touching the real env.
	envOrDefault := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}
	envIntOrDefault := func(key string, fallback int) int {
		v := getenv(key)
		if v == "" {
			return fallback
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fallback
		}
		return n
	}
	envToolApprovalModeOrDefault := func(key string, fallback harness.ToolApprovalMode) harness.ToolApprovalMode {
		value := strings.TrimSpace(strings.ToLower(getenv(key)))
		if value == "" {
			return fallback
		}
		switch harness.ToolApprovalMode(value) {
		case harness.ToolApprovalModeFullAuto, harness.ToolApprovalModePermissions:
			return harness.ToolApprovalMode(value)
		default:
			return fallback
		}
	}
	envMemoryModeOrDefault := func(key string, fallback om.Mode) om.Mode {
		value := strings.TrimSpace(strings.ToLower(getenv(key)))
		if value == "" {
			return fallback
		}
		switch om.Mode(value) {
		case om.ModeAuto, om.ModeOff, om.ModeLocalCoordinator:
			return om.Mode(value)
		default:
			return fallback
		}
	}
	envBoolOrDefault := func(key string, fallback bool) bool {
		value := strings.TrimSpace(strings.ToLower(getenv(key)))
		if value == "" {
			return fallback
		}
		switch value {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		default:
			return fallback
		}
	}

	apiKey := getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required")
	}

	// Load the layered configuration stack (layers 1–5).
	// This resolves model, addr, max_steps, and cost settings from:
	//   ~/.harness/config.toml → .harness/config.toml → profile → HARNESS_* env vars.
	home, _ := os.UserHomeDir()
	workspace := envOrDefault("HARNESS_WORKSPACE", ".")
	harnessConfigDir := filepath.Join(home, ".harness")
	harnessProfilesDir := filepath.Join(harnessConfigDir, "profiles")
	harnessUserConfig := filepath.Join(harnessConfigDir, "config.toml")
	harnessProjectConfig := filepath.Join(workspace, ".harness", "config.toml")
	harnessCfg, cfgErr := config.Load(config.LoadOptions{
		UserConfigPath:    harnessUserConfig,
		ProjectConfigPath: harnessProjectConfig,
		ProfilesDir:       harnessProfilesDir,
		ProfileName:       profileName,
		Getenv:            getenv,
	})
	if cfgErr != nil {
		return fmt.Errorf("load config: %w", cfgErr)
	}
	harnessCfg = harnessCfg.Resolve()

	// Use the resolved config values. HARNESS_MODEL, HARNESS_ADDR,
	// HARNESS_MAX_STEPS, and HARNESS_MAX_COST_PER_RUN_USD env vars are
	// already applied by the config stack at layer 5 — backward-compatible.
	model := harnessCfg.Model
	addr := harnessCfg.Addr
	maxSteps := harnessCfg.MaxSteps
	// When maxSteps is 0 from config (unlimited), preserve the old default of 8
	// unless the user has explicitly set HARNESS_MAX_STEPS.
	if maxSteps == 0 && getenv("HARNESS_MAX_STEPS") == "" {
		maxSteps = 8
	}

	systemPrompt := envOrDefault("HARNESS_SYSTEM_PROMPT", "You are a practical coding assistant. Prefer using tools for file inspection and tests when needed.")
	defaultAgentIntent := envOrDefault("HARNESS_DEFAULT_AGENT_INTENT", "general")
	promptsDir := strings.TrimSpace(envOrDefault("HARNESS_PROMPTS_DIR", findDefaultPromptsDir()))
	askUserTimeoutSeconds := envIntOrDefault("HARNESS_ASK_USER_TIMEOUT_SECONDS", 300)
	approvalMode := envToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModeFullAuto)
	memoryMode := envMemoryModeOrDefault("HARNESS_MEMORY_MODE", om.ModeAuto)
	memoryDriver := strings.TrimSpace(strings.ToLower(envOrDefault("HARNESS_MEMORY_DB_DRIVER", "sqlite")))
	memoryDBDSN := strings.TrimSpace(getenv("HARNESS_MEMORY_DB_DSN"))
	memorySQLitePath := strings.TrimSpace(envOrDefault("HARNESS_MEMORY_SQLITE_PATH", ".harness/state.db"))
	memoryDefaultEnabled := envBoolOrDefault("HARNESS_MEMORY_DEFAULT_ENABLED", false)
	memoryObserveMinTokens := envIntOrDefault("HARNESS_MEMORY_OBSERVE_MIN_TOKENS", 1200)
	memorySnippetMaxTokens := envIntOrDefault("HARNESS_MEMORY_SNIPPET_MAX_TOKENS", 900)
	memoryReflectThresholdTokens := envIntOrDefault("HARNESS_MEMORY_REFLECT_THRESHOLD_TOKENS", 4000)
	memoryLLMMode := strings.TrimSpace(strings.ToLower(envOrDefault("HARNESS_MEMORY_LLM_MODE", "openai")))
	memoryLLMModel := strings.TrimSpace(envOrDefault("HARNESS_MEMORY_LLM_MODEL", "gpt-5-nano"))
	memoryLLMBaseURL := strings.TrimSpace(envOrDefault("HARNESS_MEMORY_LLM_BASE_URL", getenv("OPENAI_BASE_URL")))
	memoryLLMAPIKey := strings.TrimSpace(envOrDefault("HARNESS_MEMORY_LLM_API_KEY", apiKey))
	pricingCatalogPath := strings.TrimSpace(getenv("HARNESS_PRICING_CATALOG_PATH"))
	modelCatalogPath := strings.TrimSpace(getenv("HARNESS_MODEL_CATALOG_PATH"))
	// Auto-discover catalog from workspace when env var not set.
	if modelCatalogPath == "" {
		candidates := []string{
			filepath.Join(workspace, "catalog", "models.json"),
			"catalog/models.json",
		}
		for _, p := range candidates {
			if _, statErr := os.Stat(p); statErr == nil {
				modelCatalogPath = p
				break
			}
		}
	}
	skillsEnabled := envBoolOrDefault("HARNESS_SKILLS_ENABLED", true)
	watchEnabled := envBoolOrDefault("HARNESS_WATCH_ENABLED", true)
	watchIntervalSeconds := envIntOrDefault("HARNESS_WATCH_INTERVAL_SECONDS", 5)
	recipesDir := strings.TrimSpace(getenv("HARNESS_RECIPES_DIR"))
	cronURL := strings.TrimSpace(getenv("HARNESS_CRON_URL"))
	callbacksEnabled := envBoolOrDefault("HARNESS_ENABLE_CALLBACKS", true)
	sourcegraphEndpoint := strings.TrimSpace(getenv("HARNESS_SOURCEGRAPH_ENDPOINT"))
	sourcegraphToken := strings.TrimSpace(getenv("HARNESS_SOURCEGRAPH_TOKEN"))
	rolloutDir := strings.TrimSpace(getenv("HARNESS_ROLLOUT_DIR"))

	var providerRegistry *catalog.ProviderRegistry
	var modelCatalog *catalog.Catalog
	if modelCatalogPath != "" {
		cat, err := catalog.LoadCatalog(modelCatalogPath)
		if err != nil {
			log.Printf("warning: failed to load model catalog from %s: %v (continuing without catalog)", modelCatalogPath, err)
		} else {
			modelCatalog = cat
			providerRegistry = catalog.NewProviderRegistryWithEnv(cat, getenv)
			log.Printf("loaded model catalog with %d providers", len(cat.Providers))
		}
	}

	// lookupModelAPI routes models to the correct API endpoint (e.g. "responses" for Codex models).
	// Returns the catalog's "api" field value for the given model, or "" for standard chat/completions.
	lookupModelAPI := func(providerName, modelID string) string {
		if modelCatalog == nil {
			return ""
		}
		entry, ok := modelCatalog.Providers[providerName]
		if !ok {
			return ""
		}
		// Resolve alias if needed.
		resolved := modelID
		if target, ok := entry.Aliases[modelID]; ok {
			if _, exists := entry.Models[target]; exists {
				resolved = target
			}
		}
		m, ok := entry.Models[resolved]
		if !ok {
			return ""
		}
		return m.API
	}

	var pricingResolver pricing.Resolver
	if pricingCatalogPath != "" {
		resolver, err := pricing.NewFileResolver(pricingCatalogPath)
		if err != nil {
			return fmt.Errorf("load pricing catalog from %s: %w", pricingCatalogPath, err)
		}
		pricingResolver = resolver
	}

	if providerRegistry != nil {
		providerRegistry.SetClientFactory(func(apiKey, baseURL, providerName string) (catalog.ProviderClient, error) {
			if providerName == "anthropic" {
				return anthropic.NewClient(anthropic.Config{
					APIKey:          apiKey,
					BaseURL:         baseURL,
					ProviderName:    providerName,
					PricingResolver: pricingResolver,
				})
			}
			return newProvider(openai.Config{
				APIKey:          apiKey,
				BaseURL:         baseURL,
				ProviderName:    providerName,
				PricingResolver: pricingResolver,
				ModelAPILookup:  lookupModelAPI,
			})
		})
	}

	provider, err := newProvider(openai.Config{
		APIKey:          apiKey,
		BaseURL:         getenv("OPENAI_BASE_URL"),
		Model:           model,
		PricingResolver: pricingResolver,
		ModelAPILookup:  lookupModelAPI,
	})
	if err != nil {
		return fmt.Errorf("create openai provider: %w", err)
	}
	promptEngine, err := systemprompt.NewFileEngine(promptsDir)
	if err != nil {
		return fmt.Errorf("load prompt engine from %s: %w", promptsDir, err)
	}

	memoryManager, err := newObservationalMemoryManager(observationalMemoryManagerOptions{
		Mode:           memoryMode,
		Driver:         memoryDriver,
		DBDSN:          memoryDBDSN,
		SQLitePath:     memorySQLitePath,
		WorkspaceRoot:  workspace,
		Provider:       provider,
		Model:          model,
		DefaultEnabled: memoryDefaultEnabled,
		DefaultConfig: om.Config{
			ObserveMinTokens:       memoryObserveMinTokens,
			SnippetMaxTokens:       memorySnippetMaxTokens,
			ReflectThresholdTokens: memoryReflectThresholdTokens,
		},
		MemoryLLMMode:    memoryLLMMode,
		MemoryLLMModel:   memoryLLMModel,
		MemoryLLMBaseURL: memoryLLMBaseURL,
		MemoryLLMAPIKey:  memoryLLMAPIKey,
	})
	if err != nil {
		return fmt.Errorf("create observational memory manager: %w", err)
	}
	defer func() {
		if memoryManager != nil {
			_ = memoryManager.Close()
		}
	}()

	// Skills system
	globalDir := envOrDefault("HARNESS_GLOBAL_DIR", filepath.Join(home, ".go-harness"))
	var skillLister htools.SkillLister
	var skillLoader *skills.Loader  // retained for hot-reload
	var skillRegistry *skills.Registry // retained for hot-reload
	if skillsEnabled {
		skillLoader = skills.NewLoader(skills.LoaderConfig{
			GlobalDir:    filepath.Join(globalDir, "skills"),
			WorkspaceDir: filepath.Join(workspace, ".go-harness", "skills"),
		})
		skillRegistry = skills.NewRegistry()
		if err := skillRegistry.Load(skillLoader); err != nil {
			log.Printf("warning: failed to load skills: %v (continuing without skills)", err)
			skillRegistry = nil
		} else {
			skillResolver := skills.NewResolver(skillRegistry)
			promptEngine.SetSkillResolver(skillResolver)
			skillLister = &skillListerAdapter{registry: skillRegistry, resolver: skillResolver, workspace: workspace}
			loaded := skillRegistry.List()
			if len(loaded) > 0 {
				log.Printf("loaded %d skill(s)", len(loaded))
			}
		}
	}

	var cronClient htools.CronClient
	var cronStore cron.Store
	var cronScheduler *cron.Scheduler

	if cronURL != "" {
		cronClient = &cronClientAdapter{client: cron.NewClient(cronURL)}
	} else {
		cronDBPath := filepath.Join(workspace, ".harness", "cron.db")
		st, err := cron.NewSQLiteStore(cronDBPath)
		if err != nil {
			return fmt.Errorf("create cron store: %w", err)
		}
		if err := st.Migrate(context.Background()); err != nil {
			st.Close()
			return fmt.Errorf("migrate cron store: %w", err)
		}
		clock := cron.RealClock{}
		sched := cron.NewScheduler(st, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 5})
		if err := sched.Start(context.Background()); err != nil {
			st.Close()
			return fmt.Errorf("start cron scheduler: %w", err)
		}
		cronStore = st
		cronScheduler = sched
		cronClient = &embeddedCronAdapter{store: st, scheduler: sched, clock: clock}
		log.Printf("embedded cron scheduler started (db: %s)", cronDBPath)
	}

	// Delayed callbacks
	var callbackStarter *callbackRunStarter
	var callbackMgr *htools.CallbackManager
	if callbacksEnabled {
		callbackStarter = &callbackRunStarter{}
		callbackMgr = htools.NewCallbackManager(callbackStarter)
		log.Printf("delayed callbacks enabled")
	}

	// MCP server startup: TOML config (layers 1-3, no profile) then env var
	// servers are registered additively. TOML entries take precedence over env
	// var entries with the same name.
	mcpManager := mcp.NewClientManager()
	defer func() { _ = mcpManager.Close() }() // safety-net defer; explicit close is in shutdown sequence below
	{
		// Load config WITHOUT a profile so the global ClientManager only gets
		// layers 1-3 (user global + project); profile-specific servers are
		// scoped to individual runs, not the global manager.
		globalCfg, globalCfgErr := config.Load(config.LoadOptions{
			UserConfigPath:    harnessUserConfig,
			ProjectConfigPath: harnessProjectConfig,
			Getenv:            getenv,
		})
		if globalCfgErr != nil {
			log.Printf("warning: failed to load config for MCP server registration: %v (continuing without TOML-configured MCP servers)", globalCfgErr)
		}

		var envServers []mcp.ServerConfig
		mcpConfigs, mcpErr := mcp.ParseMCPServersEnvWith(getenv)
		if mcpErr != nil {
			log.Printf("warning: failed to parse %s: %v (continuing without env-configured MCP servers)", mcp.EnvVarMCPServers, mcpErr)
		} else {
			envServers = mcpConfigs
		}

		registerMCPServersFromConfig(mcpManager, globalCfg.MCPServers, envServers, log.Printf)
	}

	// Conversation persistence
	convRetentionDays := envIntOrDefault("HARNESS_CONVERSATION_RETENTION_DAYS", 30)
	var convStore harness.ConversationStore
	var convCleanerCtx context.Context
	var convCleanerCancel context.CancelFunc
	if dbPath := getenv("HARNESS_CONVERSATION_DB"); dbPath != "" {
		if !filepath.IsAbs(dbPath) {
			dbPath = filepath.Join(workspace, dbPath)
		}
		store, err := harness.NewSQLiteConversationStore(dbPath)
		if err != nil {
			return fmt.Errorf("create conversation store: %w", err)
		}
		if err := store.Migrate(context.Background()); err != nil {
			store.Close()
			return fmt.Errorf("migrate conversation store: %w", err)
		}
		convStore = store
		defer store.Close()
		log.Printf("conversation persistence enabled: %s", dbPath)

		// Start retention cleaner background goroutine.
		if convRetentionDays > 0 {
			log.Printf("conversation retention policy: %d days", convRetentionDays)
			convCleanerCtx, convCleanerCancel = context.WithCancel(context.Background())
			cleaner := harness.NewConversationCleaner(store, convRetentionDays)
			cleaner.Start(convCleanerCtx, 24*time.Hour)
		}
	}

	askUserBroker := harness.NewInMemoryAskUserQuestionBroker(time.Now)
	activations := harness.NewActivationTracker()
	msgSummarizer := &lazySummarizer{}
	promptBehaviorsDir, promptTalentsDir := promptEngine.ExtensionDirs()
	tools := harness.NewDefaultRegistryWithOptions(workspace, harness.DefaultRegistryOptions{
		ApprovalMode:    approvalMode,
		Policy:          nil,
		AskUserBroker:   askUserBroker,
		AskUserTimeout:  time.Duration(askUserTimeoutSeconds) * time.Second,
		MemoryManager:   memoryManager,
		SkillLister:     skillLister,
		SkillsDir:       filepath.Join(globalDir, "skills"),
		ModelCatalog:    modelCatalog,
		CronClient:      cronClient,
		CallbackManager: callbackMgr,
		Activations:     activations,
		Sourcegraph: htools.SourcegraphConfig{
			Endpoint: sourcegraphEndpoint,
			Token:    sourcegraphToken,
		},
		RecipesDir: recipesDir,
		PromptExtensionDirs: htools.PromptExtensionDirs{
			BehaviorsDir: promptBehaviorsDir,
			TalentsDir:   promptTalentsDir,
		},
		ScriptToolsDir:    filepath.Join(globalDir, "tools"),
		ConversationStore: convStore,
		MessageSummarizer: msgSummarizer,
	})
	if rolloutDir != "" {
		log.Printf("rollout recording enabled: %s", rolloutDir)
	}
	runner := harness.NewRunner(provider, tools, harness.RunnerConfig{
		DefaultModel:        model,
		DefaultSystemPrompt: systemPrompt,
		DefaultAgentIntent:  defaultAgentIntent,
		MaxSteps:            maxSteps,
		AskUserTimeout:      time.Duration(askUserTimeoutSeconds) * time.Second,
		AskUserBroker:       askUserBroker,
		MemoryManager:       memoryManager,
		PromptEngine:        promptEngine,
		ToolApprovalMode:    approvalMode,
		ProviderRegistry:    providerRegistry,
		ConversationStore:   convStore,
		Logger:              &stdLogger{},
		Activations:         activations,
		RolloutDir:          rolloutDir,
	})

	// Wire the runner into the callback adapter now that it exists
	if callbackStarter != nil {
		callbackStarter.mu.Lock()
		callbackStarter.runner = runner
		callbackStarter.mu.Unlock()
	}

	// Wire the message summarizer now that the runner exists
	msgSummarizer.mu.Lock()
	msgSummarizer.summarizer = runner.NewMessageSummarizer()
	msgSummarizer.mu.Unlock()

	// Hot-reload file watcher: monitors skills directories and reloads
	// when SKILL.md files are created, modified, or deleted.
	watchCtx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()

	if watchEnabled && skillsEnabled && skillRegistry != nil && skillLoader != nil {
		pollInterval := time.Duration(watchIntervalSeconds) * time.Second
		w := watcher.New(pollInterval)

		reloadSkills := func() error {
			if err := skillRegistry.Reload(skillLoader); err != nil {
				log.Printf("watcher: skill reload error: %v", err)
				return err
			}
			log.Printf("watcher: skills reloaded (%d skill(s))", len(skillRegistry.List()))
			return nil
		}

		globalSkillsDir := filepath.Join(globalDir, "skills")
		workspaceSkillsDir := filepath.Join(workspace, ".go-harness", "skills")

		w.Watch(watcher.WatchedDir{Path: globalSkillsDir, Reload: reloadSkills})
		w.Watch(watcher.WatchedDir{Path: workspaceSkillsDir, Reload: reloadSkills})

		go w.Start(watchCtx)
		log.Printf("hot-reload watcher started (interval: %s, dirs: %s, %s)",
			pollInterval, globalSkillsDir, workspaceSkillsDir)
	}

	handler := server.NewWithCatalog(runner, modelCatalog)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		log.Printf("harness server listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-sig:
	}

	// Shut down callbacks before the HTTP server to prevent new runs during shutdown
	if callbackMgr != nil {
		callbackMgr.Shutdown()
	}

	// Shut down conversation retention cleaner goroutine.
	if convCleanerCancel != nil {
		convCleanerCancel()
	}

	// Shut down embedded cron scheduler
	if cronScheduler != nil {
		cronScheduler.Stop()
	}
	if cronStore != nil {
		cronStore.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	// Explicitly close MCP connections after the HTTP server has drained
	// in-flight requests. This ensures any tool calls that were in progress
	// finish before their underlying MCP connections are torn down.
	// The defer above is kept as a safety net for abnormal exit paths.
	if err := mcpManager.Close(); err != nil {
		log.Printf("mcp shutdown error: %v", err)
	}

	select {
	case err := <-serverErr:
		return err
	case <-serverDone:
	}
	return nil
}

func getenvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func findDefaultPromptsDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "prompts"
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "prompts")
		if _, statErr := os.Stat(filepath.Join(candidate, "catalog.yaml")); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "prompts"
}

func getenvIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func getenvToolApprovalModeOrDefault(key string, fallback harness.ToolApprovalMode) harness.ToolApprovalMode {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch harness.ToolApprovalMode(value) {
	case harness.ToolApprovalModeFullAuto, harness.ToolApprovalModePermissions:
		return harness.ToolApprovalMode(value)
	default:
		return fallback
	}
}

func getenvMemoryModeOrDefault(key string, fallback om.Mode) om.Mode {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch om.Mode(value) {
	case om.ModeAuto, om.ModeOff, om.ModeLocalCoordinator:
		return om.Mode(value)
	default:
		return fallback
	}
}

func getenvBoolOrDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

// stdLogger wraps log.Printf to implement harness.Logger.
type stdLogger struct{}

func (l *stdLogger) Error(msg string, keysAndValues ...any) {
	args := []any{msg}
	args = append(args, keysAndValues...)
	log.Println(args...)
}

type observationalMemoryManagerOptions struct {
	Mode             om.Mode
	Driver           string
	DBDSN            string
	SQLitePath       string
	WorkspaceRoot    string
	Provider         harness.Provider
	Model            string
	DefaultEnabled   bool
	DefaultConfig    om.Config
	MemoryLLMMode    string
	MemoryLLMModel   string
	MemoryLLMBaseURL string
	MemoryLLMAPIKey  string
}

func newObservationalMemoryManager(opts observationalMemoryManagerOptions) (om.Manager, error) {
	mode := opts.Mode
	if mode == "" {
		mode = om.ModeAuto
	}
	if mode == om.ModeOff {
		return om.NewDisabledManager(mode), nil
	}

	var store om.Store
	switch opts.Driver {
	case "", "sqlite":
		path := opts.SQLitePath
		if strings.TrimSpace(path) == "" {
			path = ".harness/state.db"
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(opts.WorkspaceRoot, path)
		}
		sqliteStore, err := om.NewSQLiteStore(path)
		if err != nil {
			return nil, err
		}
		store = sqliteStore
	case "postgres":
		pgStore, err := om.NewPostgresStore(opts.DBDSN)
		if err != nil {
			return nil, err
		}
		store = pgStore
	default:
		return nil, fmt.Errorf("unsupported memory db driver %q", opts.Driver)
	}

	var model om.Model
	llmMode := strings.TrimSpace(strings.ToLower(opts.MemoryLLMMode))
	if llmMode == "" {
		llmMode = "inherit"
	}
	switch llmMode {
	case "inherit":
		model = observationalMemoryModel{
			provider: opts.Provider,
			model:    opts.Model,
		}
	case "openai":
		openAIModel, err := om.NewOpenAIModel(om.OpenAIConfig{
			APIKey:  opts.MemoryLLMAPIKey,
			BaseURL: opts.MemoryLLMBaseURL,
			Model:   opts.MemoryLLMModel,
		})
		if err != nil {
			return nil, fmt.Errorf("create observational memory openai model: %w", err)
		}
		model = openAIModel
	default:
		return nil, fmt.Errorf("unsupported memory llm mode %q", opts.MemoryLLMMode)
	}

	return om.NewService(om.ServiceOptions{
		Mode:           mode,
		Store:          store,
		Coordinator:    om.NewLocalCoordinator(),
		Observer:       om.ModelObserver{Model: model},
		Reflector:      om.ModelReflector{Model: model},
		Estimator:      om.RuneTokenEstimator{},
		DefaultConfig:  opts.DefaultConfig,
		DefaultEnabled: opts.DefaultEnabled,
		Now:            time.Now,
	})
}

// lazySummarizer implements htools.MessageSummarizer with deferred runner binding.
// The runner is created after the tool registry, so this adapter allows the
// compact_history tool to access the runner's summarization capability.
type lazySummarizer struct {
	mu        sync.Mutex
	summarizer htools.MessageSummarizer
}

func (s *lazySummarizer) SummarizeMessages(ctx context.Context, msgs []map[string]any) (string, error) {
	s.mu.Lock()
	inner := s.summarizer
	s.mu.Unlock()
	if inner == nil {
		return "", fmt.Errorf("summarizer not configured yet")
	}
	return inner.SummarizeMessages(ctx, msgs)
}

type observationalMemoryModel struct {
	provider harness.Provider
	model    string
}

func (m observationalMemoryModel) Complete(ctx context.Context, req om.ModelRequest) (string, error) {
	if m.provider == nil {
		return "", fmt.Errorf("provider is required")
	}
	messages := make([]harness.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, harness.Message{Role: msg.Role, Content: msg.Content})
	}
	result, err := m.provider.Complete(ctx, harness.CompletionRequest{
		Model:    m.model,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Content), nil
}

// skillListerAdapter bridges skills.Registry to htools.SkillLister.
type skillListerAdapter struct {
	registry  *skills.Registry
	resolver  *skills.Resolver
	workspace string
}

func (a *skillListerAdapter) GetSkill(name string) (htools.SkillInfo, bool) {
	s, ok := a.registry.Get(name)
	if !ok {
		return htools.SkillInfo{}, false
	}
	return htools.SkillInfo{
		Name:         s.Name,
		Description:  s.Description,
		ArgumentHint: s.ArgumentHint,
		AllowedTools: s.AllowedTools,
		Source:       string(s.Source),
		Context:      string(s.Context),
		Agent:        s.Agent,
		Verified:     s.Verified,
		VerifiedAt:   s.VerifiedAt,
		VerifiedBy:   s.VerifiedBy,
		FilePath:     s.FilePath,
	}, true
}

func (a *skillListerAdapter) ListSkills() []htools.SkillInfo {
	all := a.registry.List()
	result := make([]htools.SkillInfo, len(all))
	for i, s := range all {
		result[i] = htools.SkillInfo{
			Name:         s.Name,
			Description:  s.Description,
			ArgumentHint: s.ArgumentHint,
			AllowedTools: s.AllowedTools,
			Source:       string(s.Source),
			Context:      string(s.Context),
			Agent:        s.Agent,
			Verified:     s.Verified,
			VerifiedAt:   s.VerifiedAt,
			VerifiedBy:   s.VerifiedBy,
			FilePath:     s.FilePath,
		}
	}
	return result
}

func (a *skillListerAdapter) ResolveSkill(ctx context.Context, name, args, workspace string) (string, error) {
	ws := workspace
	if ws == "" {
		ws = a.workspace
	}
	return a.resolver.ResolveSkill(ctx, name, args, ws)
}

// cronClientAdapter bridges cron.Client to htools.CronClient.
type cronClientAdapter struct {
	client *cron.Client
}

func (a *cronClientAdapter) CreateJob(ctx context.Context, req htools.CronCreateJobRequest) (htools.CronJob, error) {
	j, err := a.client.CreateJob(ctx, cron.CreateJobRequest{
		Name:       req.Name,
		Schedule:   req.Schedule,
		ExecType:   req.ExecType,
		ExecConfig: req.ExecConfig,
		TimeoutSec: req.TimeoutSec,
		Tags:       req.Tags,
	})
	if err != nil {
		return htools.CronJob{}, err
	}
	return cronJobFromCron(j), nil
}

func (a *cronClientAdapter) ListJobs(ctx context.Context) ([]htools.CronJob, error) {
	jobs, err := a.client.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]htools.CronJob, len(jobs))
	for i, j := range jobs {
		result[i] = cronJobFromCron(j)
	}
	return result, nil
}

func (a *cronClientAdapter) GetJob(ctx context.Context, id string) (htools.CronJob, error) {
	j, err := a.client.GetJob(ctx, id)
	if err != nil {
		return htools.CronJob{}, err
	}
	return cronJobFromCron(j), nil
}

func (a *cronClientAdapter) UpdateJob(ctx context.Context, id string, req htools.CronUpdateJobRequest) (htools.CronJob, error) {
	j, err := a.client.UpdateJob(ctx, id, cron.UpdateJobRequest{
		Schedule:   req.Schedule,
		ExecConfig: req.ExecConfig,
		Status:     req.Status,
		TimeoutSec: req.TimeoutSec,
		Tags:       req.Tags,
	})
	if err != nil {
		return htools.CronJob{}, err
	}
	return cronJobFromCron(j), nil
}

func (a *cronClientAdapter) DeleteJob(ctx context.Context, id string) error {
	return a.client.DeleteJob(ctx, id)
}

func (a *cronClientAdapter) ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]htools.CronExecution, error) {
	execs, err := a.client.ListExecutions(ctx, jobID, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]htools.CronExecution, len(execs))
	for i, e := range execs {
		result[i] = cronExecFromCron(e)
	}
	return result, nil
}

func (a *cronClientAdapter) Health(ctx context.Context) error {
	return a.client.Health(ctx)
}

func cronJobFromCron(j cron.Job) htools.CronJob {
	return htools.CronJob{
		ID:         j.ID,
		Name:       j.Name,
		Schedule:   j.Schedule,
		ExecType:   j.ExecType,
		ExecConfig: j.ExecConfig,
		Status:     j.Status,
		TimeoutSec: j.TimeoutSec,
		Tags:       j.Tags,
		NextRunAt:  j.NextRunAt,
		LastRunAt:  j.LastRunAt,
		CreatedAt:  j.CreatedAt,
		UpdatedAt:  j.UpdatedAt,
	}
}

func cronExecFromCron(e cron.Execution) htools.CronExecution {
	return htools.CronExecution{
		ID:            e.ID,
		JobID:         e.JobID,
		StartedAt:     e.StartedAt,
		FinishedAt:    e.FinishedAt,
		Status:        e.Status,
		RunID:         e.RunID,
		OutputSummary: e.OutputSummary,
		Error:         e.Error,
		DurationMs:    e.DurationMs,
	}
}

// embeddedCronAdapter implements htools.CronClient by calling cron.Store
// and cron.Scheduler directly, without HTTP.
type embeddedCronAdapter struct {
	store     cron.Store
	scheduler *cron.Scheduler
	clock     cron.Clock
}

func (a *embeddedCronAdapter) CreateJob(ctx context.Context, req htools.CronCreateJobRequest) (htools.CronJob, error) {
	if req.Name == "" {
		return htools.CronJob{}, fmt.Errorf("name is required")
	}
	if req.Schedule == "" {
		return htools.CronJob{}, fmt.Errorf("schedule is required")
	}
	nextRun, err := cron.NextRunTime(req.Schedule, a.clock.Now())
	if err != nil {
		return htools.CronJob{}, fmt.Errorf("invalid schedule: %w", err)
	}
	if req.ExecType != cron.ExecTypeShell && req.ExecType != cron.ExecTypeHarness {
		return htools.CronJob{}, fmt.Errorf("execution_type must be \"shell\" or \"harness\"")
	}
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 30
	}
	now := a.clock.Now()
	job := cron.Job{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Schedule:   req.Schedule,
		ExecType:   req.ExecType,
		ExecConfig: req.ExecConfig,
		Status:     cron.StatusActive,
		TimeoutSec: req.TimeoutSec,
		Tags:       req.Tags,
		NextRunAt:  nextRun,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	job, err = a.store.CreateJob(ctx, job)
	if err != nil {
		return htools.CronJob{}, fmt.Errorf("store: %w", err)
	}
	if addErr := a.scheduler.AddJob(job); addErr != nil {
		return htools.CronJob{}, fmt.Errorf("scheduler: %w", addErr)
	}
	return cronJobFromCron(job), nil
}

func (a *embeddedCronAdapter) ListJobs(ctx context.Context) ([]htools.CronJob, error) {
	jobs, err := a.store.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]htools.CronJob, len(jobs))
	for i, j := range jobs {
		result[i] = cronJobFromCron(j)
	}
	return result, nil
}

func (a *embeddedCronAdapter) GetJob(ctx context.Context, id string) (htools.CronJob, error) {
	job, err := a.store.GetJob(ctx, id)
	if err != nil {
		job, err = a.store.GetJobByName(ctx, id)
		if err != nil {
			return htools.CronJob{}, fmt.Errorf("job not found")
		}
	}
	return cronJobFromCron(job), nil
}

func (a *embeddedCronAdapter) UpdateJob(ctx context.Context, id string, req htools.CronUpdateJobRequest) (htools.CronJob, error) {
	job, err := a.store.GetJob(ctx, id)
	if err != nil {
		return htools.CronJob{}, fmt.Errorf("job not found")
	}

	if req.Schedule != nil {
		trimmed := strings.TrimSpace(*req.Schedule)
		if trimmed == "" {
			return htools.CronJob{}, fmt.Errorf("schedule must not be empty")
		}
		nextRun, err := cron.NextRunTime(*req.Schedule, a.clock.Now())
		if err != nil {
			return htools.CronJob{}, fmt.Errorf("invalid schedule: %w", err)
		}
		job.Schedule = *req.Schedule
		job.NextRunAt = nextRun
	}
	if req.ExecConfig != nil {
		job.ExecConfig = *req.ExecConfig
	}
	if req.TimeoutSec != nil {
		job.TimeoutSec = *req.TimeoutSec
	}
	if req.Tags != nil {
		job.Tags = *req.Tags
	}

	if req.Status != nil {
		if *req.Status != cron.StatusActive && *req.Status != cron.StatusPaused {
			return htools.CronJob{}, fmt.Errorf("status must be \"active\" or \"paused\"")
		}
		oldStatus := job.Status
		job.Status = *req.Status

		if *req.Status == cron.StatusPaused && oldStatus != cron.StatusPaused {
			a.scheduler.RemoveJob(job.ID)
		}
		if *req.Status == cron.StatusActive && oldStatus != cron.StatusActive {
			if addErr := a.scheduler.AddJob(job); addErr != nil {
				return htools.CronJob{}, fmt.Errorf("scheduler: %w", addErr)
			}
		}
	}

	if req.Schedule != nil && (req.Status == nil || *req.Status == cron.StatusActive) {
		if err := a.scheduler.UpdateJobSchedule(job); err != nil {
			return htools.CronJob{}, fmt.Errorf("scheduler: %w", err)
		}
	}

	job.UpdatedAt = a.clock.Now()
	if err := a.store.UpdateJob(ctx, job); err != nil {
		return htools.CronJob{}, fmt.Errorf("store: %w", err)
	}
	return cronJobFromCron(job), nil
}

func (a *embeddedCronAdapter) DeleteJob(ctx context.Context, id string) error {
	if err := a.store.DeleteJob(ctx, id); err != nil {
		return err
	}
	a.scheduler.RemoveJob(id)
	return nil
}

func (a *embeddedCronAdapter) ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]htools.CronExecution, error) {
	execs, err := a.store.ListExecutions(ctx, jobID, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]htools.CronExecution, len(execs))
	for i, e := range execs {
		result[i] = cronExecFromCron(e)
	}
	return result, nil
}

func (a *embeddedCronAdapter) Health(_ context.Context) error {
	return nil
}
