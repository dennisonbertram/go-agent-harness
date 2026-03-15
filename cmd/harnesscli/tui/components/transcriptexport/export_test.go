package transcriptexport_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-agent-harness/cmd/harnesscli/tui/components/transcriptexport"
)

// ─── Exporter tests ──────────────────────────────────────────────────────────

func TestTUI059_ExportEmptyEntries(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	path, err := e.Export(nil)
	if err != nil {
		t.Fatalf("Export(nil) unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("Export returned empty path")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "# Conversation Transcript") {
		t.Errorf("missing header in: %q", s)
	}
	if !strings.Contains(s, "Exported:") {
		t.Errorf("missing exported timestamp in: %q", s)
	}
}

func TestTUI059_ExportSingleUserEntry(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	ts := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	entries := []transcriptexport.TranscriptEntry{
		{Role: "user", Content: "Hello, world!", Timestamp: ts},
	}

	path, err := e.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "## User") {
		t.Errorf("missing User heading: %q", s)
	}
	if !strings.Contains(s, "Hello, world!") {
		t.Errorf("missing content: %q", s)
	}
}

func TestTUI059_ExportSingleAssistantEntry(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	ts := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	entries := []transcriptexport.TranscriptEntry{
		{Role: "assistant", Content: "I can help with that.", Timestamp: ts},
	}

	path, err := e.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)

	if !strings.Contains(s, "## Assistant") {
		t.Errorf("missing Assistant heading: %q", s)
	}
	if !strings.Contains(s, "I can help with that.") {
		t.Errorf("missing content: %q", s)
	}
}

func TestTUI059_ExportSingleToolEntry(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	ts := time.Date(2026, 3, 15, 14, 31, 0, 0, time.UTC)
	entries := []transcriptexport.TranscriptEntry{
		{Role: "tool", Content: "exit 0", Timestamp: ts, ToolName: "bash"},
	}

	path, err := e.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)

	if !strings.Contains(s, "## Tool: bash") {
		t.Errorf("missing Tool heading: %q", s)
	}
	if !strings.Contains(s, "exit 0") {
		t.Errorf("missing tool content: %q", s)
	}
}

func TestTUI059_ExportToolEntryNoName(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	ts := time.Date(2026, 3, 15, 14, 31, 0, 0, time.UTC)
	entries := []transcriptexport.TranscriptEntry{
		{Role: "tool", Content: "result", Timestamp: ts, ToolName: ""},
	}

	path, err := e.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)

	// Should fall back to "tool" as name
	if !strings.Contains(s, "## Tool: tool") {
		t.Errorf("expected fallback tool name: %q", s)
	}
}

func TestTUI059_ExportMixedEntries(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	base := time.Date(2026, 3, 15, 14, 30, 0, 0, time.UTC)
	entries := []transcriptexport.TranscriptEntry{
		{Role: "user", Content: "Run ls", Timestamp: base},
		{Role: "assistant", Content: "Running ls for you.", Timestamp: base.Add(time.Second)},
		{Role: "tool", Content: "file1.go\nfile2.go", Timestamp: base.Add(2 * time.Second), ToolName: "bash"},
	}

	path, err := e.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)

	if !strings.Contains(s, "## User") {
		t.Errorf("missing User: %q", s)
	}
	if !strings.Contains(s, "## Assistant") {
		t.Errorf("missing Assistant: %q", s)
	}
	if !strings.Contains(s, "## Tool: bash") {
		t.Errorf("missing Tool: %q", s)
	}
	if !strings.Contains(s, "Run ls") {
		t.Errorf("missing user content: %q", s)
	}
	if !strings.Contains(s, "Running ls for you.") {
		t.Errorf("missing assistant content: %q", s)
	}
	if !strings.Contains(s, "file1.go") {
		t.Errorf("missing tool output: %q", s)
	}
}

func TestTUI059_ExportCreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "subdir")

	e := transcriptexport.NewExporter(dir)
	path, err := e.Export(nil)
	if err != nil {
		t.Fatalf("Export with nested dir: %v", err)
	}
	if path == "" {
		t.Fatal("got empty path")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file not created at %s", path)
	}
}

func TestTUI059_ExportFilenameFormat(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	path, err := e.Export(nil)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}

	base := filepath.Base(path)
	if !strings.HasPrefix(base, "transcript-") {
		t.Errorf("filename should start with transcript-: %q", base)
	}
	if !strings.HasSuffix(base, ".md") {
		t.Errorf("filename should end with .md: %q", base)
	}
}

func TestTUI059_ExportReturnsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	path, err := e.Export(nil)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got: %q", path)
	}
}

func TestTUI059_ExportDefaultOutputDir(t *testing.T) {
	// NewExporter("") should not error — it uses "."
	e := transcriptexport.NewExporter("")
	if e.OutputDir != "." {
		t.Errorf("expected OutputDir='.', got %q", e.OutputDir)
	}
}

