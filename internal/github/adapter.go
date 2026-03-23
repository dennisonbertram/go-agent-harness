package github

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"go-agent-harness/internal/trigger"
)

// GitHubAdapter converts GitHub HTTP webhook requests into ExternalTriggerEnvelopes.
type GitHubAdapter struct {
	// Secret is the GITHUB_WEBHOOK_SECRET used for HMAC-SHA256 validation.
	// The actual signature verification is performed by the GitHubValidator
	// in the trigger package; the adapter only plumbs the signature value
	// into the envelope.
	Secret string
}

// NewGitHubAdapter returns a new GitHubAdapter configured with the given HMAC secret.
func NewGitHubAdapter(secret string) *GitHubAdapter {
	return &GitHubAdapter{Secret: secret}
}

// ParseWebhookRequest reads GitHub-specific headers and body from an HTTP request
// and returns a populated ExternalTriggerEnvelope ready for the trigger routing logic.
//
// Required headers:
//   - X-GitHub-Event  — event type (e.g. "issues", "issue_comment")
//   - X-GitHub-Delivery — unique delivery UUID
//
// Optional but strongly recommended:
//   - X-Hub-Signature-256 — HMAC-SHA256 signature of the body
//
// The returned envelope has:
//   - Source     = "github"
//   - SourceID   = X-GitHub-Delivery value
//   - ThreadID   = issue/PR number as decimal string
//   - Action     = derived via DeriveAction (may be empty for unsupported events)
//   - Message    = composed via ComposeMessage
//   - Signature  = X-Hub-Signature-256 value (used by GitHubValidator)
//   - RawBody    = raw request body (required for HMAC verification)
func (a *GitHubAdapter) ParseWebhookRequest(r *http.Request) (*trigger.ExternalTriggerEnvelope, error) {
	eventType := strings.TrimSpace(r.Header.Get("X-GitHub-Event"))
	if eventType == "" {
		return nil, fmt.Errorf("missing X-GitHub-Event header")
	}

	deliveryID := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	if deliveryID == "" {
		return nil, fmt.Errorf("missing X-GitHub-Delivery header")
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	payload, err := ParseWebhookPayload(eventType, rawBody)
	if err != nil {
		return nil, err
	}

	action := DeriveAction(eventType, payload.Action)
	message := ComposeMessage(payload)

	// The signature goes into the envelope's Signature field.
	// The GitHubValidator in the trigger package reads env.Signature and
	// compares it against the HMAC-SHA256 of env.RawBody.
	signature := strings.TrimSpace(r.Header.Get("X-Hub-Signature-256"))

	env := &trigger.ExternalTriggerEnvelope{
		Source:    "github",
		SourceID:  deliveryID,
		RepoOwner: payload.RepoOwner,
		RepoName:  payload.RepoName,
		ThreadID:  fmt.Sprintf("%d", payload.IssueNumber),
		Action:    action,
		Message:   message,
		Signature: signature,
		RawBody:   rawBody,
	}

	return env, nil
}
