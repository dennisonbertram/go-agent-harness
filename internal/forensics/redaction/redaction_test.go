package redaction_test

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/internal/forensics/redaction"
)

// ---------------------------------------------------------------------------
// Redactor tests
// ---------------------------------------------------------------------------

func TestRedactor_APIKey_Generic(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	inputs := []string{
		"api_key=sk-proj-abcdefghijklmnopqrstuvwxyz123456",
		"OPENAI_API_KEY=sk-abcdefABCDEF1234567890abcdefABCDEF12345678",
		"key: sk-ant-api03-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"Authorization: Bearer sk-1234567890abcdefghij1234567890abcdef1234",
	}
	for _, input := range inputs {
		got := r.Redact(input)
		if !strings.Contains(got, "[REDACTED:") {
			t.Errorf("expected redaction for %q, got %q", input, got)
		}
	}
}

func TestRedactor_JWT(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	// A minimal valid JWT format: header.payload.signature
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	got := r.Redact("token=" + jwt)
	if !strings.Contains(got, "[REDACTED:jwt]") {
		t.Errorf("expected [REDACTED:jwt], got %q", got)
	}
}

func TestRedactor_ConnectionString_Postgres(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cs := "postgres://user:password@localhost:5432/mydb"
	got := r.Redact(cs)
	if !strings.Contains(got, "[REDACTED:connection_string]") {
		t.Errorf("expected [REDACTED:connection_string], got %q", got)
	}
}

func TestRedactor_ConnectionString_MySQL(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cs := "mysql://root:secret@127.0.0.1:3306/production"
	got := r.Redact(cs)
	if !strings.Contains(got, "[REDACTED:connection_string]") {
		t.Errorf("expected [REDACTED:connection_string], got %q", got)
	}
}

func TestRedactor_ConnectionString_Redis(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cs := "redis://:password@localhost:6379/0"
	got := r.Redact(cs)
	if !strings.Contains(got, "[REDACTED:connection_string]") {
		t.Errorf("expected [REDACTED:connection_string], got %q", got)
	}
}

func TestRedactor_AWS_AccessKeyID(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	input := "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	got := r.Redact(input)
	if !strings.Contains(got, "[REDACTED:aws_key]") {
		t.Errorf("expected [REDACTED:aws_key], got %q", got)
	}
}

func TestRedactor_AWS_SecretAccessKey(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	input := "aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	got := r.Redact(input)
	if !strings.Contains(got, "[REDACTED:aws_secret]") {
		t.Errorf("expected [REDACTED:aws_secret], got %q", got)
	}
}

func TestRedactor_PrivateKey(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	input := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----"
	got := r.Redact(input)
	if !strings.Contains(got, "[REDACTED:private_key]") {
		t.Errorf("expected [REDACTED:private_key], got %q", got)
	}
}

func TestRedactor_BearerToken(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	input := "Authorization: Bearer eysome_opaque_token_here_1234567890abcdef"
	got := r.Redact(input)
	if !strings.Contains(got, "[REDACTED:") {
		t.Errorf("expected redaction for bearer token, got %q", got)
	}
}

func TestRedactor_CustomRegex(t *testing.T) {
	t.Parallel()
	custom := []*regexp.Regexp{
		regexp.MustCompile(`MYAPP_SECRET_[A-Z0-9]+`),
	}
	r := redaction.NewRedactor(custom)
	input := "config: MYAPP_SECRET_ABC123"
	got := r.Redact(input)
	if !strings.Contains(got, "[REDACTED:custom]") {
		t.Errorf("expected [REDACTED:custom], got %q", got)
	}
}

func TestRedactor_FalsePositives(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	safe := []string{
		"Hello, world!",
		"The quick brown fox jumps over the lazy dog.",
		"user: john, age: 30",
		"http://example.com/path?foo=bar",
		"function add(a, b) { return a + b; }",
		"2024-01-15T10:30:00Z",
		"Error: file not found at /tmp/test.txt",
		"step=3 event=run.completed status=completed",
	}
	for _, input := range safe {
		got := r.Redact(input)
		if strings.Contains(got, "[REDACTED:") {
			t.Errorf("false positive: %q redacted to %q", input, got)
		}
	}
}

