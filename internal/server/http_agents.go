package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// agentRunnerIface is a local interface to avoid importing internal/harness/tools,
// which would create an import cycle through internal/harness.
type agentRunnerIface interface {
	RunPrompt(ctx context.Context, prompt string) (string, error)
}

// forkedAgentRunnerIface extends agentRunnerIface with skill-based forked execution.
type forkedAgentRunnerIface interface {
	agentRunnerIface
	RunForkedSkill(ctx context.Context, config agentForkConfig) (agentForkResult, error)
}

// agentForkConfig holds configuration for a forked skill execution.
type agentForkConfig struct {
	Prompt       string
	SkillName    string
	SkillArgs    string
	AllowedTools []string
}

// agentForkResult holds the output from a forked skill execution.
type agentForkResult struct {
	Output  string
	Summary string
	Error   string
}

// skillListerIface supports resolving skill content for the fallback path.
type skillListerIface interface {
	ResolveSkill(ctx context.Context, name, args, workspace string) (string, error)
}

const (
	defaultAgentTimeoutSeconds = 120
	maxAgentTimeoutSeconds     = 600
)

// agentRequest is the JSON body for POST /v1/agents.
type agentRequest struct {
	Prompt         string   `json:"prompt"`
	Skill          string   `json:"skill"`
	SkillArgs      string   `json:"skill_args"`
	AllowedTools   []string `json:"allowed_tools"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

// agentResponse is the JSON body returned by POST /v1/agents.
type agentResponse struct {
	Output     string `json:"output"`
	Summary    string `json:"summary"`
	DurationMs int64  `json:"duration_ms"`
}

// handleAgents handles POST /v1/agents.
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	if s.agentRunner == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "agent runner is not configured")
		return
	}

	var req agentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	hasPrompt := strings.TrimSpace(req.Prompt) != ""
	hasSkill := strings.TrimSpace(req.Skill) != ""

	if !hasPrompt && !hasSkill {
		writeError(w, http.StatusBadRequest, "invalid_request", "either prompt or skill is required")
		return
	}
	if hasPrompt && hasSkill {
		writeError(w, http.StatusBadRequest, "invalid_request", "prompt and skill are mutually exclusive")
		return
	}

	// Determine timeout.
	timeoutSecs := req.TimeoutSeconds
	if timeoutSecs <= 0 {
		timeoutSecs = defaultAgentTimeoutSeconds
	}
	if timeoutSecs > maxAgentTimeoutSeconds {
		timeoutSecs = maxAgentTimeoutSeconds
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	start := time.Now()

	var output, summary string

	if hasPrompt {
		result, err := s.agentRunner.RunPrompt(ctx, req.Prompt)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				writeError(w, http.StatusRequestTimeout, "timeout", "agent execution timed out")
				return
			}
			writeError(w, http.StatusInternalServerError, "execution_error", err.Error())
			return
		}
		output = result
	} else {
		// Skill-based execution.
		if s.forkedAgentRunner != nil {
			config := agentForkConfig{
				SkillName:    req.Skill,
				SkillArgs:    req.SkillArgs,
				AllowedTools: req.AllowedTools,
			}
			result, err := s.forkedAgentRunner.RunForkedSkill(ctx, config)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					writeError(w, http.StatusRequestTimeout, "timeout", "agent execution timed out")
					return
				}
				// Treat "not found" in the error message as a 404.
				if strings.Contains(err.Error(), "not found") {
					writeError(w, http.StatusNotFound, "skill_not_found", err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, "execution_error", err.Error())
				return
			}
			output = result.Output
			summary = result.Summary
		} else if s.skillLister != nil {
			// Fallback: resolve skill content then run as a plain prompt.
			content, err := s.skillLister.ResolveSkill(ctx, req.Skill, req.SkillArgs, "")
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					writeError(w, http.StatusRequestTimeout, "timeout", "agent execution timed out")
					return
				}
				if strings.Contains(err.Error(), "not found") {
					writeError(w, http.StatusNotFound, "skill_not_found", err.Error())
					return
				}
				writeError(w, http.StatusInternalServerError, "execution_error", err.Error())
				return
			}
			result, err := s.agentRunner.RunPrompt(ctx, content)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					writeError(w, http.StatusRequestTimeout, "timeout", "agent execution timed out")
					return
				}
				writeError(w, http.StatusInternalServerError, "execution_error", err.Error())
				return
			}
			output = result
		} else {
			writeError(w, http.StatusNotImplemented, "not_implemented", "skill execution requires a forked agent runner or skill lister")
			return
		}
	}

	durationMs := time.Since(start).Milliseconds()

	writeJSON(w, http.StatusOK, agentResponse{
		Output:     output,
		Summary:    summary,
		DurationMs: durationMs,
	})
}
