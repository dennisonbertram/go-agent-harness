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
	if _, err := ResolveWorkspacePath("", "a"); err == nil {
		t.Fatalf("expected missing workspace root error")
	}
	// Absolute paths are now passed through directly (intentional: container environments).
	if got, err := ResolveWorkspacePath(workspace, "/abs"); err != nil || got != "/abs" {
		t.Fatalf("expected absolute path to pass through, got %q err %v", got, err)
	}
	if _, err := ResolveWorkspacePath(workspace, "../escape"); err == nil {
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

func TestJobManagerOutputHeadTailBuffer(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)
	mgr.maxOutputBytes = 256

	cmd := "i=0; while [ $i -lt 300 ]; do printf 'line-%04d\\n' $i; i=$((i+1)); done"
	started, err := mgr.runBackground(context.Background(), cmd, 5, ".")
	if err != nil {
		t.Fatalf("runBackground: %v", err)
	}
	shellID, _ := started["shell_id"].(string)
	if shellID == "" {
		t.Fatalf("expected shell_id in response")
	}

	out, err := mgr.output(shellID, true)
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	output, _ := out["output"].(string)
	if output == "" {
		t.Fatalf("expected output")
	}
	if !strings.Contains(output, "line-0000") {
		t.Fatalf("expected head content to be preserved: %s", output)
	}
	if !strings.Contains(output, "line-0299") {
		t.Fatalf("expected tail content to be preserved: %s", output)
	}
	if strings.Contains(output, "line-0150") {
		t.Fatalf("expected middle content to be omitted: %s", output)
	}
	if !strings.Contains(output, "[truncated output]") {
		t.Fatalf("expected truncation marker in output: %s", output)
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

// ---------- ApplyPolicy exported wrapper tests ----------

type allowPolicy struct{}

func (a allowPolicy) Allow(_ context.Context, _ PolicyInput) (PolicyDecision, error) {
	return PolicyDecision{Allow: true}, nil
}

type denyPolicy struct{ reason string }

func (d denyPolicy) Allow(_ context.Context, _ PolicyInput) (PolicyDecision, error) {
	return PolicyDecision{Allow: false, Reason: d.reason}, nil
}

// TestApplyPolicy_FullAuto verifies ApplyPolicy in full-auto mode passes through.
func TestApplyPolicy_FullAuto(t *testing.T) {
	called := false
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		called = true
		return "ok", nil
	}
	def := Definition{Name: "test", Action: ActionWrite, Mutating: true}
	wrapped := ApplyPolicy(def, ApprovalModeFullAuto, nil, handler)
	result, err := wrapped(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler not called")
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

// TestApplyPolicy_Permissions_NilPolicy verifies ApplyPolicy returns permission_denied with nil policy.
func TestApplyPolicy_Permissions_NilPolicy(t *testing.T) {
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		return "should not run", nil
	}
	def := Definition{Name: "test", Action: ActionWrite, Mutating: true}
	wrapped := ApplyPolicy(def, ApprovalModePermissions, nil, handler)
	result, err := wrapped(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "permission_denied") {
		t.Fatalf("expected permission_denied, got %s", result)
	}
}

// TestApplyPolicy_Permissions_AllowPolicy verifies ApplyPolicy passes through on allow.
func TestApplyPolicy_Permissions_AllowPolicy(t *testing.T) {
	called := false
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		called = true
		return "allowed", nil
	}
	def := Definition{Name: "test", Action: ActionWrite, Mutating: true}
	wrapped := ApplyPolicy(def, ApprovalModePermissions, allowPolicy{}, handler)
	result, err := wrapped(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler not called")
	}
	if result != "allowed" {
		t.Errorf("expected 'allowed', got %q", result)
	}
}

// TestApplyPolicy_Permissions_DenyPolicy verifies ApplyPolicy returns permission_denied on deny.
func TestApplyPolicy_Permissions_DenyPolicy(t *testing.T) {
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		return "should not run", nil
	}
	def := Definition{Name: "test", Action: ActionWrite, Mutating: true}
	wrapped := ApplyPolicy(def, ApprovalModePermissions, denyPolicy{reason: "not allowed"}, handler)
	result, err := wrapped(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "permission_denied") {
		t.Fatalf("expected permission_denied, got %s", result)
	}
	if !strings.Contains(result, "not allowed") {
		t.Fatalf("expected reason 'not allowed' in result, got %s", result)
	}
}

// TestApplyPolicy_ReadAction_SkipsPolicy verifies ApplyPolicy skips policy for read actions.
func TestApplyPolicy_ReadAction_SkipsPolicy(t *testing.T) {
	called := false
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		called = true
		return "read", nil
	}
	def := Definition{Name: "test", Action: ActionRead}
	wrapped := ApplyPolicy(def, ApprovalModePermissions, denyPolicy{reason: "denied"}, handler)
	result, err := wrapped(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler not called for read action")
	}
	if result != "read" {
		t.Errorf("expected 'read', got %q", result)
	}
}

// TestApplyPolicy_DenyWithEmptyReason verifies default reason when policy provides none.
func TestApplyPolicy_DenyWithEmptyReason(t *testing.T) {
	handler := func(ctx context.Context, args json.RawMessage) (string, error) {
		return "nope", nil
	}
	def := Definition{Name: "test", Action: ActionWrite, Mutating: true}
	wrapped := ApplyPolicy(def, ApprovalModePermissions, denyPolicy{reason: ""}, handler)
	result, err := wrapped(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "policy denied") {
		t.Fatalf("expected default 'policy denied' reason, got %s", result)
	}
}

// ---------- JobManager exported wrapper tests ----------

// TestJobManager_RunForeground verifies the RunForeground exported wrapper.
func TestJobManager_RunForeground(t *testing.T) {
	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)
	result, err := mgr.RunForeground(context.Background(), "echo hello", 5, ".")
	if err != nil {
		t.Fatalf("RunForeground: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestJobManager_RunBackground verifies the RunBackground exported wrapper.
func TestJobManager_RunBackground(t *testing.T) {
	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)
	result, err := mgr.RunBackground("echo bg", 5, ".")
	if err != nil {
		t.Fatalf("RunBackground: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	shellID, ok := result["shell_id"].(string)
	if !ok || shellID == "" {
		t.Fatal("expected shell_id in result")
	}

	// Wait for the process to complete
	time.Sleep(100 * time.Millisecond)

	// Test Output exported wrapper
	outResult, err := mgr.Output(shellID, false)
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if outResult == nil {
		t.Fatal("expected non-nil output result")
	}
}

// TestJobManager_Output_Unknown verifies Output returns error for unknown shell_id.
func TestJobManager_Output_Unknown(t *testing.T) {
	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)
	_, err := mgr.Output("nonexistent", false)
	if err == nil {
		t.Fatal("expected error for unknown shell_id")
	}
}

// TestJobManager_Kill_Unknown verifies Kill returns error for unknown shell_id.
func TestJobManager_Kill_Unknown(t *testing.T) {
	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)
	_, err := mgr.Kill("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown shell_id")
	}
}
