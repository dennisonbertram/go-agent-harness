package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-agent-harness/internal/checkpoints"
	"go-agent-harness/internal/config"
	"go-agent-harness/internal/cron"
	githubadapter "go-agent-harness/internal/github"
	"go-agent-harness/internal/harness"
	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/deferred"
	"go-agent-harness/internal/hooks"
	linearadapter "go-agent-harness/internal/linear"
	"go-agent-harness/internal/modelstore"
	"go-agent-harness/internal/networks"
	"go-agent-harness/internal/provider"
	"go-agent-harness/internal/provider/anthropic"
	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/provider/codex"
	"go-agent-harness/internal/provider/kimi"
	openai "go-agent-harness/internal/provider/openai"
	"go-agent-harness/internal/provider/pricing"
	"go-agent-harness/internal/provider/tokencache"
	"go-agent-harness/internal/providercatalog"
	"go-agent-harness/internal/relay"
	"go-agent-harness/internal/server"
	slackadapter "go-agent-harness/internal/slack"
	istore "go-agent-harness/internal/store"
	"go-agent-harness/internal/subagents"
	"go-agent-harness/internal/trigger"
	scriptworkflow "go-agent-harness/internal/workflow"
	"go-agent-harness/internal/workflows"
)

type catalogBootstrapOptions struct {
	workspace    string
	getenv       func(string) string
	newProvider  providerFactory
	logger       func(string, ...any)
	codexStore   *codex.Store
	codexRefresh tokencache.RefreshFunc
}

type catalogBootstrap struct {
	modelCatalog *catalog.Catalog
	// providerFiles is what catalog/providers/ declared. Client construction
	// and discovery read it so a provider's behaviour follows what its file
	// says rather than a hardcoded list of provider names.
	providerFiles         *providercatalog.Catalog
	providerRegistry      *catalog.ProviderRegistry
	pricingResolver       pricing.Resolver
	lookupModelAPI        func(providerName, modelID string) string
	lookupModelModalities func(providerName, modelID string) []string
}

