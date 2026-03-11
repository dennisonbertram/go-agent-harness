package symphd

import (
	"encoding/json"
	"net/http"
)

// NewHandler returns an http.Handler for the symphd API.
func NewHandler(o *Orchestrator) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/state", handleState(o))
	mux.HandleFunc("GET /api/v1/issues", handleIssues(o))
	mux.HandleFunc("POST /api/v1/refresh", handleRefresh(o))
	mux.HandleFunc("GET /api/v1/dead-letters", handleDeadLetters(o))
	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func handleState(o *Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"state":  o.State(),
		})
	}
}

func handleIssues(o *Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"issues": o.Issues(),
		})
	}
}

func handleRefresh(o *Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := o.Refresh(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"message": "refresh queued",
		})
	}
}

func handleDeadLetters(o *Orchestrator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items := o.DeadLetters()
		if items == nil {
			items = []*DeadLetter{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":       "ok",
			"dead_letters": items,
		})
	}
}
