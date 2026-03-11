package observationalmemory

import (
	"strings"
	"testing"
)

func TestParseStructuredReflectionLegacyPlainText(t *testing.T) {
	t.Parallel()

	raw := "User prefers short responses. Never auto-commit."
	sr := ParseStructuredReflection(raw)

	if sr.SchemaVersion != 0 {
		t.Fatalf("expected SchemaVersion 0 for legacy, got %d", sr.SchemaVersion)
	}
	if sr.Summary != raw {
		t.Fatalf("expected full text as summary, got %q", sr.Summary)
	}
	if len(sr.Supersessions) != 0 {
		t.Fatalf("expected no supersessions, got %d", len(sr.Supersessions))
	}
	if len(sr.Contradictions) != 0 {
		t.Fatalf("expected no contradictions, got %d", len(sr.Contradictions))
	}
}

func TestParseStructuredReflectionEmpty(t *testing.T) {
	t.Parallel()

	sr := ParseStructuredReflection("")
	if sr.SchemaVersion != 0 {
		t.Fatalf("expected SchemaVersion 0 for empty, got %d", sr.SchemaVersion)
	}
	if sr.Summary != "" {
		t.Fatalf("expected empty summary, got %q", sr.Summary)
	}
}

func TestParseStructuredReflectionFullStructured(t *testing.T) {
	t.Parallel()

	raw := `SUMMARY:
User changed indentation preference from tabs to spaces. Auth method changed to bearer token.

SUPERSESSIONS:
- [seq:5] replaces [seq:2]: user changed from tabs to spaces
- [seq:8] replaces [seq:1]: auth method changed to bearer token

CONTRADICTIONS:
- [seq:3] vs [seq:7]: conflicting constraints on retry count (3 vs 5)
`
	sr := ParseStructuredReflection(raw)

	if sr.SchemaVersion != 1 {
		t.Fatalf("expected SchemaVersion 1, got %d", sr.SchemaVersion)
	}
	if sr.Summary == "" {
		t.Fatalf("expected non-empty summary")
	}
	if !containsString(sr.Summary, "tabs to spaces") && !containsString(sr.Summary, "bearer token") {
		t.Fatalf("unexpected summary content: %q", sr.Summary)
	}

	if len(sr.Supersessions) != 2 {
		t.Fatalf("expected 2 supersessions, got %d: %+v", len(sr.Supersessions), sr.Supersessions)
	}
	if sr.Supersessions[0].NewerSeq != 5 || sr.Supersessions[0].OlderSeq != 2 {
		t.Fatalf("unexpected first supersession: %+v", sr.Supersessions[0])
	}
	if sr.Supersessions[0].Reason == "" {
		t.Fatalf("expected non-empty reason for first supersession")
	}
	if sr.Supersessions[1].NewerSeq != 8 || sr.Supersessions[1].OlderSeq != 1 {
		t.Fatalf("unexpected second supersession: %+v", sr.Supersessions[1])
	}

	if len(sr.Contradictions) != 1 {
		t.Fatalf("expected 1 contradiction, got %d: %+v", len(sr.Contradictions), sr.Contradictions)
	}
	if sr.Contradictions[0].SeqA != 3 || sr.Contradictions[0].SeqB != 7 {
		t.Fatalf("unexpected contradiction: %+v", sr.Contradictions[0])
	}
	if sr.Contradictions[0].Detail == "" {
		t.Fatalf("expected non-empty detail for contradiction")
	}
}

func TestParseStructuredReflectionNoSupersessionsNoContradictions(t *testing.T) {
	t.Parallel()

	raw := `SUMMARY:
User wants concise responses and prefers Go idioms.

SUPERSESSIONS:

CONTRADICTIONS:
`
	sr := ParseStructuredReflection(raw)

	if sr.SchemaVersion != 1 {
		t.Fatalf("expected SchemaVersion 1, got %d", sr.SchemaVersion)
	}
	if sr.Summary == "" {
		t.Fatalf("expected non-empty summary")
	}
	if len(sr.Supersessions) != 0 {
		t.Fatalf("expected no supersessions, got %d", len(sr.Supersessions))
	}
	if len(sr.Contradictions) != 0 {
		t.Fatalf("expected no contradictions, got %d", len(sr.Contradictions))
	}
}

