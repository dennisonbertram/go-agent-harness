package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/provider/catalog"
)

func New(runner *harness.Runner) http.Handler {
	return NewWithCatalog(runner, nil)
}

// NewWithCatalog creates an HTTP handler with an optional model catalog.
// When catalog is non-nil, the GET /v1/models endpoint returns the catalog contents.
func NewWithCatalog(runner *harness.Runner, cat *catalog.Catalog) http.Handler {
	s := &Server{runner: runner, catalog: cat}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/runs", s.handleRuns)
	mux.HandleFunc("/v1/runs/", s.handleRunByID)
	mux.HandleFunc("/v1/conversations/", s.handleConversations)
	mux.HandleFunc("/v1/models", s.handleModels)
	return mux
}

type Server struct {
	runner  *harness.Runner
	catalog *catalog.Catalog
}

// ModelResponse is the JSON shape for a single model in the /v1/models response.
type ModelResponse struct {
	ID                 string   `json:"id"`
	Provider           string   `json:"provider"`
	Aliases            []string `json:"aliases"`
	InputCostPerMTok   float64  `json:"input_cost_per_mtok"`
	OutputCostPerMTok  float64  `json:"output_cost_per_mtok"`
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
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req harness.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	run, err := s.runner.StartRun(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"run_id": run.ID,
		"status": run.Status,
	})
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
	state, ok := s.runner.GetRun(runID)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("run %q not found", runID))
		return
	}
	writeJSON(w, http.StatusOK, state)
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
