package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/deferred"
	"go-agent-harness/internal/harness/tools/recipe"
	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/subagents"
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
	// Store is an optional persistence layer for run state.
	// When provided, GET /v1/runs supports filtering and completed runs are
	// retrievable after the runner forgets them.
	Store store.Store
	// AuthDisabled skips Bearer token authentication for all requests (issue #9).
	// Set to true in tests that do not provision API keys.
	AuthDisabled bool
}

// NewWithOptions creates an HTTP handler with the full set of optional dependencies.
func NewWithOptions(opts ServerOptions) http.Handler {
	s := &Server{
		runner:            opts.Runner,
		catalog:           opts.Catalog,
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
		mcpServers:        make(map[string]connectedMCPServer),
		timeNow:           time.Now,
		authDisabled:      opts.AuthDisabled || authDisabledFromEnv(),
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
	auth := s.authMiddleware
	mux.Handle("/v1/runs", auth(http.HandlerFunc(s.handleRuns)))
	mux.Handle("/v1/runs/", auth(http.HandlerFunc(s.handleRunByID)))
	mux.Handle("/v1/conversations/", auth(http.HandlerFunc(s.handleConversations)))
	mux.Handle("/v1/models", auth(http.HandlerFunc(s.handleModels)))
	mux.Handle("/v1/agents", auth(http.HandlerFunc(s.handleAgents)))
	mux.Handle("/v1/subagents", auth(http.HandlerFunc(s.handleSubagents)))
	mux.Handle("/v1/subagents/", auth(http.HandlerFunc(s.handleSubagentByID)))
	mux.Handle("/v1/providers", auth(http.HandlerFunc(s.handleProviders)))
	mux.Handle("/v1/summarize", auth(http.HandlerFunc(s.handleSummarize)))
	mux.Handle("/v1/cron/jobs", auth(http.HandlerFunc(s.handleCronJobsRoot)))
	mux.Handle("/v1/cron/jobs/", auth(http.HandlerFunc(s.handleCronJobByID)))
	mux.Handle("/v1/skills", auth(http.HandlerFunc(s.handleSkillsRoot)))
	mux.Handle("/v1/skills/", auth(http.HandlerFunc(s.handleSkillByName)))
	mux.Handle("/v1/recipes", auth(http.HandlerFunc(s.handleRecipes)))
	mux.Handle("/v1/recipes/", auth(http.HandlerFunc(s.handleRecipes)))
	mux.Handle("/v1/search/code", auth(http.HandlerFunc(s.handleSearchCode)))
	mux.Handle("/v1/mcp/servers", auth(http.HandlerFunc(s.handleMCPServers)))
	return mux
}

type Server struct {
	runner            *harness.Runner
	catalog           *catalog.Catalog
	agentRunner       agentRunnerIface
	forkedAgentRunner forkedAgentRunnerIface
	skillLister       skillListerIface
	cronClient        CronClient
	skills            SkillManager

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

	timeNow func() time.Time // injectable for tests; defaults to time.Now

	// authDisabled disables Bearer token auth for all requests (issue #9).
	authDisabled bool
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
		providers = append(providers, ProviderResponse{
			Name:       name,
			Configured: os.Getenv(entry.APIKeyEnv) != "",
			APIKeyEnv:  entry.APIKeyEnv,
			BaseURL:    entry.BaseURL,
			ModelCount: len(entry.Models),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
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

	if s.catalog == nil {
		writeJSON(w, http.StatusOK, map[string]any{"models": []ModelResponse{}})
		return
	}

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

	var models []ModelResponse
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

	if models == nil {
		models = []ModelResponse{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePostRun(w, r)
	case http.MethodGet:
		s.handleListRuns(w, r)
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// handlePostRun handles POST /v1/runs — starts a new run.
func (s *Server) handlePostRun(w http.ResponseWriter, r *http.Request) {
	var req harness.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	// Populate InitiatorAPIKeyPrefix from auth context for audit trail provenance.
	req.InitiatorAPIKeyPrefix = APIKeyPrefixFromContext(r.Context())

	run, err := s.runner.StartRun(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Persist the initial run record to the store when configured.
	if s.runStore != nil {
		storeRun := harnessRunToStore(run)
		_ = s.runStore.CreateRun(r.Context(), storeRun) // best-effort
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"run_id": run.ID,
		"status": run.Status,
	})
}

// handleListRuns handles GET /v1/runs?conversation_id=X&status=Y&tenant_id=Z.
// Requires a runStore; returns 501 if store is not configured.
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if s.runStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "run persistence is not configured")
		return
	}
	filter := store.RunFilter{
		ConversationID: strings.TrimSpace(r.URL.Query().Get("conversation_id")),
		TenantID:       strings.TrimSpace(r.URL.Query().Get("tenant_id")),
		Status:         store.RunStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
	}
	runs, err := s.runStore.ListRuns(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

// handleListConversationRuns handles GET /v1/conversations/{id}/runs.
// Returns all runs associated with the given conversation ID, ordered newest first.
// Requires a runStore; returns 501 if store is not configured.
func (s *Server) handleListConversationRuns(w http.ResponseWriter, r *http.Request, conversationID string) {
	if s.runStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "run persistence is not configured")
		return
	}
	if strings.TrimSpace(conversationID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "conversation ID is required")
		return
	}
	filter := store.RunFilter{
		ConversationID: conversationID,
	}
	runs, err := s.runStore.ListRuns(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *Server) handleRunByID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/runs/") {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/runs/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	runID := parts[0]

	// Intercept /v1/runs/replay before treating "replay" as a run ID.
	if runID == "replay" && len(parts) == 1 {
		s.handleRunReplay(w, r)
		return
	}

	if len(parts) == 1 {
		s.handleGetRun(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		s.handleRunEvents(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "input" {
		s.handleRunInput(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "summary" {
		s.handleRunSummary(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "continue" {
		s.handleRunContinue(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "steer" {
		s.handleRunSteer(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "context" {
		s.handleRunContext(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "compact" {
		s.handleRunCompact(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "todos" {
		s.handleRunTodos(w, r, runID)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleRunSteer(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "message is required")
		return
	}

	if err := s.runner.SteerRun(runID, req.Message); err != nil {
		if errors.Is(err, harness.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
			return
		}
		if errors.Is(err, harness.ErrRunNotActive) {
			writeError(w, http.StatusConflict, "run_not_active", err.Error())
			return
		}
		if errors.Is(err, harness.ErrSteeringBufferFull) {
			writeError(w, http.StatusTooManyRequests, "steering_buffer_full", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}

// handleRunContext handles GET /v1/runs/{id}/context.
// Returns the current context window status for a run.
func (s *Server) handleRunContext(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	status, err := s.runner.GetRunContextStatus(runID)
	if err != nil {
		if errors.Is(err, harness.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// handleRunCompact handles POST /v1/runs/{id}/compact.
// Triggers in-memory context compaction on the active run.
func (s *Server) handleRunCompact(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req struct {
		Mode     string `json:"mode"`
		KeepLast int    `json:"keep_last"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	result, err := s.runner.CompactRun(r.Context(), runID, harness.CompactRunRequest{
		Mode:     req.Mode,
		KeepLast: req.KeepLast,
	})
	if err != nil {
		if errors.Is(err, harness.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
			return
		}
		if errors.Is(err, harness.ErrRunNotActive) {
			writeError(w, http.StatusConflict, "run_not_active", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"messages_removed": result.MessagesRemoved,
	})
}

func (s *Server) handleRunContinue(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "message is required")
		return
	}

	newRun, err := s.runner.ContinueRun(runID, req.Message)
	if err != nil {
		if errors.Is(err, harness.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
			return
		}
		if errors.Is(err, harness.ErrRunNotCompleted) {
			writeError(w, http.StatusConflict, "run_not_completed", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"run_id": newRun.ID,
		"status": newRun.Status,
	})
}

func (s *Server) handleRunInput(w http.ResponseWriter, r *http.Request, runID string) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetRunInput(w, runID)
		return
	case http.MethodPost:
		s.handlePostRunInput(w, r, runID)
		return
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func (s *Server) handleGetRunInput(w http.ResponseWriter, runID string) {
	pending, err := s.runner.PendingInput(runID)
	if err != nil {
		if errors.Is(err, harness.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
			return
		}
		if errors.Is(err, harness.ErrNoPendingInput) {
			writeError(w, http.StatusConflict, "no_pending_input", "run is not waiting for user input")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pending)
}

func (s *Server) handlePostRunInput(w http.ResponseWriter, r *http.Request, runID string) {
	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if req.Answers == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "answers is required")
		return
	}

	err := s.runner.SubmitInput(runID, req.Answers)
	if err != nil {
		if errors.Is(err, harness.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
			return
		}
		if errors.Is(err, harness.ErrNoPendingInput) {
			writeError(w, http.StatusConflict, "no_pending_input", "run is not waiting for user input")
			return
		}
		if errors.Is(err, harness.ErrInvalidRunInput) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	// Check the runner's in-memory state first (active or recently completed runs).
	if state, ok := s.runner.GetRun(runID); ok {
		writeJSON(w, http.StatusOK, state)
		return
	}
	// Fall back to the persistent store for completed/historical runs.
	if s.runStore != nil {
		storeRun, err := s.runStore.GetRun(r.Context(), runID)
		if err == nil {
			// Convert store.Run back to a minimal harness.Run-compatible response.
			writeJSON(w, http.StatusOK, storeRunToHarness(storeRun))
			return
		}
		if !store.IsNotFound(err) {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
}

func (s *Server) handleRunSummary(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	summary, err := s.runner.GetRunSummary(runID)
	if err != nil {
		if errors.Is(err, harness.ErrRunNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
			return
		}
		writeError(w, http.StatusConflict, "run_not_finished", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleRunEvents(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	history, stream, cancel, err := s.runner.Subscribe(runID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
		return
	}
	defer cancel()

	// Support Last-Event-ID reconnection: skip already-seen events.
	if lastID := r.Header.Get("Last-Event-ID"); lastID != "" {
		if _, seq, err := harness.ParseEventID(lastID); err == nil {
			if int(seq+1) < len(history) {
				history = history[seq+1:]
			} else {
				history = nil
			}
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "response writer does not support streaming")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for _, event := range history {
		if err := writeSSE(w, event); err != nil {
			return
		}
		flusher.Flush()
		if harness.IsTerminalEvent(event.Type) {
			return
		}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			if err := writeSSE(w, event); err != nil {
				if errors.Is(err, http.ErrHandlerTimeout) {
					return
				}
				return
			}
			flusher.Flush()
			if harness.IsTerminalEvent(event.Type) {
				return
			}
		}
	}
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/conversations/")

	// GET /v1/conversations/ — list conversations
	if path == "" || r.URL.Path == "/v1/conversations/" {
		s.handleListConversations(w, r)
		return
	}

	parts := strings.Split(path, "/")

	// GET /v1/conversations/search?q=... — full-text search
	if len(parts) == 1 && parts[0] == "search" {
		s.handleSearchConversations(w, r)
		return
	}

	// DELETE /v1/conversations/{id}
	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleDeleteConversation(w, r, parts[0])
		return
	}

	// GET /v1/conversations/{id}/messages
	if len(parts) == 2 && parts[1] == "messages" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		convID := parts[0]
		msgs, ok := s.runner.ConversationMessages(convID)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("conversation %q not found", convID))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
		return
	}

	// GET /v1/conversations/{id}/runs — list runs for a conversation
	if len(parts) == 2 && parts[1] == "runs" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleListConversationRuns(w, r, parts[0])
		return
	}

	// GET /v1/conversations/{id}/export — JSONL export
	if len(parts) == 2 && parts[1] == "export" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleExportConversation(w, r, parts[0])
		return
	}

	// POST /v1/conversations/{id}/compact — context compaction (Issue #33)
	if len(parts) == 2 && parts[1] == "compact" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleCompactConversation(w, r, parts[0])
		return
	}

	// POST /v1/conversations/cleanup — retention-based bulk delete (Issue #34)
	if len(parts) == 1 && parts[0] == "cleanup" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleConversationsCleanup(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleSearchConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	store := s.runner.GetConversationStore()
	if store == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "conversation persistence is not configured")
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "query parameter \"q\" is required")
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			limit = n
		}
	}

	results, err := store.SearchMessages(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleExportConversation(w http.ResponseWriter, r *http.Request, convID string) {
	// Try in-memory first (active run), fall back to store.
	msgs, ok := s.runner.ConversationMessages(convID)
	if !ok {
		store := s.runner.GetConversationStore()
		if store == nil {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("conversation %q not found", convID))
			return
		}
		loaded, err := store.LoadMessages(r.Context(), convID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if len(loaded) == 0 {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("conversation %q not found", convID))
			return
		}
		msgs = loaded
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	for _, msg := range msgs {
		if err := enc.Encode(msg); err != nil {
			return
		}
	}
}

// handleCompactConversation handles POST /v1/conversations/{id}/compact.
// It replaces early messages with a summary (Issue #33).
func (s *Server) handleCompactConversation(w http.ResponseWriter, r *http.Request, convID string) {
	store := s.runner.GetConversationStore()
	if store == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "conversation persistence is not configured")
		return
	}

	var req struct {
		KeepFromStep int    `json:"keep_from_step"`
		Summary      string `json:"summary"`
		Role         string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(req.Summary) == "" {
		// Auto-generate summary via LLM when none provided.
		msgs, err := store.LoadMessages(r.Context(), convID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if len(msgs) == 0 {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("conversation %q not found", convID))
			return
		}
		generated, err := s.runner.SummarizeMessages(r.Context(), msgs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("auto-summary failed: %s", err.Error()))
			return
		}
		req.Summary = generated
	}
	if req.KeepFromStep < 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "keep_from_step must be >= 0")
		return
	}

	role := req.Role
	if role == "" {
		role = "system"
	}

	summaryMsg := harness.Message{
		Role:             role,
		Content:          req.Summary,
		IsCompactSummary: true,
	}

	if err := store.CompactConversation(r.Context(), convID, req.KeepFromStep, summaryMsg); err != nil {
		// Distinguish "not found" from other errors by checking the error message.
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("conversation %q not found", convID))
			return
		}
		if strings.Contains(err.Error(), "keepFromStep must be") {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Return the new message count.
	msgs, err := store.LoadMessages(r.Context(), convID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"compacted":     true,
		"message_count": len(msgs),
	})
}

// handleConversationsCleanup handles POST /v1/conversations/cleanup.
// It deletes non-pinned conversations older than max_age_days (default 30).
// Response: {"deleted": N}
func (s *Server) handleConversationsCleanup(w http.ResponseWriter, r *http.Request) {
	store := s.runner.GetConversationStore()
	if store == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "conversation persistence is not configured")
		return
	}

	var req struct {
		MaxAgeDays *int `json:"max_age_days"`
	}
	// Body is optional — ignore decode errors for empty body.
	_ = json.NewDecoder(r.Body).Decode(&req)

	maxAgeDays := 30
	if req.MaxAgeDays != nil {
		maxAgeDays = *req.MaxAgeDays
	}

	if maxAgeDays <= 0 {
		// 0 means disabled — nothing to delete.
		writeJSON(w, http.StatusOK, map[string]any{"deleted": 0})
		return
	}

	threshold := s.timeNow().UTC().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
	n, err := store.DeleteOldConversations(r.Context(), threshold)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	// Delegate to search handler when ?q= is present.
	if q := r.URL.Query().Get("q"); q != "" {
		s.handleSearchConversations(w, r)
		return
	}
	store := s.runner.GetConversationStore()
	if store == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "conversation persistence is not configured")
		return
	}

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			offset = n
		}
	}

	filter := harness.ConversationFilter{
		Workspace: strings.TrimSpace(r.URL.Query().Get("workspace")),
		TenantID:  strings.TrimSpace(r.URL.Query().Get("tenant_id")),
	}

	convs, err := store.ListConversations(r.Context(), filter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": convs})
}

func (s *Server) handleDeleteConversation(w http.ResponseWriter, r *http.Request, convID string) {
	store := s.runner.GetConversationStore()
	if store == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "conversation persistence is not configured")
		return
	}
	if err := store.DeleteConversation(r.Context(), convID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// harnessRunToStore converts a harness.Run to a store.Run for initial persistence.
// Usage/cost JSON fields are left empty; they are updated via UpdateRun later.
func harnessRunToStore(run harness.Run) *store.Run {
	return &store.Run{
		ID:             run.ID,
		ConversationID: run.ConversationID,
		TenantID:       run.TenantID,
		AgentID:        run.AgentID,
		Model:          run.Model,
		ProviderName:   run.ProviderName,
		Prompt:         run.Prompt,
		Status:         store.RunStatus(run.Status),
		Output:         run.Output,
		Error:          run.Error,
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
	}
}

// storeRunToHarness converts a store.Run to a map suitable for JSON response.
// This avoids a circular import between server and harness packages.
func storeRunToHarness(r *store.Run) map[string]any {
	return map[string]any{
		"id":              r.ID,
		"conversation_id": r.ConversationID,
		"tenant_id":       r.TenantID,
		"agent_id":        r.AgentID,
		"model":           r.Model,
		"provider_name":   r.ProviderName,
		"prompt":          r.Prompt,
		"status":          r.Status,
		"output":          r.Output,
		"error":           r.Error,
		"created_at":      r.CreatedAt,
		"updated_at":      r.UpdatedAt,
	}
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
