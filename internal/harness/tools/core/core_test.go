package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	tools "go-agent-harness/internal/harness/tools"
)

// TestReadTool_Definition verifies the read tool constructor returns a valid tool.
func TestReadTool_Definition(t *testing.T) {
	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "read", tools.TierCore)
}

// TestReadTool_Handler_MissingPath verifies read returns an error when path is empty.
func TestReadTool_Handler_MissingPath(t *testing.T) {
	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// TestReadTool_Handler_Success verifies read returns file content.
func TestReadTool_Handler_Success(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ReadTool(tools.BuildOptions{WorkspaceRoot: dir})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestWriteTool_Definition verifies the write tool constructor.
func TestWriteTool_Definition(t *testing.T) {
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "write", tools.TierCore)
}

// TestWriteTool_Handler_MissingPath verifies write returns an error when path is empty.
func TestWriteTool_Handler_MissingPath(t *testing.T) {
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"content":"x"}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// TestWriteTool_Handler_MissingContent verifies write returns an error when content is missing.
func TestWriteTool_Handler_MissingContent(t *testing.T) {
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"foo.txt"}`))
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

// TestWriteTool_Handler_Success verifies write creates a file.
func TestWriteTool_Handler_Success(t *testing.T) {
	dir := t.TempDir()
	tool := WriteTool(tools.BuildOptions{WorkspaceRoot: dir})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"out.txt","content":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

// TestEditTool_Definition verifies the edit tool constructor.
func TestEditTool_Definition(t *testing.T) {
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "edit", tools.TierCore)
}

// TestEditTool_Handler_MissingPath verifies edit returns an error when path is empty.
func TestEditTool_Handler_MissingPath(t *testing.T) {
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"old_text":"a","new_text":"b"}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// TestEditTool_Handler_Success verifies edit replaces text in a file.
func TestEditTool_Handler_Success(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("foo bar"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditTool(tools.BuildOptions{WorkspaceRoot: dir})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"f.txt","old_text":"foo","new_text":"baz"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(data) != "baz bar" {
		t.Errorf("expected 'baz bar', got %q", string(data))
	}
}

// TestGlobTool_Definition verifies the glob tool constructor.
func TestGlobTool_Definition(t *testing.T) {
	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "glob", tools.TierCore)
}

// TestGlobTool_Handler_MissingPattern verifies glob returns an error when pattern is empty.
func TestGlobTool_Handler_MissingPattern(t *testing.T) {
	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

// TestGlobTool_Handler_Success verifies glob finds files.
func TestGlobTool_Handler_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte(""), 0o644)
	tool := GlobTool(tools.BuildOptions{WorkspaceRoot: dir})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"pattern":"*.go"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestGrepTool_Definition verifies the grep tool constructor.
func TestGrepTool_Definition(t *testing.T) {
	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "grep", tools.TierCore)
}

// TestGrepTool_Handler_MissingQuery verifies grep returns an error when query is empty.
func TestGrepTool_Handler_MissingQuery(t *testing.T) {
	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

// TestGrepTool_Handler_Success verifies grep finds matching text.
func TestGrepTool_Handler_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "data.txt"), []byte("needle in haystack"), 0o644)
	tool := GrepTool(tools.BuildOptions{WorkspaceRoot: dir})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"needle"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestLsTool_Definition verifies the ls tool constructor.
func TestLsTool_Definition(t *testing.T) {
	tool := LsTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "ls", tools.TierCore)
}

// TestLsTool_Handler_Success verifies ls lists directory contents.
func TestLsTool_Handler_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte(""), 0o644)
	tool := LsTool(tools.BuildOptions{WorkspaceRoot: dir})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestGitStatusTool_Definition verifies the git_status tool constructor.
func TestGitStatusTool_Definition(t *testing.T) {
	tool := GitStatusTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "git_status", tools.TierCore)
}

// TestGitDiffTool_Definition verifies the git_diff tool constructor.
func TestGitDiffTool_Definition(t *testing.T) {
	tool := GitDiffTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "git_diff", tools.TierCore)
}

// TestApplyPatchTool_Definition verifies the apply_patch tool constructor.
func TestApplyPatchTool_Definition(t *testing.T) {
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "apply_patch", tools.TierCore)
}

// TestApplyPatchTool_Handler_MissingPath verifies apply_patch returns an error when path is empty.
func TestApplyPatchTool_Handler_MissingPath(t *testing.T) {
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"find":"x","replace":"y"}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// TestApplyPatchTool_Handler_FindReplace verifies apply_patch find/replace mode.
func TestApplyPatchTool_Handler_FindReplace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "patch.txt"), []byte("hello world"), 0o644)
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"path":"patch.txt","find":"hello","replace":"goodbye"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "patch.txt"))
	if string(data) != "goodbye world" {
		t.Errorf("expected 'goodbye world', got %q", string(data))
	}
}

// TestApplyPatchTool_Handler_UnifiedPatch verifies apply_patch unified patch mode.
func TestApplyPatchTool_Handler_UnifiedPatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "u.txt"), []byte("line1\nline2\nline3\n"), 0o644)
	patch := `*** Begin Patch
*** Update File: u.txt
@@ context
 line1
-line2
+lineXX
 line3
*** End Patch`
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args, _ := json.Marshal(map[string]string{"patch": patch})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestApplyPatchTool_Handler_MultiEdit verifies apply_patch multi-edit mode.
func TestApplyPatchTool_Handler_MultiEdit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "me.txt"), []byte("aaa bbb ccc"), 0o644)
	tool := ApplyPatchTool(tools.BuildOptions{WorkspaceRoot: dir})
	args := `{"path":"me.txt","edits":[{"old_text":"aaa","new_text":"xxx"},{"old_text":"ccc","new_text":"zzz"}]}`
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "me.txt"))
	if string(data) != "xxx bbb zzz" {
		t.Errorf("expected 'xxx bbb zzz', got %q", string(data))
	}
}

// TestBashTool_Definition verifies the bash tool constructor.
func TestBashTool_Definition(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	assertToolDef(t, tool, "bash", tools.TierCore)
}

// TestBashTool_Handler_EmptyCommand verifies bash returns an error for empty command.
func TestBashTool_Handler_EmptyCommand(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":""}`))
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

