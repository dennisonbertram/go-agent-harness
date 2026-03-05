package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathAndCollectEntriesBranches(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if _, err := resolveWorkspacePath("", "a"); err == nil {
		t.Fatalf("expected missing workspace root error")
	}
	if _, err := resolveWorkspacePath(workspace, "/abs"); err == nil {
		t.Fatalf("expected absolute path rejection")
	}
	if _, err := resolveWorkspacePath(workspace, "../escape"); err == nil {
		t.Fatalf("expected escape rejection")
	}

	if err := os.MkdirAll(filepath.Join(workspace, "dir", ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "dir", "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	entries, truncated, err := collectEntries(workspace, filepath.Join(workspace, "dir"), false, 1, false, 0)
	if err != nil {
		t.Fatalf("collect entries non-recursive: %v", err)
	}
	if !truncated || len(entries) != 1 {
		t.Fatalf("expected truncated non-recursive result: entries=%v truncated=%v", entries, truncated)
	}

	entries, _, err = collectEntries(workspace, filepath.Join(workspace, "dir"), true, 50, false, 1)
	if err != nil {
		t.Fatalf("collect entries recursive: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected recursive entries")
	}
}

func TestReadWriteAndEditAdditionalBranches(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	list, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	write := findToolByName(t, list, "write")
	_, err = write.Handler(context.Background(), json.RawMessage(`{"path":"n.txt","content":"alpha"}`))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err = write.Handler(context.Background(), json.RawMessage(`{"path":"n.txt","content":"-beta","append":true}`))
	if err != nil {
		t.Fatalf("append write: %v", err)
	}

	read := findToolByName(t, list, "read")
	out, err := read.Handler(context.Background(), json.RawMessage(`{"path":"n.txt","max_bytes":3}`))
	if err != nil {
		t.Fatalf("read max_bytes: %v", err)
	}
	if !strings.Contains(out, `"truncated":true`) {
		t.Fatalf("expected truncated read output: %s", out)
	}

	edit := findToolByName(t, list, "edit")
	if _, err := edit.Handler(context.Background(), json.RawMessage(`{"path":"n.txt","old_text":"missing","new_text":"x"}`)); err == nil {
		t.Fatalf("expected missing edit target error")
	}
}

func TestBashManagerAndJobErrorBranches(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)

	if _, err := mgr.runForeground(context.Background(), "echo hi", 1, "../bad"); err == nil {
		t.Fatalf("expected working dir escape error")
	}

	result, err := mgr.runForeground(context.Background(), "sleep 0.2", 1, ".")
	if err != nil {
		t.Fatalf("runForeground timeout call should return result: %v", err)
	}
	if _, ok := result["timed_out"]; !ok {
		t.Fatalf("expected timed_out field in result")
	}

	if _, err := mgr.output("unknown", false); err == nil {
		t.Fatalf("expected unknown shell output error")
	}
	if _, err := mgr.kill("unknown"); err == nil {
		t.Fatalf("expected unknown shell kill error")
	}
}

func TestAgentAndWebInputValidationBranches(t *testing.T) {
	t.Parallel()

	tool := agentTool(&fakeRunner{})
	if _, err := tool.Handler(context.Background(), json.RawMessage(`{"prompt":""}`)); err == nil {
		t.Fatalf("expected empty prompt error")
	}

	search := webSearchTool(&fakeWeb{})
	if _, err := search.Handler(context.Background(), json.RawMessage(`{"query":""}`)); err == nil {
		t.Fatalf("expected empty search query error")
	}

	fetch := webFetchTool(&fakeWeb{})
	if _, err := fetch.Handler(context.Background(), json.RawMessage(`{"url":""}`)); err == nil {
		t.Fatalf("expected empty url error")
	}

	agentic := agenticFetchTool(&fakeWeb{}, &fakeRunner{})
	if _, err := agentic.Handler(context.Background(), json.RawMessage(`{"prompt":""}`)); err == nil {
		t.Fatalf("expected empty prompt error")
	}
}

func TestDynamicMCPToolsErrorBranch(t *testing.T) {
	t.Parallel()

	bad := &badMCP{}
	if _, err := dynamicMCPTools(context.Background(), bad); err == nil {
		t.Fatalf("expected dynamic mcp list error")
	}
}

type badMCP struct{}

func (b *badMCP) ListResources(context.Context, string) ([]MCPResource, error) { return nil, nil }
func (b *badMCP) ReadResource(context.Context, string, string) (string, error) { return "", nil }
func (b *badMCP) ListTools(context.Context) (map[string][]MCPToolDefinition, error) {
	return nil, errors.New("list tools failed")
}
func (b *badMCP) CallTool(context.Context, string, string, json.RawMessage) (string, error) {
	return "", nil
}