func TestRedactor_EmptyString(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	got := r.Redact("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRedactor_MarkerFormat(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	got := r.Redact(jwt)
	if !strings.Contains(got, "[REDACTED:jwt]") {
		t.Errorf("expected [REDACTED:jwt] marker format, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// StorageMode tests
// ---------------------------------------------------------------------------

func TestStorageModeConstants(t *testing.T) {
	t.Parallel()
	// Verify the constants exist and have the expected string values.
	modes := map[redaction.StorageMode]string{
		redaction.StorageModeRedacted: "redacted",
		redaction.StorageModeFull:     "full",
		redaction.StorageModeHashed:   "hashed",
		redaction.StorageModeNone:     "none",
	}
	for mode, want := range modes {
		if string(mode) != want {
			t.Errorf("StorageMode %v: got %q, want %q", mode, string(mode), want)
		}
	}
}

// ---------------------------------------------------------------------------
// Pipeline tests
// ---------------------------------------------------------------------------

func TestPipeline_RedactedMode(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cfg := redaction.EventClassConfig{
		"tool.result": redaction.StorageModeRedacted,
	}
	p := redaction.NewPipeline(r, cfg)

	payload := map[string]any{
		"content": "postgres://user:secret@db:5432/prod",
		"step":    1,
	}
	out, keep := p.Apply("tool.result", payload)
	if !keep {
		t.Fatal("expected keep=true for redacted mode")
	}
	content, _ := out["content"].(string)
	if !strings.Contains(content, "[REDACTED:connection_string]") {
		t.Errorf("expected redacted content, got %q", content)
	}
	// Non-string fields should be preserved.
	if out["step"] != 1 {
		t.Errorf("expected step=1, got %v", out["step"])
	}
}

func TestPipeline_FullMode(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cfg := redaction.EventClassConfig{
		"run.started": redaction.StorageModeFull,
	}
	p := redaction.NewPipeline(r, cfg)

	secret := "postgres://user:secret@db:5432/prod"
	payload := map[string]any{"content": secret}
	out, keep := p.Apply("run.started", payload)
	if !keep {
		t.Fatal("expected keep=true for full mode")
	}
	content, _ := out["content"].(string)
	// Full mode: no redaction applied.
	if content != secret {
		t.Errorf("full mode should not redact: got %q", content)
	}
}

func TestPipeline_HashedMode(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cfg := redaction.EventClassConfig{
		"tool.call": redaction.StorageModeHashed,
	}
	p := redaction.NewPipeline(r, cfg)

	secret := "postgres://user:secret@db:5432/prod"
	payload := map[string]any{"content": secret}
	out, keep := p.Apply("tool.call", payload)
	if !keep {
		t.Fatal("expected keep=true for hashed mode")
	}
	content, _ := out["content"].(string)
	// Hashed mode: string values are replaced with their SHA-256 hex digest.
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte(secret)))
	if content != expected {
		t.Errorf("hashed mode: got %q, want %q", content, expected)
	}
}

func TestPipeline_NoneMode(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cfg := redaction.EventClassConfig{
		"debug.verbose": redaction.StorageModeNone,
	}
	p := redaction.NewPipeline(r, cfg)

	payload := map[string]any{"content": "anything"}
	_, keep := p.Apply("debug.verbose", payload)
	if keep {
		t.Fatal("expected keep=false for none mode")
	}
}

func TestPipeline_DefaultMode_Redacted(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	// Empty config: unknown event types default to redacted.
	p := redaction.NewPipeline(r, redaction.EventClassConfig{})

	payload := map[string]any{
		"content": "mysql://root:pass@localhost/db",
	}
	out, keep := p.Apply("unknown.event", payload)
	if !keep {
		t.Fatal("expected keep=true for default (redacted) mode")
	}
	content, _ := out["content"].(string)
	if !strings.Contains(content, "[REDACTED:connection_string]") {
		t.Errorf("default mode should redact, got %q", content)
	}
}

func TestPipeline_NestedStringValues(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	p := redaction.NewPipeline(r, redaction.EventClassConfig{})

	payload := map[string]any{
		"top": "safe text",
		"nested": map[string]any{
			"secret": "redis://user:password@host:6379",
			"count":  42,
		},
	}
	out, keep := p.Apply("any.event", payload)
	if !keep {
		t.Fatal("expected keep=true")
	}
	nested, _ := out["nested"].(map[string]any)
	if nested == nil {
		t.Fatal("nested map lost")
	}
	secretVal, _ := nested["secret"].(string)
	if !strings.Contains(secretVal, "[REDACTED:connection_string]") {
		t.Errorf("nested secret not redacted, got %q", secretVal)
	}
	if nested["count"] != 42 {
		t.Errorf("non-string value changed: %v", nested["count"])
	}
	// Top-level safe text unchanged.
	if out["top"] != "safe text" {
		t.Errorf("safe top-level field changed: %v", out["top"])
	}
}

func TestPipeline_NilPayload(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	p := redaction.NewPipeline(r, redaction.EventClassConfig{})
	out, keep := p.Apply("any.event", nil)
	if !keep {
		t.Fatal("expected keep=true for nil payload")
	}
	if out == nil {
		t.Fatal("expected non-nil map for nil payload")
	}
}

func TestPipeline_DoesNotMutateInputPayload(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	p := redaction.NewPipeline(r, redaction.EventClassConfig{})

	original := "postgres://user:secret@db:5432/prod"
	payload := map[string]any{"content": original}
	p.Apply("any.event", payload)
	// Original map must not be modified.
	if payload["content"] != original {
		t.Errorf("Apply mutated input payload: got %q", payload["content"])
	}
}

// ---------------------------------------------------------------------------
// Concurrency / race detector tests
// ---------------------------------------------------------------------------

func TestRedactor_ConcurrentRedact(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			input := "postgres://user:secret@localhost:5432/db"
			out := r.Redact(input)
			if !strings.Contains(out, "[REDACTED:") {
				t.Errorf("race test: unexpected output %q", out)
			}
		}()
	}
	wg.Wait()
}

func TestPipeline_ConcurrentApply(t *testing.T) {
	t.Parallel()
	r := redaction.NewRedactor(nil)
	cfg := redaction.EventClassConfig{
		"tool.result": redaction.StorageModeRedacted,
	}
	p := redaction.NewPipeline(r, cfg)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			payload := map[string]any{
				"content": "redis://:pass@host:6379",
			}
			out, keep := p.Apply("tool.result", payload)
			if !keep {
				t.Errorf("race test: unexpected keep=false")
			}
			content, _ := out["content"].(string)
			if !strings.Contains(content, "[REDACTED:") {
				t.Errorf("race test: not redacted: %q", content)
			}
		}()
	}
	wg.Wait()
}
