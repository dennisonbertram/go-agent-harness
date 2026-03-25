package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	githubadapter "go-agent-harness/internal/github"
	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/deferred"
	"go-agent-harness/internal/harness/tools/recipe"
	linearadapter "go-agent-harness/internal/linear"
	"go-agent-harness/internal/provider/catalog"
	slackadapter "go-agent-harness/internal/slack"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/subagents"
	"go-agent-harness/internal/trigger"
)

// CronClient is the interface the HTTP server uses to manage cron jobs.
// It mirrors tools.CronClient to allow easy wiring without import complexity.
type CronClient interface {
	CreateJob(ctx context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error)
	ListJobs(ctx context.Context) ([]tools.CronJob, error)
	GetJob(ctx context.Context, id string) (tools.CronJob, error)
	UpdateJob(ctx context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error)
	DeleteJob(ctx context.Context, id string) error
	ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]tools.CronExecution, error)
	Health(ctx context.Context) error
}

// SkillManager is the interface the HTTP server uses to query and verify skills.
// It mirrors tools.SkillVerifier to allow easy wiring without import complexity.
type SkillManager interface {
	GetSkill(name string) (tools.SkillInfo, bool)
	ListSkills() []tools.SkillInfo
	ResolveSkill(ctx context.Context, name, args, workspace string) (string, error)
	GetSkillFilePath(name string) (string, bool)
	UpdateSkillVerification(ctx context.Context, name string, verified bool, verifiedAt time.Time, verifiedBy string) error
}

func New(runner *harness.Runner) http.Handler {
	return NewWithCatalog(runner, nil)
}

// NewWithCatalog creates an HTTP handler with an optional model catalog.
// When catalog is non-nil, the GET /v1/models endpoint returns the catalog contents.
func NewWithCatalog(runner *harness.Runner, cat *catalog.Catalog) http.Handler {
	return NewWithOptions(ServerOptions{Runner: runner, Catalog: cat})
}

// ServerOptions holds the full set of optional dependencies for the HTTP server.
type ServerOptions struct {
	Runner            *harness.Runner
	Catalog           *catalog.Catalog
	AgentRunner       agentRunnerIface
	ForkedAgentRunner forkedAgentRunnerIface
	SkillLister       skillListerIface
	CronClient        CronClient
	Skills            SkillManager
	Todos             deferred.TodoManager
	Recipes           []recipe.Recipe
	Sourcegraph       sourcegraphConfig
	HTTPClient        *http.Client
	MCPConnector      MCPConnector
	SubagentManager   subagents.Manager
	ProviderRegistry  *catalog.ProviderRegistry
	// Store is an optional persistence layer for run state.
	// When provided, GET /v1/runs supports filtering and completed runs are
	// retrievable after the runner forgets them.
	Store store.Store
	// AuthDisabled skips Bearer token authentication for all requests (issue #9).
	// Set to true in tests that do not provision API keys.
	AuthDisabled bool
	// ApprovalBroker is the broker for POST /v1/runs/{id}/approve and
	// POST /v1/runs/{id}/deny. When nil, those endpoints return 501.
	ApprovalBroker harness.ApprovalBroker
	// ProfilesProject is the project-level profiles directory for GET /v1/profiles.
	// Defaults to .harness/profiles relative to cwd when empty.
	ProfilesProject string
	// ProfilesUser is the user-global profiles directory for GET /v1/profiles.
	// Defaults to ~/.harness/profiles when empty.
	ProfilesUser string
	// ProfilesDir is the directory for user-created profiles.
	// When non-empty, POST/PUT/DELETE /v1/profiles/{name} endpoints are enabled.
	ProfilesDir string
	// Validators is an optional registry of webhook signature validators for
	// POST /v1/external/trigger. When nil, the endpoint returns 401 for all requests.
	Validators *trigger.ValidatorRegistry
	// GitHubAdapter is an optional GitHub webhook adapter for POST /v1/webhooks/github.
	// When nil, the endpoint returns 401 for all requests.
	GitHubAdapter *githubadapter.GitHubAdapter
	// SlackAdapter is an optional Slack webhook adapter for POST /v1/webhooks/slack.
	// When nil, the endpoint returns 401 for all requests.
	SlackAdapter *slackadapter.SlackAdapter
	// LinearAdapter is an optional Linear webhook adapter for POST /v1/webhooks/linear.
	// When nil, the endpoint returns 401 for all requests.
	LinearAdapter *linearadapter.LinearAdapter
}

