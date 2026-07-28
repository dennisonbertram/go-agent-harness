package harness

import (
	"strings"
	"testing"
)

// A JSON API error is the common case and must survive untouched: it is the
// text a user needs to read.
func TestProviderHTTPErrorKeepsJSONBody(t *testing.T) {
	err := &ProviderHTTPError{
		Provider:   "openai",
		StatusCode: 429,
		Body:       `{"error":{"message":"rate limit reached"}}`,
	}
	want := `openai request failed (429): {"error":{"message":"rate limit reached"}}`
	if got := err.Error(); got != want {
		t.Fatalf("body was altered:\n got %q\nwant %q", got, want)
	}
}

// The regression: chatgpt.com answers a wrong-endpoint request with a styled
// HTML block page, and the whole stylesheet used to land in the transcript.
func TestProviderHTTPErrorCollapsesHTMLPage(t *testing.T) {
	page := `<html><head><style global>body{font-family:Arial}.container{display:flex;` +
		strings.Repeat("gap:2rem;height:100%;justify-content:center;", 40) +
		`}</style></head><body><div class="container">Access denied</div></body></html>`

	got := (&ProviderHTTPError{Provider: "codex-subscription", StatusCode: 403, Body: page}).Error()

	if !strings.HasPrefix(got, "codex-subscription request failed (403): ") {
		t.Fatalf("lost the provider and status prefix: %q", got)
	}
	if strings.Contains(got, "font-family") || strings.Contains(got, "justify-content") {
		t.Fatalf("stylesheet leaked into the error: %q", got)
	}
	if !strings.Contains(got, "Access denied") {
		t.Fatalf("dropped the one readable line: %q", got)
	}
	if len(got) > 600 {
		t.Fatalf("error is %d chars, still unreadable in a transcript", len(got))
	}
}

// A page with no prose at all still has to say what happened rather than
// rendering as an empty error.
func TestProviderHTTPErrorHTMLWithNoText(t *testing.T) {
	got := SummarizeErrorBody(`<html><head><style>body{color:red}</style></head><body></body></html>`)
	if !strings.Contains(got, "HTML error page") {
		t.Fatalf("expected an HTML marker, got %q", got)
	}
}

func TestProviderHTTPErrorTruncatesLongPlainBody(t *testing.T) {
	got := SummarizeErrorBody(strings.Repeat("x", 5000))
	if len(got) > maxErrorBodyChars+40 {
		t.Fatalf("long body was not truncated: %d chars", len(got))
	}
	if !strings.HasSuffix(got, "(truncated)") {
		t.Fatalf("truncation was not signposted: %q", got[len(got)-40:])
	}
}
