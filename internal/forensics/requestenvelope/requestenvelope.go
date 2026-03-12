// Package requestenvelope provides types and helpers for capturing LLM request
// and response metadata for forensic/observability purposes.
package requestenvelope

import (
	"crypto/sha256"
	"encoding/hex"
)

// RequestSnapshot captures the key inputs to an LLM provider call.
// It is designed to be compact: the full prompt text is hashed (not stored)
// to avoid bloat, while tool names and memory snippets are stored verbatim
// as they are typically short.
type RequestSnapshot struct {
	// Step is the 1-based step number within the run.
	Step int `json:"step"`
	// PromptHash is the SHA-256 hex digest of the concatenated system prompt
	// and message contents sent to the provider. Used for deduplication and
	// change detection without storing PII.
	PromptHash string `json:"prompt_hash"`
	// ToolNames is the list of tool names (not full schemas) sent to the provider.
	ToolNames []string `json:"tool_names"`
	// MemorySnippet is the memory context injected into the turn (empty if none).
	MemorySnippet string `json:"memory_snippet,omitempty"`
}

// ResponseMeta captures provider metadata returned after an LLM provider call.
type ResponseMeta struct {
	// Step is the 1-based step number within the run.
	Step int `json:"step"`
	// LatencyMS is the wall-clock time from provider request start to full response,
	// in milliseconds.
	LatencyMS int64 `json:"latency_ms"`
	// ModelVersion is the specific model version string returned by the provider,
	// if available. Empty when the provider does not report this.
	ModelVersion string `json:"model_version,omitempty"`
}

// HashPrompt computes a SHA-256 hex digest of the given prompt string.
// The result is always a 64-character lowercase hex string.
func HashPrompt(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(h[:])
}
