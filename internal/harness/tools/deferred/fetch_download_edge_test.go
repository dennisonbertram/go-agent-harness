package deferred

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tools "go-agent-harness/internal/harness/tools"
)

// TestFetchTool_Handler_InvalidJSON verifies malformed args are rejected with
// a wrapped parse error instead of silently treating them as an empty request.
func TestFetchTool_Handler_InvalidJSON(t *testing.T) {
	t.Parallel()

	tool := FetchTool(http.DefaultClient, nil)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse fetch args") {
		t.Errorf("expected 'parse fetch args' in error, got %q", err.Error())
	}
}

// TestFetchTool_Handler_InvalidURL verifies fetch rejects a URL that fails to
// parse (distinct from one that parses but has a disallowed scheme).
func TestFetchTool_Handler_InvalidURL(t *testing.T) {
	t.Parallel()

	tool := FetchTool(http.DefaultClient, nil)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"url":"http://%zz"}`))
	if err == nil {
		t.Fatal("expected error for a malformed URL")
	}
	if !strings.Contains(err.Error(), "invalid url") {
		t.Errorf("expected 'invalid url' in error, got %q", err.Error())
	}
}

// TestFetchTool_Handler_MaxBytesNonPositiveDefaultsTo128KB verifies an
// explicit non-positive max_bytes falls back to the 128KiB default rather
// than being used verbatim (which would either error or return no content).
func TestFetchTool_Handler_MaxBytesNonPositiveDefaultsTo128KB(t *testing.T) {
	t.Parallel()

	const defaultMax = 128 * 1024
	body := strings.Repeat("a", defaultMax+100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	tool := FetchTool(srv.Client(), []string{testServerHost(t, srv.URL)})
	raw, _ := json.Marshal(map[string]any{"url": srv.URL, "max_bytes": 0})
	result, err := tool.Handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	content, _ := out["content"].(string)
	if len(content) != defaultMax {
		t.Errorf("expected content truncated to default %d bytes, got %d", defaultMax, len(content))
	}
	if out["truncated"] != true {
		t.Errorf("expected truncated=true once the default 128KiB cap is exceeded, got %v", out["truncated"])
	}
}

// TestFetchTool_Handler_MaxBytesOverCapClampsToOneMiB verifies a max_bytes
// above the 1MiB ceiling is clamped rather than honored verbatim.
func TestFetchTool_Handler_MaxBytesOverCapClampsToOneMiB(t *testing.T) {
	t.Parallel()

	const capBytes = 1024 * 1024
	body := strings.Repeat("b", capBytes+100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	tool := FetchTool(srv.Client(), []string{testServerHost(t, srv.URL)})
	raw, _ := json.Marshal(map[string]any{"url": srv.URL, "max_bytes": 5_000_000})
	result, err := tool.Handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	content, _ := out["content"].(string)
	if len(content) != capBytes {
		t.Errorf("expected content clamped to the 1MiB cap (%d), got %d", capBytes, len(content))
	}
	if out["truncated"] != true {
		t.Errorf("expected truncated=true once the clamped cap is exceeded, got %v", out["truncated"])
	}
}

// TestFetchTool_Handler_FormatEchoedWhenProvided verifies the optional
// "format" field is included in the result only when supplied.
func TestFetchTool_Handler_FormatEchoedWhenProvided(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hi"))
	}))
	defer srv.Close()

	tool := FetchTool(srv.Client(), []string{testServerHost(t, srv.URL)})
	raw, _ := json.Marshal(map[string]any{"url": srv.URL, "format": "markdown"})
	result, err := tool.Handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["format"] != "markdown" {
		t.Errorf("expected format 'markdown' echoed back, got %v", out["format"])
	}
}

// TestFetchTool_Handler_TransportFailure verifies a transport-level failure
// (nothing listening on the destination) surfaces as a wrapped "fetch
// request failed" error.
func TestFetchTool_Handler_TransportFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	host := testServerHost(t, srv.URL)
	srv.Close()

	tool := FetchTool(&http.Client{Timeout: 3 * time.Second}, []string{host})
	raw, _ := json.Marshal(map[string]any{"url": deadURL})
	_, err := tool.Handler(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error when the destination is unreachable")
	}
	if !strings.Contains(err.Error(), "fetch request failed") {
		t.Errorf("expected 'fetch request failed' in error, got %q", err.Error())
	}
}

// TestDownloadTool_Handler_InvalidJSON verifies malformed args are rejected
// with a wrapped parse error.
func TestDownloadTool_Handler_InvalidJSON(t *testing.T) {
	t.Parallel()

	tool := DownloadTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse download args") {
		t.Errorf("expected 'parse download args' in error, got %q", err.Error())
	}
}

// TestDownloadTool_Handler_FilePathEscapeRejected is a regression test for
// path-traversal in file_path: a "../" destination outside the workspace
// root must be rejected before any network request or file write happens.
func TestDownloadTool_Handler_FilePathEscapeRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tool := DownloadTool(tools.BuildOptions{WorkspaceRoot: dir})
	raw, _ := json.Marshal(map[string]string{"url": "https://example.com/file", "file_path": "../outside.txt"})
	_, err := tool.Handler(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for a file_path escaping the workspace root")
	}
	if !strings.Contains(err.Error(), "escapes workspace") {
		t.Errorf("expected 'escapes workspace' in error, got %q", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "outside.txt")); statErr == nil {
		t.Fatal("expected no file to be written outside the workspace root")
	}
}

// TestDownloadTool_Handler_BadScheme verifies download rejects non-http(s)
// schemes before attempting any network access.
func TestDownloadTool_Handler_BadScheme(t *testing.T) {
	t.Parallel()

	tool := DownloadTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	raw, _ := json.Marshal(map[string]string{"url": "ftp://example.com/file", "file_path": "out.txt"})
	_, err := tool.Handler(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
	if !strings.Contains(err.Error(), "unsupported url scheme") {
		t.Errorf("expected 'unsupported url scheme' in error, got %q", err.Error())
	}
}

// TestDownloadTool_Handler_InvalidURL verifies download rejects a URL that
// fails to parse (as opposed to one that parses but has a bad scheme).
func TestDownloadTool_Handler_InvalidURL(t *testing.T) {
	t.Parallel()

	tool := DownloadTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	raw, _ := json.Marshal(map[string]string{"url": "http://%zz", "file_path": "out.txt"})
	_, err := tool.Handler(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for a malformed URL")
	}
	if !strings.Contains(err.Error(), "invalid url") {
		t.Errorf("expected 'invalid url' in error, got %q", err.Error())
	}
}

// TestDownloadTool_Handler_MkdirAllFailsWhenParentIsAFile verifies a clear
// error (not a panic or a silently-empty file) when the destination's parent
// directory path is blocked by an existing regular file.
func TestDownloadTool_Handler_MkdirAllFailsWhenParentIsAFile(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// "blocker" is a regular file, so "blocker/out.txt" cannot have its parent created.
	if err := os.WriteFile(filepath.Join(dir, "blocker"), []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tool := DownloadTool(tools.BuildOptions{WorkspaceRoot: dir, HTTPClient: srv.Client(), NetworkAllowlist: []string{testServerHost(t, srv.URL)}})
	raw, _ := json.Marshal(map[string]string{"url": srv.URL, "file_path": "blocker/out.txt"})
	_, err := tool.Handler(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error when the parent path is blocked by a file")
	}
	if !strings.Contains(err.Error(), "create parent dir") {
		t.Errorf("expected 'create parent dir' in error, got %q", err.Error())
	}
}

// TestDownloadTool_Handler_WriteFileFailsWhenTargetIsADirectory verifies a
// clear error when the destination path itself is an existing directory.
func TestDownloadTool_Handler_WriteFileFailsWhenTargetIsADirectory(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "out.txt"), 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tool := DownloadTool(tools.BuildOptions{WorkspaceRoot: dir, HTTPClient: srv.Client(), NetworkAllowlist: []string{testServerHost(t, srv.URL)}})
	raw, _ := json.Marshal(map[string]string{"url": srv.URL, "file_path": "out.txt"})
	_, err := tool.Handler(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error when the destination path is a directory")
	}
	if !strings.Contains(err.Error(), "write downloaded file") {
		t.Errorf("expected 'write downloaded file' in error, got %q", err.Error())
	}
}

// TestDownloadTool_Handler_MaxBytesNonPositiveDefaultsToOneMiB verifies an
// explicit non-positive max_bytes falls back to the 1MiB default.
func TestDownloadTool_Handler_MaxBytesNonPositiveDefaultsToOneMiB(t *testing.T) {
	t.Parallel()

	const defaultMax = 1024 * 1024
	body := strings.Repeat("c", defaultMax+100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dir := t.TempDir()
	tool := DownloadTool(tools.BuildOptions{WorkspaceRoot: dir, HTTPClient: srv.Client(), NetworkAllowlist: []string{testServerHost(t, srv.URL)}})
	raw, _ := json.Marshal(map[string]any{"url": srv.URL, "file_path": "out.bin", "max_bytes": 0})
	result, err := tool.Handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["bytes_written"].(float64) != float64(defaultMax) {
		t.Errorf("expected bytes_written=%d (default cap), got %v", defaultMax, out["bytes_written"])
	}
	if out["truncated"] != true {
		t.Errorf("expected truncated=true once the default 1MiB cap is exceeded, got %v", out["truncated"])
	}
	data, err := os.ReadFile(filepath.Join(dir, "out.bin"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if len(data) != defaultMax {
		t.Errorf("expected written file truncated to %d bytes, got %d", defaultMax, len(data))
	}
}