func buildCatalogBootstrap(opts catalogBootstrapOptions) (catalogBootstrap, error) {
	if opts.getenv == nil {
		opts.getenv = os.Getenv
	}
	if opts.logger == nil {
		opts.logger = func(string, ...any) {}
	}

	pricingCatalogPath := strings.TrimSpace(opts.getenv("HARNESS_PRICING_CATALOG_PATH"))
	modelCatalogPath := strings.TrimSpace(opts.getenv("HARNESS_MODEL_CATALOG_PATH"))
	if modelCatalogPath == "" {
		candidates := []string{
			filepath.Join(opts.workspace, "catalog", "models.json"),
			"catalog/models.json",
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				modelCatalogPath = candidate
				break
			}
		}
	}

	var bootstrap catalogBootstrap
	if modelCatalogPath != "" {
		cat, err := catalog.LoadCatalog(modelCatalogPath)
		if err != nil {
			opts.logger("warning: failed to load model catalog from %s: %v (continuing without catalog)", modelCatalogPath, err)
		} else {
			bootstrap.modelCatalog = cat
			bootstrap.providerRegistry = catalog.NewProviderRegistryWithEnv(cat, opts.getenv)
			opts.logger("loaded model catalog with %d providers", len(cat.Providers))
		}
	}

	// Fold in the per-provider files. They are the source of truth for
	// endpoints, capabilities and dated pricing; the bundled catalog keeps
	// everything it already curated. The merge is additive, so this can only
	// add providers and fill gaps — and a missing directory is not fatal.
	//
	// This runs before the pricing resolver is built below, so rates from the
	// provider files reach cost reporting without any further wiring.
	if dir := providerCatalogDir(opts); dir != "" {
		pc, err := providercatalog.Load(dir)
		if err != nil {
			// Deliberately fatal. One bad file makes Load reject the whole
			// directory, so continuing would silently drop every provider it
			// describes — the picker would just stop offering them with
			// nothing to say why. A startup error naming the file is far
			// easier to act on.
			return catalogBootstrap{}, fmt.Errorf("load provider catalog from %s: %w", dir, err)
		} else {
			if bootstrap.modelCatalog == nil {
				bootstrap.modelCatalog = pc.ToModelCatalog()
				bootstrap.providerRegistry = catalog.NewProviderRegistryWithEnv(bootstrap.modelCatalog, opts.getenv)
			}
			bootstrap.providerFiles = pc
			addedProviders, addedModels := pc.MergeInto(bootstrap.modelCatalog)
			opts.logger("provider catalog: %d files, added %d providers and %d models",
				len(pc.Providers), len(addedProviders), len(addedModels))
			// A price nobody has checked in months is worse than a missing one,
			// because it still looks authoritative. Say so at startup.
			if stale := pc.StaleProviders(time.Now(), 90*24*time.Hour); len(stale) > 0 {
				opts.logger("warning: pricing not re-checked in over 90 days for: %s",
					strings.Join(stale, ", "))
			}
			if unpriced := pc.UnpricedInUSD(); len(unpriced) > 0 {
				opts.logger("note: rates not expressible in USD per million tokens, so excluded from cost totals: %s",
					strings.Join(unpriced, ", "))
			}
		}
	}

	bootstrap.lookupModelAPI = func(providerName, modelID string) string {
		if bootstrap.modelCatalog == nil {
			return ""
		}
		entry, ok := bootstrap.modelCatalog.Providers[providerName]
		if !ok {
			return ""
		}
		resolved := modelID
		if target, ok := entry.Aliases[modelID]; ok {
			if _, exists := entry.Models[target]; exists {
				resolved = target
			}
		}
		model, ok := entry.Models[resolved]
		if !ok {
			return ""
		}
		return model.API
	}

	// lookupModelModalities mirrors lookupModelAPI for the modalities list so
	// provider clients can refuse image input for catalog-known text-only
	// models (epic #818 slice 4). Nil catalog / unknown model → nil (the
	// client's refusal check is skipped).
	bootstrap.lookupModelModalities = func(providerName, modelID string) []string {
		if bootstrap.modelCatalog == nil {
			return nil
		}
		entry, ok := bootstrap.modelCatalog.Providers[providerName]
		if !ok {
			return nil
		}
		resolved := modelID
		if target, ok := entry.Aliases[modelID]; ok {
			if _, exists := entry.Models[target]; exists {
				resolved = target
			}
		}
		model, ok := entry.Models[resolved]
		if !ok {
			return nil
		}
		return model.Modalities
	}

	if pricingCatalogPath != "" {
		resolver, err := pricing.NewFileResolver(pricingCatalogPath)
		if err != nil {
			return catalogBootstrap{}, fmt.Errorf("load pricing catalog from %s: %w", pricingCatalogPath, err)
		}
		// An explicit pricing file used to replace the provider files outright,
		// so a provider present only in catalog/providers/ showed a rate in the
		// picker and then resolved to no rate at all when the cost was totalled.
		// The explicit file still wins every model it names; the provider files
		// only fill what it does not cover.
		if bootstrap.providerFiles != nil {
			bootstrap.pricingResolver = pricing.NewFallbackResolver(
				resolver, pricing.NewResolverFromCatalog(bootstrap.providerFiles.ToPricingCatalog()))
		} else {
			bootstrap.pricingResolver = resolver
		}
	} else if bootstrap.modelCatalog != nil {
		// No explicit pricing file — fall back to the pricing blocks embedded in
		// the model catalog itself (catalog/models.json already has Anthropic and
		// OpenAI rates). This ensures cost reporting works out of the box without
		// requiring HARNESS_PRICING_CATALOG_PATH to be set.
		bootstrap.pricingResolver = catalog.NewCatalogPricingResolver(bootstrap.modelCatalog)
		opts.logger("pricing resolver wired from model catalog (fallback)")
	}

	if bootstrap.providerRegistry != nil {
		// Kimi's own CLI owns the refresh; the harness reads the current token
		// rather than trying to renew a copy it cannot renew. See
		// internal/provider/kimi/vendor_source.go for why.
		bootstrap.providerRegistry.SetTokenSource(
			"kimi-subscription", kimi.NewVendorTokenSource("").WithRefresher(kimi.NewCLIRefresher()))
		store := opts.codexStore
		if store == nil {
			store = codex.DefaultStore()
		}
		refresh := opts.codexRefresh
		if refresh == nil {
			refresh = codex.NewRefreshFunc(nil, "", nil)
		}
		var codexSource *codex.Source
		if source, err := codex.NewTokenSource(store, refresh); err == nil {
			codexSource = source
			bootstrap.providerRegistry.SetTokenSource("codex-subscription", source)
		} else if !errors.Is(err, codex.ErrNotConfigured) {
			return catalogBootstrap{}, fmt.Errorf("load Codex subscription credential: %w", err)
		}
		registerModelDiscoverers(bootstrap.providerRegistry, bootstrap.providerFiles)
		bootstrap.providerRegistry.SetClientFactory(catalog.ClientFactory(func(apiKey, baseURL, providerName string, tokenSource provider.TokenSource) (catalog.ProviderClient, error) {
			// Which wire protocol to speak is a property of the provider, not
			// of its name. Reading it from the catalog lets a provider file
			// declare protocol "anthropic" and actually get an Anthropic
			// client; before this, only the provider literally named
			// "anthropic" did, and any other one was silently sent
			// OpenAI-shaped requests that it rejected.
			protocol := providerName == "anthropic"
			if entry, ok := bootstrap.modelCatalog.Providers[providerName]; ok && entry.Protocol != "" {
				protocol = entry.Protocol == providercatalog.ProtocolAnthropic
			}
			if protocol {
				return anthropic.NewClient(anthropic.Config{
					APIKey:          apiKey,
					BaseURL:         baseURL,
					ProviderName:    providerName,
					PricingResolver: bootstrap.pricingResolver,
					// Catalog lets maxTokensForModel resolve each model's real
					// max_output_tokens (e.g. 16384) instead of silently
					// falling back to the package's 4096-token default.
					Catalog: bootstrap.modelCatalog,
				})
			}
			// Look up provider quirks from the static catalog so that features
			// like "reasoning_content_passback" are honoured without hardcoding
			// provider names in the factory.
			var providerQuirks []string
			if entry, ok := bootstrap.modelCatalog.Providers[providerName]; ok {
				providerQuirks = entry.Quirks
			}
			cfg := openai.Config{
				APIKey:              apiKey,
				TokenSource:         tokenSource,
				BaseURL:             baseURL,
				ProviderName:        providerName,
				PricingResolver:     bootstrap.pricingResolver,
				ModelAPILookup:      bootstrap.lookupModelAPI,
				ModelModalityLookup: bootstrap.lookupModelModalities,
				NoParallelTools:     providerName == "gemini",
				ModelIDPrefix: func() string {
					if providerName == "gemini" {
						return "models/"
					}
					return ""
				}(),
				Quirks: providerQuirks,
			}
			// Headers a provider file declares are part of how that provider
			// authenticates. Applied first so the specific cases below can
			// still override, and copied rather than aliased so one provider's
			// map cannot be mutated through another's config.
			if bootstrap.providerFiles != nil {
				if p, ok := bootstrap.providerFiles.Get(providerName); ok && len(p.Auth.ExtraHeaders) > 0 {
					headers := make(map[string]string, len(p.Auth.ExtraHeaders))
					for k, v := range p.Auth.ExtraHeaders {
						headers[k] = v
					}
					cfg.ExtraHeaders = headers
				}
			}
			if providerName == "kimi-subscription" {
				cfg.ExtraHeaders = kimi.ExtraHeaders()
			}
			if providerName == "openrouter" {
				referer := os.Getenv("HARNESS_OPENROUTER_REFERER")
				if referer == "" {
					referer = "https://github.com/dennisonbertram/go-agent-harness"
				}
				title := os.Getenv("HARNESS_OPENROUTER_TITLE")
				if title == "" {
					title = "go-agent-harness"
				}
				cfg.OpenRouterReferer = referer
				cfg.OpenRouterTitle = title
			}
			if providerName == "codex-subscription" && codexSource != nil {
				cfg.SkipV1Path = true
				cfg.ExtraHeaders = map[string]string{"chatgpt-account-id": codexSource.AccountID()}
			}
			return opts.newProvider(cfg)
		}))
	}

	return bootstrap, nil
}

