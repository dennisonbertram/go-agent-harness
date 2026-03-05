package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go-agent-harness/internal/harness"
)

func New(runner *harness.Runner) http.Handler {
	s := &Server{runner: runner}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/runs", s.handleRuns)
	mux.HandleFunc("/v1/runs/", s.handleRunByID)
	return mux
}

type Server struct {
	runner *harness.Runner
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

	http.NotFound(w, r)
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
		if isTerminalEvent(event.Type) {
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
			if isTerminalEvent(event.Type) {
				return
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, event harness.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
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

func isTerminalEvent(eventType string) bool {
	return eventType == "run.completed" || eventType == "run.failed"
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
