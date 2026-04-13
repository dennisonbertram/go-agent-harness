package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go-agent-harness/internal/store"
	"go-agent-harness/internal/workflows"
)

type workflowManager interface {
	ListDefinitions() []workflows.Definition
	GetDefinition(name string) (workflows.Definition, bool)
	Start(name string, input map[string]any) (workflows.Run, error)
	GetRun(runID string) (workflows.Run, []workflows.StepState, error)
	Subscribe(runID string) ([]workflows.Event, <-chan workflows.Event, func(), error)
	ResumeRun(ctx context.Context, runID string, payload map[string]any) error
}

func (s *Server) registerWorkflowRoutes(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("/v1/workflows", auth(http.HandlerFunc(s.handleWorkflows)))
	mux.Handle("/v1/workflows/", auth(http.HandlerFunc(s.handleWorkflowByName)))
	mux.Handle("/v1/workflow-runs/", auth(http.HandlerFunc(s.handleWorkflowRunByID)))
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if s.workflows == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "workflow service is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"workflows": s.workflows.ListDefinitions()})
	default:
		writeMethodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) handleWorkflowByName(w http.ResponseWriter, r *http.Request) {
	if s.workflows == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "workflow service is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflows/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	name := parts[0]
	if len(parts) == 1 {
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		def, ok := s.workflows.GetDefinition(name)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("workflow %q not found", name))
			return
		}
		writeJSON(w, http.StatusOK, def)
		return
	}
	if len(parts) == 2 && parts[1] == "runs" {
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		var req struct {
			Input map[string]any `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		run, err := s.workflows.Start(name, req.Input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"run_id": run.ID, "status": run.Status})
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleWorkflowRunByID(w http.ResponseWriter, r *http.Request) {
	if s.workflows == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "workflow service is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflow-runs/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	runID := parts[0]
	if len(parts) == 1 {
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		run, steps, err := s.workflows.GetRun(runID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("workflow run %q not found", runID))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":                    run.ID,
			"workflow_name":         run.WorkflowName,
			"status":                run.Status,
			"current_step_id":       run.CurrentStepID,
			"current_checkpoint_id": run.CurrentCheckpointID,
			"steps":                 steps,
		})
		return
	}
	if len(parts) == 2 && parts[1] == "resume" {
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		var req struct {
			Payload map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.Payload == nil {
			req.Payload = map[string]any{}
		}
		if err := s.workflows.ResumeRun(r.Context(), runID, req.Payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		history, stream, cancel, err := s.workflows.Subscribe(runID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("workflow run %q not found", runID))
			return
		}
		defer cancel()
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "stream_unsupported", "response writer does not support streaming")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		for _, event := range history {
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Seq, event.Type, mustJSON(event.Payload)); err != nil {
				return
			}
			flusher.Flush()
		}
		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-stream:
				if !ok {
					return
				}
				if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Seq, event.Type, mustJSON(event.Payload)); err != nil {
					return
				}
				flusher.Flush()
				if event.Type == "workflow.completed" || event.Type == "workflow.failed" {
					return
				}
			}
		}
	}
	http.NotFound(w, r)
}

func mustJSON(payload map[string]any) string {
	if payload == nil {
		return "{}"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
