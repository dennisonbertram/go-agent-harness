package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-agent-harness/internal/cron"
	githubadapter "go-agent-harness/internal/github"
	"go-agent-harness/internal/harness"
	htools "go-agent-harness/internal/harness/tools"
	linearadapter "go-agent-harness/internal/linear"
	"go-agent-harness/internal/provider/anthropic"
	"go-agent-harness/internal/provider/catalog"
	openai "go-agent-harness/internal/provider/openai"
	"go-agent-harness/internal/provider/pricing"
	"go-agent-harness/internal/server"
	slackadapter "go-agent-harness/internal/slack"
	istore "go-agent-harness/internal/store"
	"go-agent-harness/internal/subagents"
	"go-agent-harness/internal/trigger"
)

type catalogBootstrapOptions struct {
	workspace   string
	getenv      func(string) string
	newProvider providerFactory
	logger      func(string, ...any)
}

type catalogBootstrap struct {
	modelCatalog     *catalog.Catalog
	providerRegistry *catalog.ProviderRegistry
	pricingResolver  pricing.Resolver
	lookupModelAPI   func(providerName, modelID string) string
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
		for _, p := range candidates {
			if _, statErr := os.Stat(p); statErr == nil {
				modelCatalogPath = p
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
		m, ok := entry.Models[resolved]
		if !ok {
			return ""
		}
		return m.API
	}

	if pricingCatalogPath != "" {
		resolver, err := pricing.NewFileResolver(pricingCatalogPath)
		if err != nil {
			return catalogBootstrap{}, fmt.Errorf("load pricing catalog from %s: %w", pricingCatalogPath, err)
		}
		bootstrap.pricingResolver = resolver
	}

	if bootstrap.providerRegistry != nil {
		if _, ok := bootstrap.modelCatalog.Providers["openrouter"]; ok {
			bootstrap.providerRegistry.SetOpenRouterDiscovery(catalog.NewOpenRouterDiscovery(catalog.OpenRouterDiscoveryOptions{
				TTL: 5 * time.Minute,
			}))
		}
		bootstrap.providerRegistry.SetClientFactory(func(apiKey, baseURL, providerName string) (catalog.ProviderClient, error) {
			if providerName == "anthropic" {
				return anthropic.NewClient(anthropic.Config{
					APIKey:          apiKey,
					BaseURL:         baseURL,
					ProviderName:    providerName,
					PricingResolver: bootstrap.pricingResolver,
				})
			}
			return opts.newProvider(openai.Config{
				APIKey:          apiKey,
				BaseURL:         baseURL,
				ProviderName:    providerName,
				PricingResolver: bootstrap.pricingResolver,
				ModelAPILookup:  bootstrap.lookupModelAPI,
				NoParallelTools: providerName == "gemini",
				ModelIDPrefix: func() string {
					if providerName == "gemini" {
						return "models/"
					}
					return ""
				}(),
			})
		})
	}

	return bootstrap, nil
}

type cronBootstrap struct {
	client    htools.CronClient
	store     cron.Store
	scheduler *cron.Scheduler
}

func buildCronBootstrap(workspace, cronURL string, logger func(string, ...any)) (cronBootstrap, error) {
	if logger == nil {
		logger = func(string, ...any) {}
	}
	if strings.TrimSpace(cronURL) != "" {
		return cronBootstrap{
			client: &cronClientAdapter{client: cron.NewClient(strings.TrimSpace(cronURL))},
		}, nil
	}

	cronDBPath := filepath.Join(workspace, ".harness", "cron.db")
	st, err := cron.NewSQLiteStore(cronDBPath)
	if err != nil {
		return cronBootstrap{}, fmt.Errorf("create cron store: %w", err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		st.Close()
		return cronBootstrap{}, fmt.Errorf("migrate cron store: %w", err)
	}
	clock := cron.RealClock{}
	sched := cron.NewScheduler(st, &cron.ShellExecutor{}, clock, cron.SchedulerConfig{MaxConcurrent: 5})
	if err := sched.Start(context.Background()); err != nil {
		st.Close()
		return cronBootstrap{}, fmt.Errorf("start cron scheduler: %w", err)
	}
	logger("embedded cron scheduler started (db: %s)", cronDBPath)
	return cronBootstrap{
		client:    &embeddedCronAdapter{store: st, scheduler: sched, clock: clock},
		store:     st,
		scheduler: sched,
	}, nil
}

type persistenceBootstrapOptions struct {
	workspace         string
	getenv            func(string) string
	convRetentionDays int
	logger            func(string, ...any)
}

type persistenceBootstrap struct {
	runStore          istore.Store
	conversationStore harness.ConversationStore
	convCleanerCancel context.CancelFunc
}

func buildPersistenceBootstrap(opts persistenceBootstrapOptions) (_ persistenceBootstrap, err error) {
	if opts.getenv == nil {
		opts.getenv = os.Getenv
	}
	if opts.logger == nil {
		opts.logger = func(string, ...any) {}
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
		if bootstrap.runStore != nil {
			_ = bootstrap.runStore.Close()
		}
	}()

	if runDBPath := strings.TrimSpace(opts.getenv("HARNESS_RUN_DB")); runDBPath != "" {
		if !filepath.IsAbs(runDBPath) {
			runDBPath = filepath.Join(opts.workspace, runDBPath)
		}
		rs, openErr := istore.NewSQLiteStore(runDBPath)
		if openErr != nil {
			err = fmt.Errorf("create run store: %w", openErr)
			return persistenceBootstrap{}, err
		}
		if migrateErr := rs.Migrate(context.Background()); migrateErr != nil {
			_ = rs.Close()
			err = fmt.Errorf("migrate run store: %w", migrateErr)
			return persistenceBootstrap{}, err
		}
		bootstrap.runStore = rs
		opts.logger("run persistence enabled: %s", runDBPath)
	}

	if dbPath := strings.TrimSpace(opts.getenv("HARNESS_CONVERSATION_DB")); dbPath != "" {
		if !filepath.IsAbs(dbPath) {
			dbPath = filepath.Join(opts.workspace, dbPath)
		}
		store, openErr := harness.NewSQLiteConversationStore(dbPath)
		if openErr != nil {
			err = fmt.Errorf("create conversation store: %w", openErr)
			return persistenceBootstrap{}, err
		}
		if migrateErr := store.Migrate(context.Background()); migrateErr != nil {
			_ = store.Close()
			err = fmt.Errorf("migrate conversation store: %w", migrateErr)
			return persistenceBootstrap{}, err
		}
		bootstrap.conversationStore = store
		opts.logger("conversation persistence enabled: %s", dbPath)

		if opts.convRetentionDays > 0 {
			opts.logger("conversation retention policy: %d days", opts.convRetentionDays)
			convCleanerCtx, convCleanerCancel := context.WithCancel(context.Background())
			cleaner := harness.NewConversationCleaner(store, opts.convRetentionDays)
			cleaner.Start(convCleanerCtx, 24*time.Hour)
			bootstrap.convCleanerCancel = convCleanerCancel
		}
	}

	return bootstrap, nil
}

type triggerRuntime struct {
	validators *trigger.ValidatorRegistry
	github     any
	slack      any
	linear     any
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
	if s := strings.TrimSpace(getenv("GITHUB_WEBHOOK_SECRET")); s != "" {
		runtime.validators.Register("github", &trigger.GitHubValidator{Secret: s})
		logger("registered GitHub webhook validator")
		runtime.github = githubadapter.NewGitHubAdapter(s)
		logger("registered GitHub webhook adapter for /v1/webhooks/github")
	}
	if s := strings.TrimSpace(getenv("SLACK_SIGNING_SECRET")); s != "" {
		runtime.validators.Register("slack", &trigger.SlackValidator{Secret: s})
		logger("registered Slack webhook validator")
		runtime.slack = slackadapter.NewSlackAdapter()
		logger("registered Slack webhook adapter for /v1/webhooks/slack")
	}
	if s := strings.TrimSpace(getenv("LINEAR_WEBHOOK_SECRET")); s != "" {
		runtime.validators.Register("linear", &trigger.LinearValidator{Secret: s})
		logger("registered Linear webhook validator")
		runtime.linear = linearadapter.NewLinearAdapter()
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
	providerRegistry *catalog.ProviderRegistry
	runStore         istore.Store
	triggers         triggerRuntime
}

func buildServerOptions(opts serverBootstrapOptions) server.ServerOptions {
	var ghAdapter *githubadapter.GitHubAdapter
	if adapter, ok := opts.triggers.github.(*githubadapter.GitHubAdapter); ok {
		ghAdapter = adapter
	}
	var slAdapter *slackadapter.SlackAdapter
	if adapter, ok := opts.triggers.slack.(*slackadapter.SlackAdapter); ok {
		slAdapter = adapter
	}
	var linAdapter *linearadapter.LinearAdapter
	if adapter, ok := opts.triggers.linear.(*linearadapter.LinearAdapter); ok {
		linAdapter = adapter
	}

	return server.ServerOptions{
		Runner:           opts.runner,
		Catalog:          opts.modelCatalog,
		AgentRunner:      opts.runner,
		SkillLister:      opts.skillLister,
		Skills:           opts.skillManager,
		CronClient:       opts.cronClient,
		SubagentManager:  opts.subagentManager,
		ProviderRegistry: opts.providerRegistry,
		Store:            opts.runStore,
		Validators:       opts.triggers.validators,
		GitHubAdapter:    ghAdapter,
		SlackAdapter:     slAdapter,
		LinearAdapter:    linAdapter,
	}
}
