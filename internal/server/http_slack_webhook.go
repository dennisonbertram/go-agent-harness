package server

import (
	"net/http"
)

// handleSlackWebhook handles POST /v1/webhooks/slack.
//
// It reads Slack-specific headers (X-Slack-Request-Timestamp, X-Slack-Signature),
// converts the request into an ExternalTriggerEnvelope via the SlackAdapter, then
// delegates to the shared trigger dispatch logic for start/steer/continue routing.
//
// Authentication is performed via HMAC-SHA256 signature validation (Slack's
// X-Slack-Signature mechanism) rather than Bearer tokens, so this route
// bypasses the standard auth middleware.
//
// Response codes mirror handleExternalTrigger:
//
//	202 — request accepted
//	400 — missing required headers, unsupported event type, or empty action
//	401 — missing or invalid signature, or adapter not configured
//	404 — steer/continue but no run found for thread
//	409 — run state mismatch
//	501 — run persistence not configured
func (s *Server) handleSlackWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	if s.slackAdapter == nil {
		writeError(w, http.StatusUnauthorized, "invalid_signature", "Slack webhook adapter not configured")
		return
	}

	// Parse the Slack-specific request into a normalized trigger envelope.
	env, err := s.slackAdapter.ParseWebhookRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Validate that we could derive a meaningful action from the event.
	if env.Action == "" {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"unsupported Slack event/action combination; no trigger action could be derived")
		return
	}

	// Validate the HMAC signature using the registered slack validator.
	if s.validators == nil {
		writeError(w, http.StatusUnauthorized, "invalid_signature", "no validator registry configured")
		return
	}
	validator, ok := s.validators.Get(env.Source)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_signature", "no validator configured for source: "+env.Source)
		return
	}
	if err := validator.ValidateSignature(r.Context(), *env); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_signature", err.Error())
		return
	}

	// Delegate to the shared dispatch logic (thread ID derivation + run routing).
	s.dispatchTriggerEnvelope(w, r, env)
}
