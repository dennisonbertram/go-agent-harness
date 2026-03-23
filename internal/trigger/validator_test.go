package trigger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

// --- helpers ---

func githubSig(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func slackSig(secret string, ts int64, body string) string {
	basestring := fmt.Sprintf("v0:%d:%s", ts, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(basestring))
	return fmt.Sprintf("%d:v0=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func linearSig(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// --- GitHubValidator ---

func TestGitHubValidator_ValidSignature(t *testing.T) {
	t.Parallel()
	body := `{"action":"opened"}`
	secret := "my-github-secret"
	v := &GitHubValidator{Secret: secret}
	env := ExternalTriggerEnvelope{
		Source:    "github",
		RawBody:   []byte(body),
		Signature: githubSig(secret, body),
	}
	if err := v.ValidateSignature(context.Background(), env); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}
}

func TestGitHubValidator_InvalidSignature(t *testing.T) {
	t.Parallel()
	body := `{"action":"opened"}`
	v := &GitHubValidator{Secret: "correct-secret"}
	env := ExternalTriggerEnvelope{
		Source:    "github",
		RawBody:   []byte(body),
		Signature: githubSig("wrong-secret", body),
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error for invalid signature, got nil")
	}
}

func TestGitHubValidator_NoSecret(t *testing.T) {
	t.Parallel()
	v := &GitHubValidator{Secret: ""}
	env := ExternalTriggerEnvelope{
		Source:    "github",
		RawBody:   []byte(`{}`),
		Signature: "sha256=anything",
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error when secret is empty, got nil")
	}
}

func TestGitHubValidator_TamperedBody(t *testing.T) {
	t.Parallel()
	secret := "secret"
	originalBody := `{"action":"opened"}`
	tamperedBody := `{"action":"deleted"}`
	v := &GitHubValidator{Secret: secret}
	env := ExternalTriggerEnvelope{
		Source:    "github",
		RawBody:   []byte(tamperedBody),
		Signature: githubSig(secret, originalBody), // sig for original body
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error for tampered body, got nil")
	}
}

// --- SlackValidator ---

func TestSlackValidator_ValidSignature(t *testing.T) {
	t.Parallel()
	body := `payload=test`
	secret := "slack-signing-secret"
	ts := time.Now().Unix()
	v := &SlackValidator{Secret: secret}
	env := ExternalTriggerEnvelope{
		Source:    "slack",
		RawBody:   []byte(body),
		Signature: slackSig(secret, ts, body),
	}
	if err := v.ValidateSignature(context.Background(), env); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}
}

func TestSlackValidator_InvalidSignature(t *testing.T) {
	t.Parallel()
	body := `payload=test`
	ts := time.Now().Unix()
	v := &SlackValidator{Secret: "correct-secret"}
	env := ExternalTriggerEnvelope{
		Source:    "slack",
		RawBody:   []byte(body),
		Signature: slackSig("wrong-secret", ts, body),
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error for invalid signature, got nil")
	}
}

func TestSlackValidator_ExpiredTimestamp(t *testing.T) {
	t.Parallel()
	body := `payload=test`
	secret := "slack-signing-secret"
	// Timestamp is 10 minutes in the past — beyond the 5-minute window.
	ts := time.Now().Add(-10 * time.Minute).Unix()
	v := &SlackValidator{Secret: secret}
	env := ExternalTriggerEnvelope{
		Source:    "slack",
		RawBody:   []byte(body),
		Signature: slackSig(secret, ts, body),
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error for expired timestamp, got nil")
	}
	if err := v.ValidateSignature(context.Background(), env); !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected 'expired' in error, got: %v", err)
	}
}

func TestSlackValidator_NoSecret(t *testing.T) {
	t.Parallel()
	v := &SlackValidator{Secret: ""}
	env := ExternalTriggerEnvelope{
		Source:    "slack",
		RawBody:   []byte(`payload=test`),
		Signature: "1234567890:v0=abc",
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error when secret is empty, got nil")
	}
}

func TestSlackValidator_BadSignatureFormat(t *testing.T) {
	t.Parallel()
	v := &SlackValidator{Secret: "secret"}
	env := ExternalTriggerEnvelope{
		Source:    "slack",
		RawBody:   []byte(`payload=test`),
		Signature: "invalidsig-no-colon",
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error for bad signature format, got nil")
	}
}

