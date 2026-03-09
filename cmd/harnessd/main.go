package main

import (
	"context"
	"errors"
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

	"go-agent-harness/internal/harness"
	htools "go-agent-harness/internal/harness/tools"
	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
	openai "go-agent-harness/internal/provider/openai"
	"go-agent-harness/internal/provider/pricing"
	"go-agent-harness/internal/server"
	"go-agent-harness/internal/skills"
	"go-agent-harness/internal/systemprompt"
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

type providerFactory func(config openai.Config) (harness.Provider, error)

var (
	runMain            = run
	exitFunc           = os.Exit
	runWithSignalsFunc = runWithSignals
)

func main() {
	if err := runMain(); err != nil {
		log.Printf("fatal: %v", err)
		exitFunc(1)
	}
}

func run() error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	return runWithSignalsFunc(sig, os.Getenv, func(config openai.Config) (harness.Provider, error) {
		return openai.NewClient(config)
	})
}

func runWithSignals(sig <-chan os.Signal, getenv func(string) string, newProvider providerFactory) error {
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

	workspace := envOrDefault("HARNESS_WORKSPACE", ".")
	model := envOrDefault("HARNESS_MODEL", "gpt-4.1-mini")
	addr := envOrDefault("HARNESS_ADDR", ":8080")
	systemPrompt := envOrDefault("HARNESS_SYSTEM_PROMPT", "You are a practical coding assistant. Prefer using tools for file inspection and tests when needed.")
	defaultAgentIntent := envOrDefault("HARNESS_DEFAULT_AGENT_INTENT", "general")
	promptsDir := strings.TrimSpace(envOrDefault("HARNESS_PROMPTS_DIR", findDefaultPromptsDir()))
	maxSteps := envIntOrDefault("HARNESS_MAX_STEPS", 8)
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
	skillsEnabled := envBoolOrDefault("HARNESS_SKILLS_ENABLED", true)
	callbacksEnabled := envBoolOrDefault("HARNESS_ENABLE_CALLBACKS", true)

	var providerRegistry *catalog.ProviderRegistry
	if modelCatalogPath != "" {
		cat, err := catalog.LoadCatalog(modelCatalogPath)
		if err != nil {
			log.Printf("warning: failed to load model catalog from %s: %v (continuing without catalog)", modelCatalogPath, err)
		} else {
			providerRegistry = catalog.NewProviderRegistryWithEnv(cat, getenv)
			log.Printf("loaded model catalog with %d providers", len(cat.Providers))
		}
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
			return newProvider(openai.Config{
				APIKey:          apiKey,
				BaseURL:         baseURL,
				ProviderName:    providerName,
				PricingResolver: pricingResolver,
			})
		})
	}

	provider, err := newProvider(openai.Config{
		APIKey:          apiKey,
		BaseURL:         getenv("OPENAI_BASE_URL"),
		Model:           model,
		PricingResolver: pricingResolver,
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
	home, _ := os.UserHomeDir()
	globalDir := envOrDefault("HARNESS_GLOBAL_DIR", filepath.Join(home, ".go-harness"))
	var skillLister htools.SkillLister
	if skillsEnabled {
		loader := skills.NewLoader(skills.LoaderConfig{
			GlobalDir:    filepath.Join(globalDir, "skills"),
			WorkspaceDir: filepath.Join(workspace, ".go-harness", "skills"),
		})
		skillRegistry := skills.NewRegistry()
		if err := skillRegistry.Load(loader); err != nil {
			log.Printf("warning: failed to load skills: %v (continuing without skills)", err)
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

	// Delayed callbacks
	var callbackStarter *callbackRunStarter
	var callbackMgr *htools.CallbackManager
	if callbacksEnabled {
		callbackStarter = &callbackRunStarter{}
		callbackMgr = htools.NewCallbackManager(callbackStarter)
		log.Printf("delayed callbacks enabled")
	}

	askUserBroker := harness.NewInMemoryAskUserQuestionBroker(time.Now)
	tools := harness.NewDefaultRegistryWithOptions(workspace, harness.DefaultRegistryOptions{
		ApprovalMode:    approvalMode,
		Policy:          nil,
		AskUserBroker:   askUserBroker,
		AskUserTimeout:  time.Duration(askUserTimeoutSeconds) * time.Second,
		MemoryManager:   memoryManager,
		SkillLister:     skillLister,
		CallbackManager: callbackMgr,
	})
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
	})

	// Wire the runner into the callback adapter now that it exists
	if callbackStarter != nil {
		callbackStarter.mu.Lock()
		callbackStarter.runner = runner
		callbackStarter.mu.Unlock()
	}

	handler := server.New(runner)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
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
		}
	}
	return result
}

func (a *skillListerAdapter) ResolveSkill(name, args, workspace string) (string, error) {
	ws := workspace
	if ws == "" {
		ws = a.workspace
	}
	return a.resolver.ResolveSkill(name, args, ws)
}
