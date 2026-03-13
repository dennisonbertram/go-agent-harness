package requestenvelope_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"go-agent-harness/internal/forensics/requestenvelope"
)

// TestRequestSnapshotFields verifies that RequestSnapshot has the required fields.
func TestRequestSnapshotFields(t *testing.T) {
	t.Parallel()

	snap := requestenvelope.RequestSnapshot{
		Step:          1,
		PromptHash:    "abc123",
		ToolNames:     []string{"read_file", "bash"},
		MemorySnippet: "User prefers concise answers",
	}

	if snap.Step != 1 {
		t.Errorf("Step: got %d, want 1", snap.Step)
	}
	if snap.PromptHash != "abc123" {
		t.Errorf("PromptHash: got %q, want %q", snap.PromptHash, "abc123")
	}
	if len(snap.ToolNames) != 2 {
		t.Errorf("ToolNames length: got %d, want 2", len(snap.ToolNames))
	}
	if snap.MemorySnippet != "User prefers concise answers" {
		t.Errorf("MemorySnippet: got %q, want non-empty", snap.MemorySnippet)
	}
}

// TestRequestSnapshotEmptyToolNames verifies that an empty ToolNames slice is valid.
func TestRequestSnapshotEmptyToolNames(t *testing.T) {
	t.Parallel()

	snap := requestenvelope.RequestSnapshot{
		Step:       1,
		PromptHash: "abc123",
		ToolNames:  []string{},
	}
	if snap.ToolNames == nil {
		t.Error("ToolNames should be non-nil empty slice, not nil")
	}
}

// TestRequestSnapshotNoMemory verifies that missing MemorySnippet is valid (empty string).
func TestRequestSnapshotNoMemory(t *testing.T) {
	t.Parallel()

	snap := requestenvelope.RequestSnapshot{
		Step:          2,
		PromptHash:    "def456",
		ToolNames:     []string{"glob"},
		MemorySnippet: "",
	}
	if snap.MemorySnippet != "" {
		t.Errorf("expected empty MemorySnippet, got %q", snap.MemorySnippet)
	}
}

// TestResponseMetaFields verifies that ResponseMeta has the required fields.
func TestResponseMetaFields(t *testing.T) {
	t.Parallel()

	meta := requestenvelope.ResponseMeta{
		Step:         3,
		LatencyMS:    142,
		ModelVersion: "gpt-4.1-2025-04-14",
	}

	if meta.Step != 3 {
		t.Errorf("Step: got %d, want 3", meta.Step)
	}
	if meta.LatencyMS != 142 {
		t.Errorf("LatencyMS: got %d, want 142", meta.LatencyMS)
	}
	if meta.ModelVersion != "gpt-4.1-2025-04-14" {
		t.Errorf("ModelVersion: got %q, want %q", meta.ModelVersion, "gpt-4.1-2025-04-14")
	}
}

// TestResponseMetaEmptyModelVersion verifies that an empty ModelVersion is valid.
func TestResponseMetaEmptyModelVersion(t *testing.T) {
	t.Parallel()

	meta := requestenvelope.ResponseMeta{
		Step:         1,
		LatencyMS:    50,
		ModelVersion: "",
	}
	if meta.ModelVersion != "" {
		t.Errorf("expected empty ModelVersion, got %q", meta.ModelVersion)
	}
}

// TestHashPrompt verifies that HashPrompt returns a hex-encoded SHA-256 hash.
func TestHashPrompt(t *testing.T) {
	t.Parallel()

	input := "You are a helpful assistant.\n\nPlease write tests."
	got := requestenvelope.HashPrompt(input)

	// Verify it is valid hex SHA-256 (64 chars)
	if len(got) != 64 {
		t.Errorf("HashPrompt length: got %d, want 64 (SHA-256 hex)", len(got))
	}

	// Verify it matches manual SHA-256 computation
	h := sha256.Sum256([]byte(input))
	want := hex.EncodeToString(h[:])
	if got != want {
		t.Errorf("HashPrompt mismatch: got %q, want %q", got, want)
	}
}

