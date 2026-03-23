package slack

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"go-agent-harness/internal/trigger"
)

// SlackAdapter converts Slack HTTP webhook requests into ExternalTriggerEnvelopes.
// Signature validation is performed by the SlackValidator in the trigger package;
// the adapter only plumbs the packed "timestamp:signature" string into the envelope.
type SlackAdapter struct{}

// NewSlackAdapter returns a new SlackAdapter.
func NewSlackAdapter() *SlackAdapter {
	return &SlackAdapter{}
}

// ParseWebhookRequest reads Slack headers and body from an HTTP request and returns
// a populated ExternalTriggerEnvelope ready for the trigger routing logic.
//
// Required headers:
//   - X-Slack-Request-Timestamp — unix timestamp of the request
//   - X-Slack-Signature         — Slack's HMAC-SHA256 signature ("v0=<hex>")
//
// The returned envelope has:
//   - Source     = "slack"
//   - SourceID   = event_id from the payload
//   - ThreadID   = channelID + ":" + threadTS (or channelID + ":" + messageTS for top-level)
//   - Action     = "steer" (dispatcher decides routing based on run state)
//   - Message    = composed via ComposeMessage
//   - Signature  = "<unix_timestamp>:v0=<hex>" (packed format required by SlackValidator)
//   - RawBody    = raw request body (required for HMAC verification)
func (a *SlackAdapter) ParseWebhookRequest(r *http.Request) (*trigger.ExternalTriggerEnvelope, error) {
	timestamp := strings.TrimSpace(r.Header.Get("X-Slack-Request-Timestamp"))
	if timestamp == "" {
		return nil, fmt.Errorf("missing X-Slack-Request-Timestamp header")
	}

	sig := strings.TrimSpace(r.Header.Get("X-Slack-Signature"))
	if sig == "" {
		return nil, fmt.Errorf("missing X-Slack-Signature header")
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	payload, err := ParseWebhookPayload(rawBody)
	if err != nil {
		return nil, err
	}

	// Derive thread ID: prefer thread_ts for threaded messages, fall back to message ts.
	tsForThread := payload.ThreadTS
	if tsForThread == "" {
		tsForThread = payload.MessageTS
	}
	threadID := payload.ChannelID + ":" + tsForThread

	message := ComposeMessage(payload)

	// Pack timestamp + Slack signature into envelope.Signature in the format
	// "<unix_timestamp>:v0=<hex>" as required by SlackValidator.ValidateSignature.
	packedSig := timestamp + ":" + sig

	env := &trigger.ExternalTriggerEnvelope{
		Source:    "slack",
		SourceID:  payload.EventID,
		ThreadID:  threadID,
		Action:    "steer",
		Message:   message,
		Signature: packedSig,
		RawBody:   rawBody,
	}

	return env, nil
}