// TestBashTool_Handler_Success verifies bash runs a simple command.
func TestBashTool_Handler_Success(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestJobOutputTool_Definition verifies the job_output tool constructor.
func TestJobOutputTool_Definition(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobOutputTool(jm)
	assertToolDef(t, tool, "job_output", tools.TierCore)
}

// TestJobOutputTool_Handler_MissingShellID verifies job_output returns an error when shell_id is empty.
func TestJobOutputTool_Handler_MissingShellID(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobOutputTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing shell_id")
	}
}

// TestJobKillTool_Definition verifies the job_kill tool constructor.
func TestJobKillTool_Definition(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobKillTool(jm)
	assertToolDef(t, tool, "job_kill", tools.TierCore)
}

// TestJobKillTool_Handler_MissingShellID verifies job_kill returns an error when shell_id is empty.
func TestJobKillTool_Handler_MissingShellID(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobKillTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing shell_id")
	}
}

// TestAskUserQuestionTool_Definition verifies the ask_user_question tool constructor.
func TestAskUserQuestionTool_Definition(t *testing.T) {
	tool := AskUserQuestionTool(nil, 30*time.Second)
	assertToolDef(t, tool, tools.AskUserQuestionToolName, tools.TierCore)
}

// TestAskUserQuestionTool_Handler_NilBroker verifies ask_user_question returns an error when broker is nil.
func TestAskUserQuestionTool_Handler_NilBroker(t *testing.T) {
	tool := AskUserQuestionTool(nil, 30*time.Second)
	args := `{"questions":[{"question":"What?","header":"H","options":[{"label":"A","description":"a"},{"label":"B","description":"b"}],"multiSelect":false}]}`
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for nil broker")
	}
}

// TestObservationalMemoryTool_Definition verifies the observational_memory tool constructor.
func TestObservationalMemoryTool_Definition(t *testing.T) {
	tool := ObservationalMemoryTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	assertToolDef(t, tool, "observational_memory", tools.TierCore)
}

