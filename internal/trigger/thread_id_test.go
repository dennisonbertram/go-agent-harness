package trigger

import (
	"strings"
	"testing"
)

// TestDeriveExternalThreadID_Deterministic verifies that the same inputs always
// produce the same ExternalThreadID.
func TestDeriveExternalThreadID_Deterministic(t *testing.T) {
	t.Parallel()

	id1 := DeriveExternalThreadID("github", "anthropic", "go-agent-harness", "42")
	id2 := DeriveExternalThreadID("github", "anthropic", "go-agent-harness", "42")
	if id1 != id2 {
		t.Errorf("expected identical IDs, got %q and %q", id1, id2)
	}
}

// TestDeriveExternalThreadID_UniquePerInput verifies that different inputs
// produce different ExternalThreadIDs.
func TestDeriveExternalThreadID_UniquePerInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		source, repoOwner, repoName, threadID string
	}{
		{"github", "anthropic", "go-agent-harness", "42"},
		{"github", "anthropic", "go-agent-harness", "43"},
		{"github", "anthropic", "other-repo", "42"},
		{"slack", "anthropic", "go-agent-harness", "42"},
		{"github", "other-org", "go-agent-harness", "42"},
	}

	seen := make(map[ExternalThreadID]struct{})
	for _, c := range cases {
		id := DeriveExternalThreadID(c.source, c.repoOwner, c.repoName, c.threadID)
		if _, exists := seen[id]; exists {
			t.Errorf("collision: inputs %+v produced duplicate ID %q", c, id)
		}
		seen[id] = struct{}{}
	}
}

// TestDeriveExternalThreadID_EmptyRepo verifies that empty repoOwner/repoName
// produces a valid, stable ID (no panic, deterministic).
func TestDeriveExternalThreadID_EmptyRepo(t *testing.T) {
	t.Parallel()

	id1 := DeriveExternalThreadID("slack", "", "", "C012AB3CD/1699999999.000001")
	id2 := DeriveExternalThreadID("slack", "", "", "C012AB3CD/1699999999.000001")
	if id1 != id2 {
		t.Errorf("expected identical IDs for empty repo, got %q and %q", id1, id2)
	}
	if id1.String() == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.HasPrefix(id1.String(), "slack:") {
		t.Errorf("expected ID to start with 'slack:', got %q", id1)
	}
}

// TestDeriveExternalThreadID_SourceNormalization verifies that source casing
// does not affect the resulting ID ("GitHub" == "github").
func TestDeriveExternalThreadID_SourceNormalization(t *testing.T) {
	t.Parallel()

	lower := DeriveExternalThreadID("github", "org", "repo", "99")
	upper := DeriveExternalThreadID("GitHub", "org", "repo", "99")
	mixed := DeriveExternalThreadID("GITHUB", "org", "repo", "99")

	if lower != upper {
		t.Errorf("expected 'github' == 'GitHub', got %q vs %q", lower, upper)
	}
	if lower != mixed {
		t.Errorf("expected 'github' == 'GITHUB', got %q vs %q", lower, mixed)
	}
	// The prefix should always be lowercase.
	if !strings.HasPrefix(lower.String(), "github:") {
		t.Errorf("expected ID to start with 'github:', got %q", lower)
	}
}

// TestDeriveExternalThreadID_Format verifies the output format is "source:hexhash".
func TestDeriveExternalThreadID_Format(t *testing.T) {
	t.Parallel()

	id := DeriveExternalThreadID("linear", "myorg", "myrepo", "ENG-123")
	s := id.String()

	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 'source:hash' format, got %q", s)
	}
	if parts[0] != "linear" {
		t.Errorf("expected source prefix 'linear', got %q", parts[0])
	}
	// SHA-256 hex is 64 characters.
	if len(parts[1]) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars: %q", len(parts[1]), parts[1])
	}
}
