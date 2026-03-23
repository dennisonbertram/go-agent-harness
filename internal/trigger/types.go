package trigger

import "context"

// ExternalTriggerEnvelope is a normalized incoming trigger from any external source.
type ExternalTriggerEnvelope struct {
	Source    string `json:"source"`     // "github" | "slack" | "linear"
	SourceID  string `json:"source_id"`  // webhook delivery ID or event ID
	RepoOwner string `json:"repo_owner"` // e.g. "anthropic"
	RepoName  string `json:"repo_name"`  // e.g. "go-agent-harness"
	ThreadID  string `json:"thread_id"`  // PR#, issue#, thread TS, Linear issue key
	Action    string `json:"action"`     // "start" | "steer" | "continue"
	Message   string `json:"message"`    // user-supplied prompt text
	TenantID  string `json:"tenant_id"`  // "" defaults to "default"
	AgentID   string `json:"agent_id"`   // "" defaults to "default"
	Signature string `json:"signature"`  // source-specific HMAC signature
	RawBody   []byte `json:"-"`          // raw request body for sig validation (not serialized)
}

// ExternalThreadID is a stable, deterministic conversation ID derived from external identifiers.
type ExternalThreadID string

// String returns the string form of the thread ID.
func (e ExternalThreadID) String() string { return string(e) }

// ExternalThreadValidator validates webhook signatures for an external source.
type ExternalThreadValidator interface {
	ValidateSignature(ctx context.Context, env ExternalTriggerEnvelope) error
}
