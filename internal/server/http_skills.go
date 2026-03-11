package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go-agent-harness/internal/harness/tools"
)

// handleSkillsRoot handles GET /v1/skills.
func (s *Server) handleSkillsRoot(w http.ResponseWriter, r *http.Request) {
	if s.skills == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "skills not configured")
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	s.handleListSkills(w, r)
}

// handleSkillByName handles GET /v1/skills/{name} and POST /v1/skills/{name}/verify.
func (s *Server) handleSkillByName(w http.ResponseWriter, r *http.Request) {
	if s.skills == nil {
		writeError(w, http.StatusNotImplemented, "not_configured", "skills not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/skills/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.SplitN(path, "/", 2)
	name := parts[0]

	if len(parts) == 2 {
		switch parts[1] {
		case "verify":
			s.handleVerifySkill(w, r, name)
		default:
			http.NotFound(w, r)
		}
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	s.handleGetSkill(w, r, name)
}

// handleListSkills handles GET /v1/skills.
func (s *Server) handleListSkills(w http.ResponseWriter, _ *http.Request) {
	skills := s.skills.ListSkills()
	if skills == nil {
		skills = []tools.SkillInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": skills})
}

// handleGetSkill handles GET /v1/skills/{name}.
func (s *Server) handleGetSkill(w http.ResponseWriter, _ *http.Request, name string) {
	skill, ok := s.skills.GetSkill(name)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "skill not found")
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

// handleVerifySkill handles POST /v1/skills/{name}/verify.
func (s *Server) handleVerifySkill(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	_, ok := s.skills.GetSkill(name)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "skill not found")
		return
	}

	var req struct {
		VerifiedBy string `json:"verified_by"`
	}
	// Body is optional — ignore decode errors for empty body.
	_ = json.NewDecoder(r.Body).Decode(&req)

	verifiedBy := req.VerifiedBy
	if verifiedBy == "" {
		verifiedBy = "api"
	}

	if err := s.skills.UpdateSkillVerification(r.Context(), name, true, time.Now().UTC(), verifiedBy); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	skill, ok := s.skills.GetSkill(name)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "skill disappeared after verification")
		return
	}
	writeJSON(w, http.StatusOK, skill)
}