// NewWithOptions creates an HTTP handler with the full set of optional dependencies.
func NewWithOptions(opts ServerOptions) http.Handler {
	s := &Server{
		runner:            opts.Runner,
		catalog:           opts.Catalog,
		providerRegistry:  opts.ProviderRegistry,
		agentRunner:       opts.AgentRunner,
		forkedAgentRunner: opts.ForkedAgentRunner,
		skillLister:       opts.SkillLister,
		cronClient:        opts.CronClient,
		skills:            opts.Skills,
		todos:             opts.Todos,
		recipes:           opts.Recipes,
		sourcegraph:       opts.Sourcegraph,
		httpClient:        opts.HTTPClient,
		mcpConnector:      opts.MCPConnector,
		subagentManager:   opts.SubagentManager,
		runStore:          opts.Store,
		approvalBroker:    opts.ApprovalBroker,
		profilesDir:       opts.ProfilesDir,
		mcpServers:        make(map[string]connectedMCPServer),
		timeNow:           time.Now,
		authDisabled:      opts.AuthDisabled || authDisabledFromEnv(),
		profilesProject:   opts.ProfilesProject,
		profilesUser:      opts.ProfilesUser,
		validators:        opts.Validators,
		githubAdapter:     opts.GitHubAdapter,
		slackAdapter:      opts.SlackAdapter,
		linearAdapter:     opts.LinearAdapter,
	}
	// If runner config has an approval broker, use it as default when none
	// is explicitly supplied in ServerOptions.
	if s.approvalBroker == nil && opts.Runner != nil {
		if ab := opts.Runner.ApprovalBroker(); ab != nil {
			s.approvalBroker = ab
		}
	}
	return s.buildMux()
}

// NewWithCron creates a Server with an optional cron client.
func NewWithCron(runner *harness.Runner, cat *catalog.Catalog, cronClient CronClient) *Server {
	return &Server{runner: runner, catalog: cat, cronClient: cronClient, mcpServers: make(map[string]connectedMCPServer), timeNow: time.Now}
}

// NewWithSkills creates a Server with an optional skill manager.
func NewWithSkills(runner *harness.Runner, cat *catalog.Catalog, skills SkillManager) *Server {
	return &Server{runner: runner, catalog: cat, skills: skills, mcpServers: make(map[string]connectedMCPServer), timeNow: time.Now}
}

// Handler returns an http.Handler for the server.
func (s *Server) Handler() http.Handler {
	return s.buildMux()
}