func TestSlackValidator_FutureTimestamp(t *testing.T) {
	t.Parallel()
	body := `payload=test`
	secret := "slack-signing-secret"
	// Timestamp is 10 minutes in the future — also outside the 5-minute window.
	ts := time.Now().Add(10 * time.Minute).Unix()
	v := &SlackValidator{Secret: secret}
	env := ExternalTriggerEnvelope{
		Source:    "slack",
		RawBody:   []byte(body),
		Signature: slackSig(secret, ts, body),
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error for future timestamp, got nil")
	}
}

// --- LinearValidator ---

func TestLinearValidator_ValidSignature(t *testing.T) {
	t.Parallel()
	body := `{"action":"create"}`
	secret := "linear-webhook-secret"
	v := &LinearValidator{Secret: secret}
	env := ExternalTriggerEnvelope{
		Source:    "linear",
		RawBody:   []byte(body),
		Signature: linearSig(secret, body),
	}
	if err := v.ValidateSignature(context.Background(), env); err != nil {
		t.Errorf("expected valid signature, got error: %v", err)
	}
}

func TestLinearValidator_InvalidSignature(t *testing.T) {
	t.Parallel()
	body := `{"action":"create"}`
	v := &LinearValidator{Secret: "correct-secret"}
	env := ExternalTriggerEnvelope{
		Source:    "linear",
		RawBody:   []byte(body),
		Signature: linearSig("wrong-secret", body),
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error for invalid signature, got nil")
	}
}

func TestLinearValidator_NoSecret(t *testing.T) {
	t.Parallel()
	v := &LinearValidator{Secret: ""}
	env := ExternalTriggerEnvelope{
		Source:    "linear",
		RawBody:   []byte(`{}`),
		Signature: "deadbeef",
	}
	if err := v.ValidateSignature(context.Background(), env); err == nil {
		t.Error("expected error when secret is empty, got nil")
	}
}

// --- ValidatorRegistry ---

func TestValidatorRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()
	reg := NewValidatorRegistry()
	v := &GitHubValidator{Secret: "s"}
	reg.Register("github", v)
	got, ok := reg.Get("github")
	if !ok {
		t.Fatal("expected to find registered validator")
	}
	if got != v {
		t.Error("expected same validator instance")
	}
}

func TestValidatorRegistry_GetMissing(t *testing.T) {
	t.Parallel()
	reg := NewValidatorRegistry()
	_, ok := reg.Get("unknown-source")
	if ok {
		t.Error("expected false for missing source")
	}
}

func TestValidatorRegistry_CaseInsensitive(t *testing.T) {
	t.Parallel()
	reg := NewValidatorRegistry()
	v := &GitHubValidator{Secret: "s"}
	reg.Register("GitHub", v)

	got, ok := reg.Get("github")
	if !ok {
		t.Fatal("expected to find validator registered under 'GitHub' via lowercase lookup")
	}
	if got != v {
		t.Error("expected same validator instance")
	}

	got2, ok2 := reg.Get("GITHUB")
	if !ok2 {
		t.Fatal("expected to find validator registered under 'GitHub' via uppercase lookup")
	}
	if got2 != v {
		t.Error("expected same validator instance")
	}
}

func TestValidatorRegistry_MultipleValidators(t *testing.T) {
	t.Parallel()
	reg := NewValidatorRegistry()
	gh := &GitHubValidator{Secret: "gh"}
	sl := &SlackValidator{Secret: "sl"}
	li := &LinearValidator{Secret: "li"}
	reg.Register("github", gh)
	reg.Register("slack", sl)
	reg.Register("linear", li)

	for _, tc := range []struct {
		source string
		want   ExternalThreadValidator
	}{
		{"github", gh},
		{"slack", sl},
		{"linear", li},
	} {
		got, ok := reg.Get(tc.source)
		if !ok {
			t.Errorf("expected to find validator for %q", tc.source)
			continue
		}
		if got != tc.want {
			t.Errorf("validator for %q: got %T, want %T", tc.source, got, tc.want)
		}
	}
}
