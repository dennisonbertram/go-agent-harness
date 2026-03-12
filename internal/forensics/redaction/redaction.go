// Package redaction provides a configurable PII/secret redaction pipeline for
// forensic event payloads. It filters sensitive data (API keys, JWTs, passwords,
// connection strings, etc.) before events are written to JSONL rollouts or audit logs.
package redaction

import (
	"crypto/sha256"
	"fmt"
	"regexp"
)

// ---------------------------------------------------------------------------
// StorageMode
// ---------------------------------------------------------------------------

// StorageMode controls how an event payload is stored.
type StorageMode string

const (
	// StorageModeRedacted applies redaction patterns to string values (default).
	StorageModeRedacted StorageMode = "redacted"
	// StorageModeFull stores the payload without any modification.
	StorageModeFull StorageMode = "full"
	// StorageModeHashed replaces each string value with its SHA-256 hex digest.
	StorageModeHashed StorageMode = "hashed"
	// StorageModeNone drops the event entirely (Apply returns keep=false).
	StorageModeNone StorageMode = "none"
)

// ---------------------------------------------------------------------------
// EventClassConfig
// ---------------------------------------------------------------------------

// EventClassConfig maps event type strings to their StorageMode.
// Event types not present in the map default to StorageModeRedacted.
type EventClassConfig map[string]StorageMode

// ---------------------------------------------------------------------------
// Built-in regex patterns
// ---------------------------------------------------------------------------

// pattern pairs a compiled regex with the redaction label to insert.
type pattern struct {
	re    *regexp.Regexp
	label string
}

// builtinPatterns is the ordered list of built-in secret patterns.
// Each pattern matches a distinct secret type. The patterns are applied in
// order; the first match wins for a given substring.
var builtinPatterns = []pattern{
	// JWTs — three base64url segments separated by dots.
	{
		re:    regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
		label: "jwt",
	},
	// Private keys (PEM block header, possibly with key type name).
	{
		re:    regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
		label: "private_key",
	},
	// AWS access key IDs — always AKIA... (20 uppercase alphanumeric chars).
	{
		re:    regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		label: "aws_key",
	},
	// AWS secret access key — key=value forms where the value looks like an AWS secret.
	{
		re:    regexp.MustCompile(`(?i)aws_secret_access_key\s*[=:]\s*[A-Za-z0-9/+]{40}`),
		label: "aws_secret",
	},
	// Database / broker connection strings.
	{
		re:    regexp.MustCompile(`(?i)(postgres|postgresql|mysql|redis|mongodb|amqp|amqps)://[^\s"']+`),
		label: "connection_string",
	},
	// sk- prefixed API keys (OpenAI, Anthropic, Stripe, etc.)
	{
		re:    regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
		label: "api_key",
	},
	// Generic high-entropy API key value patterns in key=value or key: value forms.
	// Matches: api_key=<32+ hex/alphanum chars>, apikey: <value>, etc.
	{
		re:    regexp.MustCompile(`(?i)(?:api[_-]?key|secret[_-]?key|access[_-]?token|auth[_-]?token)\s*[=:]\s*["']?[A-Za-z0-9_/+.-]{32,}["']?`),
		label: "api_key",
	},
	// Bearer / Authorization header tokens.
	{
		re:    regexp.MustCompile(`(?i)(?:Bearer|Authorization:\s*Bearer)\s+[A-Za-z0-9_\-./+=]{20,}`),
		label: "bearer_token",
	},
}

// ---------------------------------------------------------------------------
// Redactor
// ---------------------------------------------------------------------------

// Redactor applies redaction patterns to text strings. It is safe for
// concurrent use; all state is immutable after construction.
type Redactor struct {
	patterns []pattern
}

// NewRedactor creates a Redactor with the built-in patterns plus any additional
// custom patterns. custom patterns use the label "custom".
func NewRedactor(custom []*regexp.Regexp) *Redactor {
	pats := make([]pattern, len(builtinPatterns), len(builtinPatterns)+len(custom))
	copy(pats, builtinPatterns)
	for _, re := range custom {
		pats = append(pats, pattern{re: re, label: "custom"})
	}
	return &Redactor{patterns: pats}
}

// Redact applies all patterns to text and replaces matches with
// [REDACTED:<label>] markers. It is safe for concurrent use.
func (r *Redactor) Redact(text string) string {
	if text == "" {
		return text
	}
	result := text
	for _, p := range r.patterns {
		result = p.re.ReplaceAllString(result, fmt.Sprintf("[REDACTED:%s]", p.label))
	}
	return result
}

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

// Pipeline combines a Redactor with per-event-type StorageMode configuration.
// It is safe for concurrent use.
type Pipeline struct {
	redactor *Redactor
	cfg      EventClassConfig
}

// NewPipeline creates a Pipeline using the given Redactor and EventClassConfig.
// A nil Redactor is replaced with a default Redactor using no custom patterns.
func NewPipeline(r *Redactor, cfg EventClassConfig) *Pipeline {
	if r == nil {
		r = NewRedactor(nil)
	}
	if cfg == nil {
		cfg = EventClassConfig{}
	}
	return &Pipeline{redactor: r, cfg: cfg}
}

// Apply processes an event payload according to the configured StorageMode for
// eventType. It returns the (possibly modified) payload and a boolean indicating
// whether the event should be kept (false means drop the event).
//
// The input payload is never mutated; a deep copy is made before modification.
func (p *Pipeline) Apply(eventType string, payload map[string]any) (map[string]any, bool) {
	mode, ok := p.cfg[eventType]
	if !ok {
		mode = StorageModeRedacted
	}

	if mode == StorageModeNone {
		return nil, false
	}

	if payload == nil {
		return map[string]any{}, true
	}

	switch mode {
	case StorageModeFull:
		// Return a shallow copy to avoid aliasing but preserve all values.
		return shallowCopy(payload), true
	case StorageModeHashed:
		return deepTransformStrings(payload, hashString), true
	default: // StorageModeRedacted
		return deepTransformStrings(payload, p.redactor.Redact), true
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func shallowCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// deepTransformStrings recursively walks a map, applying fn to every string
// value it encounters. It always returns a new map and never mutates the input.
func deepTransformStrings(m map[string]any, fn func(string) string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			out[k] = fn(val)
		case map[string]any:
			out[k] = deepTransformStrings(val, fn)
		default:
			out[k] = v
		}
	}
	return out
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// ---------------------------------------------------------------------------
// Sentinel helpers used by the runner integration.
// ---------------------------------------------------------------------------

// RedactPayload is a convenience wrapper: it applies the Pipeline and returns
// the processed payload plus whether the event should be kept.
// It is equivalent to calling p.Apply directly.
func RedactPayload(p *Pipeline, eventType string, payload map[string]any) (map[string]any, bool) {
	if p == nil {
		return payload, true
	}
	return p.Apply(eventType, payload)
}

