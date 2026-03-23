package linear

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"go-agent-harness/internal/trigger"
)

// LinearAdapter converts Linear HTTP webhook requests into ExternalTriggerEnvelopes.
// Signature validation is performed by the LinearValidator in the trigger package;
// the adapter only plumbs the raw hex HMAC into the envelope.
type LinearAdapter struct{}

// NewLinearAdapter returns a new LinearAdapter.
func NewLinearAdapter() *LinearAdapter {
	return &LinearAdapter{}
}

// ParseWebhookRequest reads Linear headers and body from an HTTP request and returns
// a populated ExternalTriggerEnvelope ready for the trigger routing logic.
//
// The returned envelope has:
//   - Source     = "linear"
//   - SourceID   = payload.Identifier (e.g. "ENG-123") or empty if unavailable
//   - ThreadID   = payload.Identifier (stable per issue — empty falls back to IssueID)
//   - Action     = derived via DeriveAction (may be empty for unsupported events)
//   - Message    = composed via ComposeMessage
//   - Signature  = X-Linear-Signature value (raw hex HMAC, used by LinearValidator)
//   - RawBody    = raw request body (required for HMAC verification)
func (a *LinearAdapter) ParseWebhookRequest(r *http.Request) (*trigger.ExternalTriggerEnvelope, error) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	payload, err := ParseWebhookPayload(rawBody)
	if err != nil {
		return nil, err
	}

	sig := strings.TrimSpace(r.Header.Get("X-Linear-Signature"))

	action := DeriveAction(payload.EventType, payload.Action)
	message := ComposeMessage(payload)

	// ThreadID: prefer the stable issue identifier (e.g. "ENG-123") as it is
	// deterministic across all events for the same issue. Fall back to IssueID.
	threadID := payload.Identifier
	if threadID == "" {
		threadID = payload.IssueID
	}

	// SourceID: use identifier when available for human readability.
	sourceID := payload.Identifier
	if sourceID == "" {
		sourceID = payload.IssueID
	}

	env := &trigger.ExternalTriggerEnvelope{
		Source:    "linear",
		SourceID:  sourceID,
		ThreadID:  threadID,
		Action:    action,
		Message:   message,
		Signature: sig,
		RawBody:   rawBody,
	}

	return env, nil
}