func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)

	// auth wraps a handler with Bearer token authentication.
	auth := s.authMiddleware

	// read wraps a handler requiring runs:read scope (after auth).
	// Combine as: auth(read(handler)) — auth runs first, then scope check.
	read := s.requireScope(store.ScopeRunsRead)
	write := s.requireScope(store.ScopeRunsWrite)
	admin := s.requireScope(store.ScopeAdmin)

	// /v1/runs  — GET requires runs:read, POST requires runs:write.
	// The handler dispatches internally so scope is enforced per-method inside
	// handleRuns / handleRunByID.
	s.registerRunRoutes(mux, auth)

	// /v1/conversations/ — mixed methods; scope enforced inside handler.
	s.registerConversationRoutes(mux, auth)

	// Pure read endpoints.
	mux.Handle("/v1/models", auth(read(http.HandlerFunc(s.handleModels))))
	// POST /v1/agents — requires runs:write (agent execution is a mutating operation).
	mux.Handle("/v1/agents", auth(write(http.HandlerFunc(s.handleAgents))))

	// /v1/subagents — GET requires runs:read, POST requires runs:write.
	mux.Handle("/v1/subagents", auth(http.HandlerFunc(s.handleSubagents)))
	mux.Handle("/v1/subagents/", auth(http.HandlerFunc(s.handleSubagentByID)))

	// /v1/providers — GET requires runs:read.
	// /v1/providers/{name}/key — PUT requires admin.
	mux.Handle("/v1/providers", auth(read(http.HandlerFunc(s.handleProviders))))
	mux.Handle("/v1/providers/", auth(admin(http.HandlerFunc(s.handleProviderByName))))

	// /v1/summarize — POST requires runs:write.
	mux.Handle("/v1/summarize", auth(write(http.HandlerFunc(s.handleSummarize))))

	// /v1/cron — mixed methods; scope enforced inside handler.
	mux.Handle("/v1/cron/jobs", auth(http.HandlerFunc(s.handleCronJobsRoot)))
	mux.Handle("/v1/cron/jobs/", auth(http.HandlerFunc(s.handleCronJobByID)))

	// /v1/skills — GET requires runs:read; POST /verify requires runs:write.
	mux.Handle("/v1/skills", auth(read(http.HandlerFunc(s.handleSkillsRoot))))
	mux.Handle("/v1/skills/", auth(http.HandlerFunc(s.handleSkillByName)))

	// Pure read endpoints.
	mux.Handle("/v1/recipes", auth(read(http.HandlerFunc(s.handleRecipes))))
	mux.Handle("/v1/recipes/", auth(read(http.HandlerFunc(s.handleRecipes))))
	// POST /v1/search/code — requires runs:write (executes a search, proxying external service).
	mux.Handle("/v1/search/code", auth(write(http.HandlerFunc(s.handleSearchCode))))

	// /v1/mcp/servers — GET requires runs:read; POST/DELETE require admin.
	mux.Handle("/v1/mcp/servers", auth(http.HandlerFunc(s.handleMCPServers)))

	// /v1/profiles/ — POST requires runs:write; PUT/DELETE require runs:write.
	// GET /v1/profiles and GET /v1/profiles/{name} are read-only (runs:read).
	mux.Handle("/v1/profiles", auth(read(http.HandlerFunc(s.handleProfilesRoot))))
	mux.Handle("/v1/profiles/", auth(http.HandlerFunc(s.handleProfileByName)))

	// POST /v1/external/trigger — source-agnostic external trigger endpoint (issue #411).
	// Authentication is performed via source-specific HMAC signature validation rather
	// than the standard Bearer token middleware, so this route bypasses auth middleware.
	mux.HandleFunc("/v1/external/trigger", s.handleExternalTrigger)

	// POST /v1/webhooks/github — GitHub-specific webhook endpoint (issue #412).
	// Reads X-GitHub-Event / X-GitHub-Delivery / X-Hub-Signature-256 headers and
	// converts the GitHub payload into a normalized trigger envelope. Authentication
	// is performed via HMAC-SHA256 validation, so this route also bypasses Bearer auth.
	mux.HandleFunc("/v1/webhooks/github", s.handleGitHubWebhook)

	// POST /v1/webhooks/slack — Slack-specific webhook endpoint (issue #413).
	// Reads X-Slack-Request-Timestamp / X-Slack-Signature headers and converts the
	// Slack event_callback payload into a normalized trigger envelope. Authentication
	// is performed via HMAC-SHA256 validation, so this route also bypasses Bearer auth.
	mux.HandleFunc("/v1/webhooks/slack", s.handleSlackWebhook)

	// POST /v1/webhooks/linear — Linear-specific webhook endpoint (issue #413).
	// Reads X-Linear-Signature header and converts the Linear webhook payload into a
	// normalized trigger envelope. Authentication is performed via HMAC-SHA256 validation,
	// so this route also bypasses Bearer auth.
	mux.HandleFunc("/v1/webhooks/linear", s.handleLinearWebhook)

	return mux
}

