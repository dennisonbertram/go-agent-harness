package server

import (
	"net/http"
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
	if !hasScope(r.Context(), store.ScopeRunsRead) {
		writeScopeError(w, store.ScopeRunsRead)
		return
	}
	s.handleListProfiles(w, r)
}

// handleProfileByName handles GET /v1/profiles/{name}.
func (s *Server) handleProfileByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/v1/profiles/")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if !hasScope(r.Context(), store.ScopeRunsRead) {
		writeScopeError(w, store.ScopeRunsRead)
		return
	}
	s.handleGetProfile(w, r, name)
}

// handleListProfiles returns all profiles across all three tiers.
func (s *Server) handleListProfiles(w http.ResponseWriter, _ *http.Request) {
	summaries, err := profiles.ListProfileSummariesFromDirs(s.profilesProject, s.profilesUser)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if summaries == nil {
		summaries = []profiles.ProfileSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profiles": summaries,
		"count":    len(summaries),
	})
}

// handleGetProfile returns a single profile by name.
func (s *Server) handleGetProfile(w http.ResponseWriter, _ *http.Request, name string) {
	var p *profiles.Profile
	var err error

	if s.profilesProject != "" || s.profilesUser != "" {
		p, err = profiles.LoadProfileWithDirs(name, s.profilesProject, s.profilesUser)
	} else {
		p, err = profiles.LoadProfile(name)
	}

	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "profile not found: "+name)
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "not_found", "profile not found: "+name)
		return
	}

	// Determine source tier.
	sourceTier := s.resolveProfileSourceTier(name)

	response := map[string]any{
		"name":               p.Meta.Name,
		"description":        p.Meta.Description,
		"version":            p.Meta.Version,
		"model":              p.Runner.Model,
		"max_steps":          p.Runner.MaxSteps,
		"max_cost_usd":       p.Runner.MaxCostUSD,
		"allowed_tools":      p.Tools.Allow,
		"allowed_tool_count": len(p.Tools.Allow),
		"source_tier":        sourceTier,
		"created_by":         p.Meta.CreatedBy,
	}
	if p.Runner.SystemPrompt != "" {
		response["system_prompt"] = p.Runner.SystemPrompt
	}
	writeJSON(w, http.StatusOK, response)
}

// resolveProfileSourceTier returns the source tier for a named profile.
func (s *Server) resolveProfileSourceTier(name string) string {
	if s.profilesProject != "" {
		if p, err := profiles.LoadProfileWithDirs(name, s.profilesProject, ""); err == nil && p != nil {
			return "project"
		}
	}
	if s.profilesUser != "" {
		if p, err := profiles.LoadProfileWithDirs(name, "", s.profilesUser); err == nil && p != nil {
			return "user"
		}
	}
	return "built-in"
}
