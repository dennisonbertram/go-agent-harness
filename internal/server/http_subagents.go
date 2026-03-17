package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go-agent-harness/internal/subagents"
)

func (s *Server) handleSubagents(w http.ResponseWriter, r *http.Request) {
	if s.subagentManager == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "subagent manager is not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := s.subagentManager.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subagents": items})
	case http.MethodPost:
		var req subagents.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		item, err := s.subagentManager.Create(r.Context(), req)
		if err != nil {
			switch {
			case errors.Is(err, subagents.ErrInvalidConfig):
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
			}
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		writeMethodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) handleSubagentByID(w http.ResponseWriter, r *http.Request) {
	if s.subagentManager == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "subagent manager is not configured")
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/subagents/"), "/")
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "subagent not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := s.subagentManager.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, subagents.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		err := s.subagentManager.Delete(r.Context(), id)
		if err != nil {
			switch {
			case errors.Is(err, subagents.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", err.Error())
			case errors.Is(err, subagents.ErrActive):
				writeError(w, http.StatusConflict, "subagent_active", err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeMethodNotAllowed(w, http.MethodGet)
	}
}