// registerModelDiscoverers wires live model discovery.
//
// Four providers need a bespoke discoverer because their listing does not look
// like everyone else's. Every other provider that declares a models endpoint in
// its file gets the generic OpenAI-compatible one, so adding a provider no
// longer means editing this list — which is why a configured provider's newly
// released models were invisible until someone did.
func registerModelDiscoverers(registry *catalog.ProviderRegistry, files *providercatalog.Catalog) {
	bespoke := map[string]func(apiKey, baseURL, providerName string) catalog.ModelDiscoverer{
		"openrouter": func(_, _, _ string) catalog.ModelDiscoverer {
			return catalog.NewDiscovery(catalog.DiscoveryOptions{TTL: 5 * time.Minute})
		},
		"openai": func(apiKey, baseURL, _ string) catalog.ModelDiscoverer {
			return openai.NewModelDiscovery(openai.Config{APIKey: apiKey, BaseURL: baseURL})
		},
		"anthropic": func(apiKey, baseURL, _ string) catalog.ModelDiscoverer {
			return anthropic.NewModelDiscovery(anthropic.Config{APIKey: apiKey, BaseURL: baseURL})
		},
		"deepseek": func(apiKey, baseURL, name string) catalog.ModelDiscoverer {
			return openai.NewDeepSeekModelDiscovery(openai.Config{APIKey: apiKey, BaseURL: baseURL, ProviderName: name})
		},
	}

	names := make([]string, 0, len(bespoke))
	for name := range bespoke {
		names = append(names, name)
	}
	if files != nil {
		for _, id := range files.IDs() {
			if _, already := bespoke[id]; already {
				continue
			}
			p, _ := files.Get(id)
			// Only an OpenAI-compatible provider that says it publishes a
			// listing. Anything else would be asked a question it cannot
			// answer, and a failed discovery is charged a request every time.
			if !p.Usable() || p.Capabilities.ModelsEndpoint == "" ||
				p.Protocol != providercatalog.ProtocolOpenAICompat {
				continue
			}
			names = append(names, id)
		}
	}
	sort.Strings(names)

	for _, providerName := range names {
		if !registry.IsConfigured(providerName) {
			continue
		}
		apiKey, baseURL, ok := registry.DiscoveryCredentials(providerName)
		if !ok {
			continue
		}
		if build, ok := bespoke[providerName]; ok {
			registry.SetDiscovery(providerName, build(apiKey, baseURL, providerName))
			continue
		}
		registry.SetDiscovery(providerName, openai.NewModelDiscovery(
			openai.Config{APIKey: apiKey, BaseURL: baseURL, ProviderName: providerName}))
	}
}

