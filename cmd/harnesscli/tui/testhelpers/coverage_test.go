package testhelpers_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/testhelpers"
)

func TestTUI007_GoldenFileRoundTrip(t *testing.T) {
	t.Parallel()

	name := "coverage-golden.txt"
	path := filepath.Join("testdata", "snapshots", name)
	t.Cleanup(func() {
		_ = os.Remove(path)
	})

	if got := testhelpers.GoldenFile(t, name, "golden content", true); got != "golden content" {
		t.Fatalf("expected update path to return actual content, got %q", got)
	}
	if got := testhelpers.GoldenFile(t, name, "", false); got != "golden content" {
		t.Fatalf("expected read path to return stored content, got %q", got)
	}
}

func TestTUI007_NewTestServerAndSSEHandler(t *testing.T) {
	t.Parallel()

	srv := testhelpers.NewTestServer(testhelpers.SSEHandler([]string{
		`{"type":"run.started"}`,
		`{"type":"run.completed"}`,
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "event: message") {
		t.Fatalf("expected SSE event framing, got %q", text)
	}
	if !strings.Contains(text, `{"type":"run.completed"}`) {
		t.Fatalf("expected second SSE payload, got %q", text)
	}
}