// TestObservationalMemoryTool_Handler_MissingAction verifies observational_memory returns an error when action is empty.
func TestObservationalMemoryTool_Handler_MissingAction(t *testing.T) {
	tool := ObservationalMemoryTool(tools.BuildOptions{WorkspaceRoot: t.TempDir()})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing action")
	}
}

// ---------- observational_memory helper functions ----------

// TestConfigFromArgs_Nil verifies configFromArgs returns nil when input is nil.
func TestConfigFromArgs_Nil(t *testing.T) {
	result := configFromArgs(nil)
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

// TestConfigFromArgs_NonNil verifies configFromArgs returns a populated config.
func TestConfigFromArgs_NonNil(t *testing.T) {
	input := &struct {
		ObserveMinTokens       int `json:"observe_min_tokens"`
		SnippetMaxTokens       int `json:"snippet_max_tokens"`
		ReflectThresholdTokens int `json:"reflect_threshold_tokens"`
	}{
		ObserveMinTokens:       100,
		SnippetMaxTokens:       500,
		ReflectThresholdTokens: 1000,
	}
	result := configFromArgs(input)
	if result == nil {
		t.Fatal("expected non-nil config")
	}
	if result.ObserveMinTokens != 100 {
		t.Errorf("expected ObserveMinTokens=100, got %d", result.ObserveMinTokens)
	}
	if result.SnippetMaxTokens != 500 {
		t.Errorf("expected SnippetMaxTokens=500, got %d", result.SnippetMaxTokens)
	}
	if result.ReflectThresholdTokens != 1000 {
		t.Errorf("expected ReflectThresholdTokens=1000, got %d", result.ReflectThresholdTokens)
	}
}

// TestMemoryScopeFromMetadata_Defaults verifies memoryScopeFromMetadata fills defaults.
func TestMemoryScopeFromMetadata_Defaults(t *testing.T) {
	scope := memoryScopeFromMetadata("run-123", tools.RunMetadata{})
	if scope.TenantID != "default" {
		t.Errorf("expected TenantID='default', got %q", scope.TenantID)
	}
	if scope.AgentID != "default" {
		t.Errorf("expected AgentID='default', got %q", scope.AgentID)
	}
	if scope.ConversationID != "run-123" {
		t.Errorf("expected ConversationID='run-123', got %q", scope.ConversationID)
	}
}

// TestMemoryScopeFromMetadata_Provided verifies memoryScopeFromMetadata uses provided values.
func TestMemoryScopeFromMetadata_Provided(t *testing.T) {
	md := tools.RunMetadata{
		TenantID:       "t1",
		ConversationID: "c1",
		AgentID:        "a1",
	}
	scope := memoryScopeFromMetadata("run-123", md)
	if scope.TenantID != "t1" {
		t.Errorf("expected TenantID='t1', got %q", scope.TenantID)
	}
	if scope.AgentID != "a1" {
		t.Errorf("expected AgentID='a1', got %q", scope.AgentID)
	}
	if scope.ConversationID != "c1" {
		t.Errorf("expected ConversationID='c1', got %q", scope.ConversationID)
	}
}

// TestSanitizePathPart verifies sanitizePathPart normalizes path parts.
func TestSanitizePathPart(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "default"},
		{"  ", "default"},
		{"hello", "hello"},
		{"foo/bar", "foo-bar"},
		{"a..b", "a-b"},
		{"a b c", "a-b-c"},
	}
	for _, tt := range tests {
		got := sanitizePathPart(tt.input)
		if got != tt.want {
			t.Errorf("sanitizePathPart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// assertToolDef is a test helper that checks a tool has the expected name, tier, and a non-nil handler.
func assertToolDef(t *testing.T, tool tools.Tool, expectedName string, expectedTier tools.ToolTier) {
	t.Helper()
	if tool.Definition.Name != expectedName {
		t.Errorf("expected name %q, got %q", expectedName, tool.Definition.Name)
	}
	if tool.Definition.Tier != expectedTier {
		t.Errorf("expected tier %q, got %q", expectedTier, tool.Definition.Tier)
	}
	if tool.Handler == nil {
		t.Error("handler is nil")
	}
	if tool.Definition.Parameters == nil {
		t.Error("parameters is nil")
	}
}