// TestHashPromptEmptyString verifies that an empty string produces a valid hash.
func TestHashPromptEmptyString(t *testing.T) {
	t.Parallel()

	got := requestenvelope.HashPrompt("")
	if len(got) != 64 {
		t.Errorf("HashPrompt('') length: got %d, want 64", len(got))
	}
}

// TestHashPromptDeterministic verifies that the same input always produces the same hash.
func TestHashPromptDeterministic(t *testing.T) {
	t.Parallel()

	input := "test input string"
	h1 := requestenvelope.HashPrompt(input)
	h2 := requestenvelope.HashPrompt(input)
	if h1 != h2 {
		t.Errorf("HashPrompt is not deterministic: %q != %q", h1, h2)
	}
}

// TestHashPromptDifferentInputs verifies that different inputs produce different hashes.
func TestHashPromptDifferentInputs(t *testing.T) {
	t.Parallel()

	h1 := requestenvelope.HashPrompt("input A")
	h2 := requestenvelope.HashPrompt("input B")
	if h1 == h2 {
		t.Error("different inputs should produce different hashes")
	}
}

// TestRequestSnapshotZeroStep verifies that step=0 is valid (edge case).
func TestRequestSnapshotZeroStep(t *testing.T) {
	t.Parallel()

	snap := requestenvelope.RequestSnapshot{
		Step:       0,
		PromptHash: "abc",
		ToolNames:  []string{},
	}
	if snap.Step != 0 {
		t.Errorf("expected step 0, got %d", snap.Step)
	}
}

// TestResponseMetaZeroLatency verifies that LatencyMS=0 is valid.
func TestResponseMetaZeroLatency(t *testing.T) {
	t.Parallel()

	meta := requestenvelope.ResponseMeta{
		Step:         1,
		LatencyMS:    0,
		ModelVersion: "",
	}
	if meta.LatencyMS != 0 {
		t.Errorf("expected LatencyMS=0, got %d", meta.LatencyMS)
	}
}

// TestHashPromptHMAC_DifferentFromSHA256 verifies that HashPromptHMAC with a
// key produces a different value than plain HashPrompt (HIGH-3 fix).
func TestHashPromptHMAC_DifferentFromSHA256(t *testing.T) {
	t.Parallel()
	prompt := "test prompt"
	key := []byte("secret-deployment-key")
	hmacHash := requestenvelope.HashPromptHMAC(prompt, key)
	sha256Hash := requestenvelope.HashPrompt(prompt)
	if hmacHash == sha256Hash {
		t.Error("HashPromptHMAC should differ from plain SHA-256 HashPrompt")
	}
}

// TestHashPromptHMAC_Deterministic verifies that the same prompt+key always
// yields the same hash.
func TestHashPromptHMAC_Deterministic(t *testing.T) {
	t.Parallel()
	prompt := "my system prompt"
	key := []byte("stable-key")
	h1 := requestenvelope.HashPromptHMAC(prompt, key)
	h2 := requestenvelope.HashPromptHMAC(prompt, key)
	if h1 != h2 {
		t.Errorf("HashPromptHMAC is not deterministic: %q != %q", h1, h2)
	}
}

// TestHashPromptHMAC_DifferentKeys verifies that different keys produce
// different hashes for the same prompt (offline-guess prevention).
func TestHashPromptHMAC_DifferentKeys(t *testing.T) {
	t.Parallel()
	prompt := "common system prompt"
	h1 := requestenvelope.HashPromptHMAC(prompt, []byte("key-one"))
	h2 := requestenvelope.HashPromptHMAC(prompt, []byte("key-two"))
	if h1 == h2 {
		t.Error("different keys should produce different HMAC hashes for the same prompt")
	}
}

// TestHashPromptHMAC_Returns64Hex verifies the output format.
func TestHashPromptHMAC_Returns64Hex(t *testing.T) {
	t.Parallel()
	got := requestenvelope.HashPromptHMAC("prompt", []byte("k"))
	if len(got) != 64 {
		t.Errorf("HashPromptHMAC length: got %d, want 64", len(got))
	}
}