func TestParseStructuredReflectionOnlySupersessions(t *testing.T) {
	t.Parallel()

	raw := `SUMMARY:
Auth method changed.

SUPERSESSIONS:
- [seq:3] replaces [seq:1]: bearer token replaced API key

CONTRADICTIONS:
`
	sr := ParseStructuredReflection(raw)

	if sr.SchemaVersion != 1 {
		t.Fatalf("expected SchemaVersion 1, got %d", sr.SchemaVersion)
	}
	if len(sr.Supersessions) != 1 {
		t.Fatalf("expected 1 supersession, got %d", len(sr.Supersessions))
	}
	if sr.Supersessions[0].NewerSeq != 3 || sr.Supersessions[0].OlderSeq != 1 {
		t.Fatalf("unexpected supersession seqs: %+v", sr.Supersessions[0])
	}
	if len(sr.Contradictions) != 0 {
		t.Fatalf("expected no contradictions, got %d", len(sr.Contradictions))
	}
}

func TestParseStructuredReflectionOnlyContradictions(t *testing.T) {
	t.Parallel()

	raw := `SUMMARY:
Conflicting retry count guidance.

SUPERSESSIONS:

CONTRADICTIONS:
- [seq:2] vs [seq:4]: conflicting retry count (3 vs 5)
- [seq:6] vs [seq:9]: conflicting timeout value (30s vs 60s)
`
	sr := ParseStructuredReflection(raw)

	if sr.SchemaVersion != 1 {
		t.Fatalf("expected SchemaVersion 1, got %d", sr.SchemaVersion)
	}
	if len(sr.Contradictions) != 2 {
		t.Fatalf("expected 2 contradictions, got %d", len(sr.Contradictions))
	}
	if sr.Contradictions[0].SeqA != 2 || sr.Contradictions[0].SeqB != 4 {
		t.Fatalf("unexpected first contradiction: %+v", sr.Contradictions[0])
	}
	if sr.Contradictions[1].SeqA != 6 || sr.Contradictions[1].SeqB != 9 {
		t.Fatalf("unexpected second contradiction: %+v", sr.Contradictions[1])
	}
	if len(sr.Supersessions) != 0 {
		t.Fatalf("expected no supersessions, got %d", len(sr.Supersessions))
	}
}

func TestParseStructuredReflectionMalformedLines(t *testing.T) {
	t.Parallel()

	// Lines that don't match the expected format should be silently ignored.
	raw := `SUMMARY:
Some summary.

SUPERSESSIONS:
- invalid line without seq brackets
- [seq:abc] replaces [seq:2]: non-numeric seq should be ignored
- [seq:3] replaces [seq:1]: valid line

CONTRADICTIONS:
- bad format
- [seq:5] vs [seq:7]: valid contradiction
`
	sr := ParseStructuredReflection(raw)

	if sr.SchemaVersion != 1 {
		t.Fatalf("expected SchemaVersion 1, got %d", sr.SchemaVersion)
	}
	// Only the valid supersession line should be parsed.
	if len(sr.Supersessions) != 1 {
		t.Fatalf("expected 1 valid supersession, got %d: %+v", len(sr.Supersessions), sr.Supersessions)
	}
	if sr.Supersessions[0].NewerSeq != 3 || sr.Supersessions[0].OlderSeq != 1 {
		t.Fatalf("unexpected supersession: %+v", sr.Supersessions[0])
	}
	// Only the valid contradiction line should be parsed.
	if len(sr.Contradictions) != 1 {
		t.Fatalf("expected 1 valid contradiction, got %d: %+v", len(sr.Contradictions), sr.Contradictions)
	}
	if sr.Contradictions[0].SeqA != 5 || sr.Contradictions[0].SeqB != 7 {
		t.Fatalf("unexpected contradiction: %+v", sr.Contradictions[0])
	}
}

// containsString is a helper to check if a string contains a substring.
func containsString(s, sub string) bool {
	return strings.Contains(s, sub)
}