type cronBootstrap struct {
	client    htools.CronClient
	store     cron.Store
	scheduler *cron.Scheduler
}

func cronSchedulerConfig(resolved config.CronConfig) cron.SchedulerConfig {
	return cron.SchedulerConfig{
		MaxConcurrent: 5,
		Jitter: cron.JitterConfig{
			Enabled:          resolved.JitterEnabled,
			MinSec:           resolved.JitterMinSec,
			MaxSec:           resolved.JitterMaxSec,
			AvoidMarks:       append([]int(nil), resolved.AvoidMinuteMarks...),
			LogJitteredTimes: resolved.LogJitteredTimes,
		},
	}
}

func buildCronBootstrap(
	workspace,
	cronURL string,
	resolved config.CronConfig,
	logger func(string, ...any),
	harnessStarter cron.RunStarter,
) (cronBootstrap, error) {
	if logger == nil {
		logger = func(string, ...any) {}
	}
	if strings.TrimSpace(cronURL) != "" {
		return cronBootstrap{
			client: &cronClientAdapter{client: cron.NewClient(strings.TrimSpace(cronURL))},
		}, nil
	}

	cronDBPath := filepath.Join(workspace, ".harness", "cron.db")
	store, err := cron.NewSQLiteStore(cronDBPath)
	if err != nil {
		return cronBootstrap{}, fmt.Errorf("create cron store: %w", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		store.Close()
		return cronBootstrap{}, fmt.Errorf("migrate cron store: %w", err)
	}
	clock := cron.RealClock{}
	// Route by declared execution type. Handing every job to the shell
	// executor meant a harness job could never succeed, however well formed.
	executor := &cron.DispatchExecutor{
		Shell:   &cron.ShellExecutor{},
		Harness: &cron.HarnessExecutor{Starter: harnessStarter},
	}
	scheduler := cron.NewScheduler(store, executor, clock, cronSchedulerConfig(resolved))
	if err := scheduler.Start(context.Background()); err != nil {
		store.Close()
		return cronBootstrap{}, fmt.Errorf("start cron scheduler: %w", err)
	}
	logger("embedded cron scheduler started (db: %s)", cronDBPath)
	return cronBootstrap{
		client:    &embeddedCronAdapter{store: store, scheduler: scheduler, clock: clock},
		store:     store,
		scheduler: scheduler,
	}, nil
}

type persistenceBootstrapOptions struct {
	workspace         string
	getenv            func(string) string
	convRetentionDays int
	logger            func(string, ...any)
	newCleaner        func(store harness.ConversationStore, retentionDays int) conversationCleanerStarter
}

type persistenceBootstrap struct {
	runStore          istore.Store
	conversationStore harness.ConversationStore
	relayWorkerStore  relay.WorkerStore
	relayControl      *relay.ControlPlane
	convCleanerCancel context.CancelFunc
}

func buildPersistenceBootstrap(opts persistenceBootstrapOptions) (_ persistenceBootstrap, err error) {
	if opts.getenv == nil {
		opts.getenv = os.Getenv
	}
	if opts.logger == nil {
		opts.logger = func(string, ...any) {}
	}
	if opts.newCleaner == nil {
		opts.newCleaner = func(store harness.ConversationStore, retentionDays int) conversationCleanerStarter {
			return harness.NewConversationCleaner(store, retentionDays)
		}
	}

	var bootstrap persistenceBootstrap
	defer func() {
		if err == nil {
			return
		}
		if bootstrap.convCleanerCancel != nil {
			bootstrap.convCleanerCancel()
		}
		if bootstrap.conversationStore != nil {
			_ = bootstrap.conversationStore.Close()
		}
		if bootstrap.relayWorkerStore != nil {
			_ = bootstrap.relayWorkerStore.Close()
		}
		if bootstrap.runStore != nil {
			_ = bootstrap.runStore.Close()
		}
	}()

	if runDBPath := strings.TrimSpace(opts.getenv("HARNESS_RUN_DB")); runDBPath != "" {
		if !filepath.IsAbs(runDBPath) {
			runDBPath = filepath.Join(opts.workspace, runDBPath)
		}
		runStore, openErr := istore.NewSQLiteStore(runDBPath)
		if openErr != nil {
			err = fmt.Errorf("create run store: %w", openErr)
			return persistenceBootstrap{}, err
		}
		if migrateErr := runStore.Migrate(context.Background()); migrateErr != nil {
			_ = runStore.Close()
			err = fmt.Errorf("migrate run store: %w", migrateErr)
			return persistenceBootstrap{}, err
		}
		bootstrap.runStore = runStore
		opts.logger("run persistence enabled: %s", runDBPath)
	}

	if dbPath := strings.TrimSpace(opts.getenv("HARNESS_CONVERSATION_DB")); dbPath != "" {
		if !filepath.IsAbs(dbPath) {
			dbPath = filepath.Join(opts.workspace, dbPath)
		}
		convStore, openErr := harness.NewSQLiteConversationStore(dbPath)
		if openErr != nil {
			err = fmt.Errorf("create conversation store: %w", openErr)
			return persistenceBootstrap{}, err
		}
		if migrateErr := convStore.Migrate(context.Background()); migrateErr != nil {
			_ = convStore.Close()
			err = fmt.Errorf("migrate conversation store: %w", migrateErr)
			return persistenceBootstrap{}, err
		}
		bootstrap.conversationStore = convStore
		opts.logger("conversation persistence enabled: %s", dbPath)

		if opts.convRetentionDays > 0 {
			opts.logger("conversation retention policy: %d days", opts.convRetentionDays)
			cleanerCtx, cleanerCancel := context.WithCancel(context.Background())
			opts.newCleaner(convStore, opts.convRetentionDays).Start(cleanerCtx, 24*time.Hour)
			bootstrap.convCleanerCancel = cleanerCancel
		}
	}

	if relayDBPath := strings.TrimSpace(opts.getenv("HARNESS_RELAY_DB")); relayDBPath != "" {
		if !filepath.IsAbs(relayDBPath) {
			relayDBPath = filepath.Join(opts.workspace, relayDBPath)
		}
		relayStore, openErr := relay.NewSQLiteWorkerStore(relayDBPath)
		if openErr != nil {
			err = fmt.Errorf("create relay worker store: %w", openErr)
			return persistenceBootstrap{}, err
		}
		if migrateErr := relayStore.Migrate(context.Background()); migrateErr != nil {
			_ = relayStore.Close()
			err = fmt.Errorf("migrate relay worker store: %w", migrateErr)
			return persistenceBootstrap{}, err
		}
		bootstrap.relayWorkerStore = relayStore

		// Build the self-contained control plane (capability + event stores,
		// placement router, composer, policy, operator views) over the same DB.
		control, controlErr := relay.NewControlPlane(context.Background(), relayStore)
		if controlErr != nil {
			err = fmt.Errorf("build relay control plane: %w", controlErr)
			return persistenceBootstrap{}, err
		}
		bootstrap.relayControl = control
		opts.logger("relay worker persistence + control plane enabled: %s", relayDBPath)
	}

	return bootstrap, nil
}

type triggerRuntime struct {
	validators *trigger.ValidatorRegistry
	github     *githubadapter.GitHubAdapter
	slack      *slackadapter.SlackAdapter
	linear     *linearadapter.LinearAdapter
}

func buildTriggerRuntime(getenv func(string) string, logger func(string, ...any)) triggerRuntime {
	if getenv == nil {
		getenv = os.Getenv
	}
	if logger == nil {
		logger = func(string, ...any) {}
	}

	runtime := triggerRuntime{
		validators: trigger.NewValidatorRegistry(),
	}
	if secret := strings.TrimSpace(getenv("GITHUB_WEBHOOK_SECRET")); secret != "" {
		runtime.validators.Register("github", &trigger.GitHubValidator{Secret: secret})
		runtime.github = githubadapter.NewGitHubAdapter(secret)
		logger("registered GitHub webhook validator")
		logger("registered GitHub webhook adapter for /v1/webhooks/github")
	}
	if secret := strings.TrimSpace(getenv("SLACK_SIGNING_SECRET")); secret != "" {
		runtime.validators.Register("slack", &trigger.SlackValidator{Secret: secret})
		runtime.slack = slackadapter.NewSlackAdapter()
		logger("registered Slack webhook validator")
		logger("registered Slack webhook adapter for /v1/webhooks/slack")
	}
	if secret := strings.TrimSpace(getenv("LINEAR_WEBHOOK_SECRET")); secret != "" {
		runtime.validators.Register("linear", &trigger.LinearValidator{Secret: secret})
		runtime.linear = linearadapter.NewLinearAdapter()
		logger("registered Linear webhook validator")
		logger("registered Linear webhook adapter for /v1/webhooks/linear")
	}
	return runtime
}

type serverBootstrapOptions struct {
	runner           *harness.Runner
	modelCatalog     *catalog.Catalog
	skillLister      htools.SkillLister
	skillManager     server.SkillManager
	cronClient       htools.CronClient
	subagentManager  subagents.Manager
	checkpoints      *checkpoints.Service
	workflows        *workflows.Engine
	scriptWorkflows  scriptworkflow.SourceService
	networks         *networks.Engine
	providerRegistry *catalog.ProviderRegistry
	runStore         istore.Store
	relayWorkerStore relay.WorkerStore
	relayControl     *relay.ControlPlane
	tools            *harness.Registry
	todos            deferred.TodoManager
	triggers         triggerRuntime
	rolloutDir       string
	hooksSummary     hooks.Summary
	callbackMgr      *htools.CallbackManager
	jobTracker       *harness.JobTracker
	configReloader   *configReloader
	modelSettings    *modelstore.Service
}

func buildServerOptions(opts serverBootstrapOptions) server.ServerOptions {
	// A model that lives only in the store must still be resolvable by name,
	// or the picker offers models that cannot be run.
	registerStoreModels(opts.modelSettings, opts.providerRegistry, opts.modelCatalog)
	// And a credential the store can resolve has to reach the registry, which
	// is what builds the client. Otherwise a key saved through the settings UI
	// sits in the Keychain while the run reports the environment variable is
	// unset — the credential is present and unusable at the same time.
	applyStoreCredentials(context.Background(), opts.modelSettings, opts.providerRegistry)
	return server.ServerOptions{
		Runner:           opts.runner,
		Catalog:          opts.modelCatalog,
		AgentRunner:      opts.runner,
		SkillLister:      opts.skillLister,
		Skills:           opts.skillManager,
		CronClient:       opts.cronClient,
		SubagentManager:  opts.subagentManager,
		Checkpoints:      opts.checkpoints,
		Workflows:        opts.workflows,
		ScriptWorkflows:  opts.scriptWorkflows,
		Networks:         opts.networks,
		ProviderRegistry: opts.providerRegistry,
		ModelSettings:    opts.modelSettings,
		Store:            opts.runStore,
		RelayWorkerStore: opts.relayWorkerStore,
		RelayControl:     opts.relayControl,
		Tools:            opts.tools,
		Todos:            opts.todos,
		Validators:       opts.triggers.validators,
		GitHubAdapter:    opts.triggers.github,
		SlackAdapter:     opts.triggers.slack,
		LinearAdapter:    opts.triggers.linear,
		RolloutDir:       opts.rolloutDir,
		HooksSummary:     opts.hooksSummary,
		CallbackLister:   opts.callbackMgr,
		CallbackCanceler: opts.callbackMgr,
		JobTracker:       opts.jobTracker,
		ConfigReload:     configReloadFunc(opts.configReloader),
	}
}

// configReloadFunc adapts a configReloader to server.ConfigReloadFunc,
// returning nil (endpoint disabled) when no reloader is wired.
func configReloadFunc(reloader *configReloader) server.ConfigReloadFunc {
	if reloader == nil {
		return nil
	}
	return reloader.reload
}

// providerCatalogDir locates the per-provider catalog files.
//
// Mirrors how the model catalog is found: an explicit override first, then the
// workspace, then the working directory, so a checkout and an installation
// both work without configuration.
func providerCatalogDir(opts catalogBootstrapOptions) string {
	if dir := strings.TrimSpace(opts.getenv("HARNESS_PROVIDER_CATALOG_DIR")); dir != "" {
		return dir
	}
	for _, candidate := range []string{
		filepath.Join(opts.workspace, "catalog", "providers"),
		filepath.Join("catalog", "providers"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}