type Server struct {
	runner            *harness.Runner
	catalog           *catalog.Catalog
	providerRegistry  *catalog.ProviderRegistry
	agentRunner       agentRunnerIface
	forkedAgentRunner forkedAgentRunnerIface
	skillLister       skillListerIface
	cronClient        CronClient
	skills            SkillManager
	approvalBroker    harness.ApprovalBroker

	// Todos management (issue #148)
	todos deferred.TodoManager

	// Recipe listing (issue #147)
	recipes []recipe.Recipe

	// Sourcegraph proxy (issue #150)
	sourcegraph sourcegraphConfig
	httpClient  *http.Client

	// MCP server management (issue #145)
	mcpConnector MCPConnector
	mcpMu        sync.RWMutex
	mcpServers   map[string]connectedMCPServer

	subagentManager subagents.Manager

	// runStore is an optional persistence layer for run state (issue #7).
	// When non-nil, GET /v1/runs supports filtering and run history survives restarts.
	runStore store.Store

	// profilesDir is the directory for user-created profiles (issue #378).
	// When non-empty, POST/PUT/DELETE /v1/profiles/{name} endpoints are enabled.
	profilesDir string

	timeNow func() time.Time // injectable for tests; defaults to time.Now

	// authDisabled disables Bearer token auth for all requests (issue #9).
	authDisabled bool

	// profilesProject and profilesUser are the directories used by GET /v1/profiles.
	profilesProject string
	profilesUser    string

	// validators is the registry of webhook signature validators for
	// POST /v1/external/trigger (issue #411).
	validators *trigger.ValidatorRegistry

	// githubAdapter converts GitHub webhook requests into trigger envelopes (issue #412).
	// When nil, POST /v1/webhooks/github returns 401.
	githubAdapter *githubadapter.GitHubAdapter

	// slackAdapter converts Slack webhook requests into trigger envelopes (issue #413).
	// When nil, POST /v1/webhooks/slack returns 401.
	slackAdapter *slackadapter.SlackAdapter

	// linearAdapter converts Linear webhook requests into trigger envelopes (issue #413).
	// When nil, POST /v1/webhooks/linear returns 401.
	linearAdapter *linearadapter.LinearAdapter
}

// ModelResponse is the JSON shape for a single model in the /v1/models response.
type ModelResponse struct {
	ID                string   `json:"id"`
	Provider          string   `json:"provider"`
	Aliases           []string `json:"aliases"`
	InputCostPerMTok  float64  `json:"input_cost_per_mtok"`
	OutputCostPerMTok float64  `json:"output_cost_per_mtok"`
}

// ProviderResponse is the JSON shape for a single provider in the /v1/providers response.
type ProviderResponse struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	APIKeyEnv  string `json:"api_key_env"`
	BaseURL    string `json:"base_url"`
	ModelCount int    `json:"model_count"`
}

// handleProviders handles GET /v1/providers.
// Returns provider availability based on whether their API key env vars are set.
func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	if s.catalog == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []ProviderResponse{}})
		return
	}

	// Iterate providers in sorted order for deterministic output.
	providerNames := make([]string, 0, len(s.catalog.Providers))
	for name := range s.catalog.Providers {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)

	providers := make([]ProviderResponse, 0, len(providerNames))
	for _, name := range providerNames {
		entry := s.catalog.Providers[name]
		configured := false
		if s.providerRegistry != nil {
			configured = s.providerRegistry.IsConfigured(name)
		} else {
			configured = os.Getenv(entry.APIKeyEnv) != ""
		}
		providers = append(providers, ProviderResponse{
			Name:       name,
			Configured: configured,
			APIKeyEnv:  entry.APIKeyEnv,
			BaseURL:    entry.BaseURL,
			ModelCount: len(entry.Models),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// handleProviderByName handles PUT /v1/providers/{name}/key.
func (s *Server) handleProviderByName(w http.ResponseWriter, r *http.Request) {
	// Parse /v1/providers/{name}/key
	path := strings.TrimPrefix(r.URL.Path, "/v1/providers/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "key" {
		http.NotFound(w, r)
		return
	}
	name := parts[0]
	if name == "" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPut {
		writeMethodNotAllowed(w, http.MethodPut)
		return
	}

	if s.providerRegistry == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "provider registry is not configured")
		return
	}

	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain a non-empty \"key\" field")
		return
	}

	s.providerRegistry.SetAPIKey(name, body.Key)
	w.WriteHeader(http.StatusNoContent)
}

