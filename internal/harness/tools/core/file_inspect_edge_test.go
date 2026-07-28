package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestHumanSize verifies each unit boundary (B, KB, MB, GB) formats
// correctly, proving the threshold comparisons are the right way round.
func TestHumanSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{int64(2.5 * 1024 * 1024 * 1024), "2.5 GB"},
	}
	for _, tc := range cases {
		if got := humanSize(tc.bytes); got != tc.want {
			t.Errorf("humanSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

// TestFileInspectTool_PreviewLinesClamp verifies preview_lines above the
// documented maximum (100) is clamped rather than honored verbatim.
func TestFileInspectTool_PreviewLinesClamp(t *testing.T) {
	dir := t.TempDir()
	var content string
	for i := 0; i < 150; i++ {
		content += "line\n"
	}
	mustWrite(t, filepath.Join(dir, "a.txt"), content)

	tool := FileInspectTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","preview_lines":9999}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if n, _ := result["preview_lines"].(float64); n != 100 {
		t.Errorf("expected preview_lines clamped to 100, got %v", result["preview_lines"])
	}
}

// TestFileInspectTool_HexBytesClamp verifies hex_bytes above the documented
// maximum (1024) is clamped.
func TestFileInspectTool_HexBytesClamp(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 2000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	// Ensure it is detected as binary by including a NUL byte early on.
	data[0] = 0x00
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), data, 0o644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	tool := FileInspectTool(tools.BuildOptions{WorkspaceRoot: dir})
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"a.bin","hex_bytes":99999}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	warning, _ := result["truncation_warning"].(string)
	if warning == "" {
		t.Fatal("expected a truncation_warning showing the clamped hex_bytes count")
	}
	if !strings.Contains(warning, "1024") {
		t.Errorf("expected truncation_warning to mention the clamped 1024-byte limit, got %q", warning)
	}
}

// TestFileInspectTool_BadJSON verifies malformed JSON args produce a parse error.
func TestFileInspectTool_BadJSON(t *testing.T) {
	tool := FileInspectTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
}