func TestTUI059_ExportRelativeDirResolvedToAbsolute(t *testing.T) {
	// A relative OutputDir like "." must be resolved to an absolute path
	// before use, so the returned file path is always absolute.
	e := transcriptexport.NewExporter(".")
	path, err := e.Export(nil)
	if err != nil {
		t.Fatalf("Export with relative dir '.': %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path when OutputDir is '.', got: %q", path)
	}
}

func TestTUI059_ExportMarkdownSeparators(t *testing.T) {
	dir := t.TempDir()
	e := transcriptexport.NewExporter(dir)

	ts := time.Now()
	entries := []transcriptexport.TranscriptEntry{
		{Role: "user", Content: "hello", Timestamp: ts},
		{Role: "assistant", Content: "world", Timestamp: ts},
	}

	path, err := e.Export(entries)
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
	content, _ := os.ReadFile(path)
	s := string(content)

	count := strings.Count(s, "---")
	if count < 2 {
		t.Errorf("expected at least 2 separators (---), got %d in: %q", count, s)
	}
}

// ─── StatusModel tests ───────────────────────────────────────────────────────

func TestTUI059_StatusModelIdleViewEmpty(t *testing.T) {
	m := transcriptexport.NewStatusModel()
	if m.Status != transcriptexport.ExportStatusIdle {
		t.Errorf("expected idle status")
	}
	v := m.View(80)
	if v != "" {
		t.Errorf("idle View should be empty, got %q", v)
	}
}

func TestTUI059_StatusModelSetSuccess(t *testing.T) {
	m := transcriptexport.NewStatusModel()
	m2 := m.SetSuccess("/tmp/transcript-20260315-143000.md")

	if m2.Status != transcriptexport.ExportStatusSuccess {
		t.Errorf("expected success status")
	}
	if m2.FilePath != "/tmp/transcript-20260315-143000.md" {
		t.Errorf("wrong file path: %q", m2.FilePath)
	}
	// original unchanged (immutable)
	if m.Status != transcriptexport.ExportStatusIdle {
		t.Errorf("original should remain idle")
	}
}

func TestTUI059_StatusModelSetError(t *testing.T) {
	m := transcriptexport.NewStatusModel()
	m2 := m.SetError("permission denied")

	if m2.Status != transcriptexport.ExportStatusError {
		t.Errorf("expected error status")
	}
	if m2.ErrMsg != "permission denied" {
		t.Errorf("wrong error msg: %q", m2.ErrMsg)
	}
	if m.Status != transcriptexport.ExportStatusIdle {
		t.Errorf("original should remain idle")
	}
}

func TestTUI059_StatusModelReset(t *testing.T) {
	m := transcriptexport.NewStatusModel().SetSuccess("/tmp/foo.md")
	m2 := m.Reset()

	if m2.Status != transcriptexport.ExportStatusIdle {
		t.Errorf("after reset expected idle, got %v", m2.Status)
	}
	if m2.FilePath != "" {
		t.Errorf("after reset FilePath should be empty, got %q", m2.FilePath)
	}
}

func TestTUI059_StatusModelSuccessViewWidth80(t *testing.T) {
	m := transcriptexport.NewStatusModel().SetSuccess("/tmp/transcript.md")
	v := m.View(80)

	if !strings.Contains(v, "Transcript saved to") {
		t.Errorf("success view missing saved phrase: %q", v)
	}
	if !strings.Contains(v, "/tmp/transcript.md") {
		t.Errorf("success view missing file path: %q", v)
	}
	if !strings.Contains(v, "✓") {
		t.Errorf("success view missing check mark: %q", v)
	}
}

func TestTUI059_StatusModelErrorViewWidth80(t *testing.T) {
	m := transcriptexport.NewStatusModel().SetError("disk full")
	v := m.View(80)

	if !strings.Contains(v, "Export failed") {
		t.Errorf("error view missing 'Export failed': %q", v)
	}
	if !strings.Contains(v, "disk full") {
		t.Errorf("error view missing error message: %q", v)
	}
	if !strings.Contains(v, "✗") {
		t.Errorf("error view missing cross mark: %q", v)
	}
}

func TestTUI059_StatusModelTransitions(t *testing.T) {
	m := transcriptexport.NewStatusModel()

	// idle -> success -> reset -> error -> reset
	m = m.SetSuccess("/tmp/a.md")
	if m.Status != transcriptexport.ExportStatusSuccess {
		t.Error("expected success after SetSuccess")
	}

	m = m.Reset()
	if m.Status != transcriptexport.ExportStatusIdle {
		t.Error("expected idle after Reset")
	}

	m = m.SetError("oops")
	if m.Status != transcriptexport.ExportStatusError {
		t.Error("expected error after SetError")
	}

	m = m.Reset()
	if m.Status != transcriptexport.ExportStatusIdle {
		t.Error("expected idle after second Reset")
	}
}

func TestTUI059_StatusModelViewZeroWidth(t *testing.T) {
	m := transcriptexport.NewStatusModel().SetSuccess("/tmp/x.md")
	// Should not panic at zero width
	_ = m.View(0)
}