// handleSummarize handles POST /v1/summarize.
// Accepts a list of messages and returns an LLM-generated summary.
func (s *Server) handleSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	summarizer := s.runner.GetSummarizer()
	if summarizer == nil {
		writeError(w, http.StatusServiceUnavailable, "summarizer_not_configured", "summarizer not configured")
		return
	}

	var req struct {
		Messages []harness.Message `json:"messages"`
		System   string            `json:"system"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "messages is required and must not be empty")
		return
	}

	// Convert messages to the map format expected by MessageSummarizer.
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		entry := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		msgs = append(msgs, entry)
	}

	summary, err := summarizer.SummarizeMessages(r.Context(), msgs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "summarize_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

// handleModels handles GET /v1/models.
// Returns the list of available models from the catalog.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	if s.catalog == nil && s.providerRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]any{"models": []ModelResponse{}})
		return
	}

	var models []ModelResponse
	if s.providerRegistry != nil {
		results := s.providerRegistry.ListModelsContext(r.Context())
		for _, result := range results {
			aliases := s.providerRegistry.ModelAliasesContext(r.Context(), result.Provider)[result.ModelID]
			if aliases == nil {
				aliases = []string{}
			}
			var inputCost, outputCost float64
			if result.Model.Pricing != nil {
				inputCost = result.Model.Pricing.InputPer1MTokensUSD
				outputCost = result.Model.Pricing.OutputPer1MTokensUSD
			}
			models = append(models, ModelResponse{
				ID:                result.ModelID,
				Provider:          result.Provider,
				Aliases:           aliases,
				InputCostPerMTok:  inputCost,
				OutputCostPerMTok: outputCost,
			})
		}
	} else {
		// Build a reverse alias map per provider: modelID -> []alias
		type providerAliases map[string][]string
		aliasMap := make(map[string]providerAliases)
		for providerName, providerEntry := range s.catalog.Providers {
			pa := make(providerAliases)
			for alias, target := range providerEntry.Aliases {
				pa[target] = append(pa[target], alias)
			}
			aliasMap[providerName] = pa
		}

		// Iterate providers in sorted order for deterministic output.
		providerNames := make([]string, 0, len(s.catalog.Providers))
		for name := range s.catalog.Providers {
			providerNames = append(providerNames, name)
		}
		sort.Strings(providerNames)

		for _, providerName := range providerNames {
			providerEntry := s.catalog.Providers[providerName]
			pa := aliasMap[providerName]

			// Iterate models in sorted order for deterministic output.
			modelIDs := make([]string, 0, len(providerEntry.Models))
			for id := range providerEntry.Models {
				modelIDs = append(modelIDs, id)
			}
			sort.Strings(modelIDs)

			for _, modelID := range modelIDs {
				model := providerEntry.Models[modelID]

				aliases := pa[modelID]
				if aliases == nil {
					aliases = []string{}
				}
				sort.Strings(aliases)

				var inputCost, outputCost float64
				if model.Pricing != nil {
					inputCost = model.Pricing.InputPer1MTokensUSD
					outputCost = model.Pricing.OutputPer1MTokensUSD
				}

				models = append(models, ModelResponse{
					ID:                modelID,
					Provider:          providerName,
					Aliases:           aliases,
					InputCostPerMTok:  inputCost,
					OutputCostPerMTok: outputCost,
				})
			}
		}
	}

	if models == nil {
		models = []ModelResponse{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func writeSSE(w http.ResponseWriter, event harness.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %s\n", event.ID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "retry: 3000\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event.Type); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}
	return nil
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
