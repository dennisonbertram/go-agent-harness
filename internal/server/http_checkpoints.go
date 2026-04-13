package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go-agent-harness/internal/checkpoints"
	"go-agent-harness/internal/store"
)

type checkpointManager interface {
	Get(ctx context.Context, id string) (checkpoints.Record, error)
	Resume(ctx context.Context, id string, payload map[string]any) error
}

func (s *Server) registerCheckpointRoutes(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("/v1/checkpoints/", auth(http.HandlerFunc(s.handleCheckpointByID)))
}

func (s *Server) handleCheckpointByID(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/checkpoints/") {
		http.NotFound(w, r)
		return
	}
	if s.checkpoints == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "checkpoint service is not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/checkpoints/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	checkpointID := parts[0]
	if len(parts) == 1 {
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		s.handleGetCheckpoint(w, r, checkpointID)
		return
	}
	if len(parts) == 2 && parts[1] == "resume" {
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		s.handleResumeCheckpoint(w, r, checkpointID)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetCheckpoint(w http.ResponseWriter, r *http.Request, checkpointID string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	record, err := s.checkpoints.Get(r.Context(), checkpointID)
	if err != nil {
		if checkpoints.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("checkpoint %q not found", checkpointID))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleResumeCheckpoint(w http.ResponseWriter, r *http.Request, checkpointID string) {
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

	if err := s.checkpoints.Resume(r.Context(), checkpointID, req.Payload); err != nil {
		if checkpoints.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("checkpoint %q not found", checkpointID))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "resumed"})
}
