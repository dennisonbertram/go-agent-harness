package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"go-agent-harness/internal/profiles"
	"go-agent-harness/internal/store"
)

// handleProfilesRoot handles GET /v1/profiles.
func (s *Server) handleProfilesRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	names, err := profiles.ListProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if names == nil {
		names = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": names})
}

// handleProfileByName handles GET, POST, PUT, DELETE /v1/profiles/{name}.
func (s *Server) handleProfileByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/v1/profiles/")
	name = strings.TrimRight(name, "/")
	if name == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasScope(r.Context(), store.ScopeRunsRead) {
			writeScopeError(w, store.ScopeRunsRead)
			return
		}
		s.handleGetProfile(w, r, name)
	case http.MethodPost:
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		s.handleCreateProfile(w, r, name)
	case http.MethodPut:
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		s.handleUpdateProfile(w, r, name)
	case http.MethodDelete:
		if !hasScope(r.Context(), store.ScopeRunsWrite) {
			writeScopeError(w, store.ScopeRunsWrite)
			return
		}
		s.handleDeleteProfile(w, r, name)
	default:
		writeMethodNotAllowed(w, "GET, POST, PUT, DELETE")
	}
}

// handleGetProfile handles GET /v1/profiles/{name}.
func (s *Server) handleGetProfile(w http.ResponseWriter, _ *http.Request, name string) {
	p, err := profiles.LoadProfile(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// profileMutationRequest is the JSON body for POST and PUT /v1/profiles/{name}.
type profileMutationRequest struct {
	Description  string   `json:"description"`
	Model        string   `json:"model,omitempty"`
	MaxSteps     int      `json:"max_steps,omitempty"`
	MaxCostUSD   float64  `json:"max_cost_usd,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

// handleCreateProfile handles POST /v1/profiles/{name}.
func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request, name string) {
	if s.profilesDir == "" {
		writeError(w, http.StatusNotImplemented, "not_configured", "profiles directory not configured")
		return
	}

	// Reject built-in profile names.
	if profiles.IsBuiltinProfile(name) {
		writeError(w, http.StatusConflict, "builtin_protected", "profile "+name+" is a built-in and cannot be created via this API")
		return
	}

	var req profileMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
		return
	}

	p := buildProfileFromRequest(name, req)
	if err := profiles.ValidateProfile(p); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := profiles.SaveProfileToDir(p, s.profilesDir); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "created",
		"name":   name,
	})
}

// handleUpdateProfile handles PUT /v1/profiles/{name}.
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request, name string) {
	if s.profilesDir == "" {
		writeError(w, http.StatusNotImplemented, "not_configured", "profiles directory not configured")
		return
	}

	// Check if a user file exists in profilesDir (stat directly, no fallback to builtins).
	userFile := s.profilesDir + "/" + name + ".toml"
	if _, statErr := os.Stat(userFile); os.IsNotExist(statErr) {
		// No user file. If it's a builtin, return 403. Otherwise 404.
		if profiles.IsBuiltinProfile(name) {
			writeError(w, http.StatusForbidden, "builtin_protected", "profile "+name+" is a built-in and cannot be modified")
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "profile not found in user profiles directory")
		return
	}

	// Load the existing user profile (user dir only, no fallback to builtins).
	existing, err := profiles.LoadProfileWithDirs(name, "", s.profilesDir)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "profile not found in user profiles directory")
		return
	}

	var req profileMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
		return
	}

	// Apply updates.
	if req.Description != "" {
		existing.Meta.Description = req.Description
	}
	if req.Model != "" {
		existing.Runner.Model = req.Model
	}
	if req.MaxSteps > 0 {
		existing.Runner.MaxSteps = req.MaxSteps
	}
	if req.MaxCostUSD > 0 {
		existing.Runner.MaxCostUSD = req.MaxCostUSD
	}
	if req.SystemPrompt != "" {
		existing.Runner.SystemPrompt = req.SystemPrompt
	}
	if req.AllowedTools != nil {
		existing.Tools.Allow = req.AllowedTools
	}

	if err := profiles.ValidateProfile(existing); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := profiles.SaveProfileToDir(existing, s.profilesDir); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "updated",
		"name":   name,
	})
}

// handleDeleteProfile handles DELETE /v1/profiles/{name}.
func (s *Server) handleDeleteProfile(w http.ResponseWriter, _ *http.Request, name string) {
	if s.profilesDir == "" {
		writeError(w, http.StatusNotImplemented, "not_configured", "profiles directory not configured")
		return
	}

	err := profiles.DeleteProfileFromDir(name, s.profilesDir)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "built-in") {
			writeError(w, http.StatusForbidden, "builtin_protected", errStr)
			return
		}
		if strings.Contains(errStr, "not found") {
			writeError(w, http.StatusNotFound, "not_found", errStr)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", errStr)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "deleted",
		"name":   name,
	})
}

// buildProfileFromRequest constructs a Profile from a mutation request.
func buildProfileFromRequest(name string, req profileMutationRequest) *profiles.Profile {
	return &profiles.Profile{
		Meta: profiles.ProfileMeta{
			Name:           name,
			Description:    req.Description,
			Version:        1,
			CreatedBy:      "api",
			ReviewEligible: true,
		},
		Runner: profiles.ProfileRunner{
			Model:        req.Model,
			MaxSteps:     req.MaxSteps,
			MaxCostUSD:   req.MaxCostUSD,
			SystemPrompt: req.SystemPrompt,
		},
		Tools: profiles.ProfileTools{
			Allow: req.AllowedTools,
		},
	}
}
