package trigger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ValidatorRegistry maps source names to their signature validators.
type ValidatorRegistry struct {
	validators map[string]ExternalThreadValidator
}

// NewValidatorRegistry returns an empty ValidatorRegistry.
func NewValidatorRegistry() *ValidatorRegistry {
	return &ValidatorRegistry{validators: make(map[string]ExternalThreadValidator)}
}

// Register adds a validator for the given source name.
// Source names are normalized to lowercase.
func (r *ValidatorRegistry) Register(source string, v ExternalThreadValidator) {
	r.validators[strings.ToLower(source)] = v
}

// Get retrieves the validator for source. The bool reports whether one was found.
// Source names are normalized to lowercase.
func (r *ValidatorRegistry) Get(source string) (ExternalThreadValidator, bool) {
	v, ok := r.validators[strings.ToLower(source)]
	return v, ok
}

// GitHubValidator validates GitHub webhook HMAC-SHA256 signatures.
// The signature is expected in the X-Hub-Signature-256 header format: "sha256=<hex>".
type GitHubValidator struct {
	Secret string
}

// ValidateSignature verifies that env.Signature matches the HMAC-SHA256 of env.RawBody
// using v.Secret as the key. The expected format is "sha256=<hex>".
func (v *GitHubValidator) ValidateSignature(_ context.Context, env ExternalTriggerEnvelope) error {
	if v.Secret == "" {
		return fmt.Errorf("github webhook secret not configured")
	}
	mac := hmac.New(sha256.New, []byte(v.Secret))
	mac.Write(env.RawBody)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(env.Signature), []byte(expected)) != 1 {
		return fmt.Errorf("invalid github webhook signature")
	}
	return nil
}

// SlackValidator validates Slack webhook HMAC-SHA256 signatures with timestamp
// freshness enforcement (±5 minutes).
//
// Slack packs both the timestamp and the signature into env.Signature using the
// format "timestamp:v0=<hex>" so that a single envelope field carries all of the
// validation material without requiring extra HTTP-header plumbing.
type SlackValidator struct {
	Secret string
	// nowFunc is injectable for testing; defaults to time.Now.
	nowFunc func() time.Time
}

// ValidateSignature verifies the Slack request signature.
// env.Signature must be in the format "<unix_timestamp>:v0=<hex>".
func (v *SlackValidator) ValidateSignature(_ context.Context, env ExternalTriggerEnvelope) error {
	if v.Secret == "" {
		return fmt.Errorf("slack signing secret not configured")
	}
	// Parse timestamp from Signature field (packed as "timestamp:sig" for Slack).
	parts := strings.SplitN(env.Signature, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid slack signature format (expected timestamp:sig)")
	}
	tsStr, sig := parts[0], parts[1]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid slack timestamp: %w", err)
	}
	now := time.Now
	if v.nowFunc != nil {
		now = v.nowFunc
	}
	if absInt64(now().Unix()-ts) > 300 {
		return fmt.Errorf("slack request timestamp expired")
	}
	basestring := fmt.Sprintf("v0:%d:%s", ts, string(env.RawBody))
	mac := hmac.New(sha256.New, []byte(v.Secret))
	mac.Write([]byte(basestring))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return fmt.Errorf("invalid slack signature")
	}
	return nil
}

// LinearValidator validates Linear webhook HMAC-SHA256 signatures.
// The signature is the raw hex-encoded HMAC-SHA256 of the body.
type LinearValidator struct {
	Secret string
}

// ValidateSignature verifies that env.Signature is the HMAC-SHA256 hex of env.RawBody.
func (v *LinearValidator) ValidateSignature(_ context.Context, env ExternalTriggerEnvelope) error {
	if v.Secret == "" {
		return fmt.Errorf("linear webhook secret not configured")
	}
	mac := hmac.New(sha256.New, []byte(v.Secret))
	mac.Write(env.RawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(env.Signature), []byte(expected)) != 1 {
		return fmt.Errorf("invalid linear webhook signature")
	}
	return nil
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
