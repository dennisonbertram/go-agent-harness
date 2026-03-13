package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"go-agent-harness/internal/forensics/replay"
	"go-agent-harness/internal/forensics/rollout"
	"go-agent-harness/internal/harness"
)

// replayRequest is the JSON body for POST /v1/runs/replay.
type replayRequest struct {
	RolloutPath string `json:"rollout_path"`
	Mode        string `json:"mode"`      // "simulate" | "fork"
	ForkStep    int    `json:"fork_step"`  // required when mode=fork
}

// handleRunReplay handles POST /v1/runs/replay.
// mode=simulate: replays the rollout offline, returns a JSON summary.
// mode=fork: reconstructs conversation history up to fork_step, starts a new
// live run with that history, and returns the new run ID.
func (s *Server) handleRunReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req replayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	if strings.TrimSpace(req.RolloutPath) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "rollout_path is required")
		return
	}

	switch req.Mode {
	case "simulate":
		s.handleReplaySimulate(w, req)
	case "fork":
		s.handleReplayFork(w, r, req)
	default:
		writeError(w, http.StatusBadRequest, "invalid_request",
			fmt.Sprintf("mode must be \"simulate\" or \"fork\", got %q", req.Mode))
	}
}

// handleReplaySimulate runs an offline replay simulation.
func (s *Server) handleReplaySimulate(w http.ResponseWriter, req replayRequest) {
	events, err := loadRolloutFile(req.RolloutPath)
	if err != nil {
		writeRolloutError(w, err)
		return
	}

	result := replay.Replay(events)

	writeJSON(w, http.StatusOK, map[string]any{
		"mode":            "simulate",
		"events_replayed": len(result.Events),
		"step_count":      result.StepCount,
		"matched":         result.Matched,
		"mismatches":      result.Mismatches,
	})
}

// handleReplayFork reconstructs conversation history and starts a new run.
func (s *Server) handleReplayFork(w http.ResponseWriter, r *http.Request, req replayRequest) {
	events, err := loadRolloutFile(req.RolloutPath)
	if err != nil {
		writeRolloutError(w, err)
		return
	}

	forkResult, err := replay.Fork(events, req.ForkStep, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if len(forkResult.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "no messages reconstructed from rollout")
		return
	}

	// Extract a prompt from the last user message for StartRun.
	prompt := extractLastUserPrompt(forkResult.Messages)

	// Populate InitiatorAPIKeyPrefix from auth context for audit trail.
	run, err := s.runner.StartRun(harness.RunRequest{
		Prompt:            prompt,
		InitiatorAPIKeyPrefix: APIKeyPrefixFromContext(r.Context()),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "replay_error",
			fmt.Sprintf("failed to start forked run: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"mode":                "fork",
		"run_id":              run.ID,
		"from_step":           forkResult.FromStep,
		"original_step_count": forkResult.OriginalStepCount,
		"original_outcome":    forkResult.OriginalOutcome,
		"messages_restored":   len(forkResult.Messages),
	})
}

// loadRolloutFile loads and returns rollout events, returning a descriptive
// error if the file cannot be found or parsed.
func loadRolloutFile(path string) ([]rollout.RolloutEvent, error) {
	events, err := rollout.LoadFile(path)
	if err != nil {
		return nil, err
	}
	return events, nil
}

// writeRolloutError writes an appropriate HTTP error based on the rollout
// loading error type.
func writeRolloutError(w http.ResponseWriter, err error) {
	if os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "rollout_not_found", err.Error())
		return
	}
	// Check for wrapped os.ErrNotExist from rollout.LoadFile.
	if pathErr := (*os.PathError)(nil); errors.As(err, &pathErr) {
		writeError(w, http.StatusNotFound, "rollout_not_found", err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "replay_error", err.Error())
}

// extractLastUserPrompt finds the last user message content to use as the
// StartRun prompt.
func extractLastUserPrompt(msgs []harness.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return "forked run"
}
