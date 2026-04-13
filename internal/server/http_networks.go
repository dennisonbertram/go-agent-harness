package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go-agent-harness/internal/networks"
	"go-agent-harness/internal/store"
	"go-agent-harness/internal/workflows"
)

type networkManager interface {
	ListDefinitions() []networks.Definition
	GetDefinition(name string) (networks.Definition, bool)
	Start(name string, input map[string]any) (workflows.Run, error)
}

func (s *Server) registerNetworkRoutes(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("/v1/networks", auth(http.HandlerFunc(s.handleNetworks)))
	mux.Handle("/v1/networks/", auth(http.HandlerFunc(s.handleNetworkByName)))
}

func (s *Server) handleNetworks(w http.ResponseWriter, r *http.Request) {
	if s.networks == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "network service is not configured")
		return
	}
	if !hasScope(r.Context(), store.ScopeRunsRead) {
		writeScopeError(w, store.ScopeRunsRead)
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"networks": s.networks.ListDefinitions()})
}

func (s *Server) handleNetworkByName(w http.ResponseWriter, r *http.Request) {
	if s.networks == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "network service is not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/networks/")
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
		def, ok := s.networks.GetDefinition(name)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("network %q not found", name))
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
		run, err := s.networks.Start(name, req.Input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"run_id": run.ID, "status": run.Status})
		return
	}
	http.NotFound(w, r)
}
