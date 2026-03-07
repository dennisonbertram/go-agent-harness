package harness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadWriteEditTools(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	registry := NewDefaultRegistry(workspace)

	writeOut, err := registry.Execute(context.Background(), "write", []byte(`{"path":"notes.txt","content":"hello world"}`))
	if err != nil {
		t.Fatalf("write tool failed: %v", err)
	}
	var writeResult struct {
		BytesWritten int `json:"bytes_written"`
	}
	if err := json.Unmarshal([]byte(writeOut), &writeResult); err != nil {
		t.Fatalf("unmarshal write output: %v", err)
	}
	if writeResult.BytesWritten == 0 {
		t.Fatalf("expected bytes written")
	}

	readOut, err := registry.Execute(context.Background(), "read", []byte(`{"path":"notes.txt"}`))
	if err != nil {
		t.Fatalf("read tool failed: %v", err)
	}
	var readResult struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(readOut), &readResult); err != nil {
		t.Fatalf("unmarshal read output: %v", err)
	}
	if readResult.Content != "hello world" {
		t.Fatalf("unexpected read content: %q", readResult.Content)
	}

	editOut, err := registry.Execute(context.Background(), "edit", []byte(`{"path":"notes.txt","old_text":"world","new_text":"agent"}`))
	if err != nil {
		t.Fatalf("edit tool failed: %v", err)
	}
	var editResult struct {
		Replacements int `json:"replacements"`
	}
	if err := json.Unmarshal([]byte(editOut), &editResult); err != nil {
		t.Fatalf("unmarshal edit output: %v", err)
	}
	if editResult.Replacements != 1 {
		t.Fatalf("expected 1 replacement, got %d", editResult.Replacements)
	}

	readEditedOut, err := registry.Execute(context.Background(), "read", []byte(`{"path":"notes.txt"}`))
	if err != nil {
		t.Fatalf("read edited file failed: %v", err)
	}
	if err := json.Unmarshal([]byte(readEditedOut), &readResult); err != nil {
		t.Fatalf("unmarshal read edited output: %v", err)
	}
	if readResult.Content != "hello agent" {
		t.Fatalf("unexpected edited content: %q", readResult.Content)
	}

	if _, err := registry.Execute(context.Background(), "read", []byte(`{"path":"../secret.txt"}`)); err == nil {
		t.Fatalf("expected workspace boundary error for read")
	}
	if _, err := registry.Execute(context.Background(), "write", []byte(`{"path":"../secret.txt","content":"x"}`)); err == nil {
		t.Fatalf("expected workspace boundary error for write")
	}
	if _, err := registry.Execute(context.Background(), "edit", []byte(`{"path":"../secret.txt","old_text":"x","new_text":"y"}`)); err == nil {
		t.Fatalf("expected workspace boundary error for edit")
	}
}

func TestEditToolFailsWhenTargetMissing(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "notes.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	registry := NewDefaultRegistry(workspace)
	if _, err := registry.Execute(context.Background(), "edit", []byte(`{"path":"notes.txt","old_text":"beta","new_text":"gamma"}`)); err == nil {
		t.Fatalf("expected missing target error")
	}
}

func TestApplyPatchToolAcceptsUnifiedPatchPayload(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "retry.go"), []byte("package retry\n\nfunc schedule() string {\n\treturn \"old\"\n}\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	registry := NewDefaultRegistry(workspace)
	patch := `{"patch":"*** Begin Patch\n*** Update File: retry.go\n@@\n-package retry\n-\n-func schedule() string {\n-\treturn \"old\"\n-}\n+package retry\n+\n+func schedule() string {\n+\treturn \"new\"\n+}\n*** End Patch"}`
	if _, err := registry.Execute(context.Background(), "apply_patch", []byte(patch)); err != nil {
		t.Fatalf("apply_patch unified diff failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workspace, "retry.go"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if !strings.Contains(string(content), `"new"`) {
		t.Fatalf("expected updated file content, got %q", string(content))
	}
}

func TestWriteToolAcceptsContentAliases(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	registry := NewDefaultRegistry(workspace)

	if _, err := registry.Execute(context.Background(), "write", []byte(`{"path":"notes.txt","new_text":"hello alias"}`)); err != nil {
		t.Fatalf("write tool with new_text failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workspace, "notes.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello alias" {
		t.Fatalf("unexpected file content: %q", string(content))
	}
}

func TestBashTool(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	registry := NewDefaultRegistry(workspace)

	out, err := registry.Execute(context.Background(), "bash", []byte(`{"command":"printf 'ok'","timeout_seconds":10}`))
	if err != nil {
		t.Fatalf("bash tool failed: %v", err)
	}
	var bashResult struct {
		ExitCode int    `json:"exit_code"`
		Output   string `json:"output"`
	}
	if err := json.Unmarshal([]byte(out), &bashResult); err != nil {
		t.Fatalf("unmarshal bash output: %v", err)
	}
	if bashResult.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", bashResult.ExitCode)
	}
	if bashResult.Output != "ok" {
		t.Fatalf("unexpected output: %q", bashResult.Output)
	}

	out, err = registry.Execute(context.Background(), "bash", []byte(`{"command":"exit 7"}`))
	if err != nil {
		t.Fatalf("bash non-zero run failed unexpectedly: %v", err)
	}
	if err := json.Unmarshal([]byte(out), &bashResult); err != nil {
		t.Fatalf("unmarshal bash non-zero output: %v", err)
	}
	if bashResult.ExitCode != 7 {
		t.Fatalf("expected exit 7, got %d", bashResult.ExitCode)
	}

	if _, err := registry.Execute(context.Background(), "bash", []byte(`{"command":"rm -rf /"}`)); err == nil {
		t.Fatalf("expected dangerous command rejection")
	}
}

func TestLsAndGlobTools(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write src/main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	registry := NewDefaultRegistry(workspace)

	lsOut, err := registry.Execute(context.Background(), "ls", []byte(`{"path":".","max_entries":10}`))
	if err != nil {
		t.Fatalf("ls tool failed: %v", err)
	}
	var lsResult struct {
		Entries []string `json:"entries"`
	}
	if err := json.Unmarshal([]byte(lsOut), &lsResult); err != nil {
		t.Fatalf("unmarshal ls output: %v", err)
	}
	if len(lsResult.Entries) < 2 {
		t.Fatalf("expected ls entries, got %v", lsResult.Entries)
	}

	globOut, err := registry.Execute(context.Background(), "glob", []byte(`{"pattern":"src/*.go"}`))
	if err != nil {
		t.Fatalf("glob tool failed: %v", err)
	}
	var globResult struct {
		Matches []string `json:"matches"`
	}
	if err := json.Unmarshal([]byte(globOut), &globResult); err != nil {
		t.Fatalf("unmarshal glob output: %v", err)
	}
	if len(globResult.Matches) != 1 || globResult.Matches[0] != "src/main.go" {
		t.Fatalf("unexpected glob matches: %v", globResult.Matches)
	}

	if _, err := registry.Execute(context.Background(), "ls", []byte(`{"path":"../"}`)); err == nil {
		t.Fatalf("expected ls boundary error")
	}
	if _, err := registry.Execute(context.Background(), "glob", []byte(`{"pattern":"../*.txt"}`)); err == nil {
		t.Fatalf("expected glob boundary error")
	}
}

func TestGrepTool(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	content := "alpha\nneedle here\nbeta needle\n"
	if err := os.WriteFile(filepath.Join(workspace, "docs", "notes.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	registry := NewDefaultRegistry(workspace)
	grepOut, err := registry.Execute(context.Background(), "grep", []byte(`{"query":"needle","path":"docs"}`))
	if err != nil {
		t.Fatalf("grep tool failed: %v", err)
	}
	var grepResult struct {
		Matches []struct {
			Path       string `json:"path"`
			LineNumber int    `json:"line_number"`
			Line       string `json:"line"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(grepOut), &grepResult); err != nil {
		t.Fatalf("unmarshal grep output: %v", err)
	}
	if len(grepResult.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d (%v)", len(grepResult.Matches), grepResult.Matches)
	}

	if _, err := registry.Execute(context.Background(), "grep", []byte(`{"query":"needle","path":"../"}`)); err == nil {
		t.Fatalf("expected grep boundary error")
	}
}

func TestApplyPatchTool(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "patch.txt")
	if err := os.WriteFile(filePath, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("write patch.txt: %v", err)
	}

	registry := NewDefaultRegistry(workspace)
	out, err := registry.Execute(context.Background(), "apply_patch", []byte(`{"path":"patch.txt","find":"two","replace":"TWO"}`))
	if err != nil {
		t.Fatalf("apply_patch failed: %v", err)
	}
	var result struct {
		Replacements int `json:"replacements"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal apply_patch output: %v", err)
	}
	if result.Replacements != 1 {
		t.Fatalf("expected 1 replacement, got %d", result.Replacements)
	}
	updated, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if !strings.Contains(string(updated), "TWO") {
		t.Fatalf("expected updated content, got %q", string(updated))
	}

	if _, err := registry.Execute(context.Background(), "apply_patch", []byte(`{"path":"patch.txt","find":"missing","replace":"x"}`)); err == nil {
		t.Fatalf("expected missing target error")
	}
	if _, err := registry.Execute(context.Background(), "apply_patch", []byte(`{"path":"../patch.txt","find":"one","replace":"ONE"}`)); err == nil {
		t.Fatalf("expected boundary error")
	}
}

func TestGitStatusAndGitDiffTools(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	workspace := t.TempDir()
	runGit(t, workspace, "init")
	runGit(t, workspace, "config", "user.email", "test@example.com")
	runGit(t, workspace, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	runGit(t, workspace, "add", "a.txt")
	runGit(t, workspace, "commit", "-m", "init")

	registry := NewDefaultRegistry(workspace)

	statusOut, err := registry.Execute(context.Background(), "git_status", []byte(`{}`))
	if err != nil {
		t.Fatalf("git_status failed: %v", err)
	}
	var statusResult struct {
		Clean bool `json:"clean"`
	}
	if err := json.Unmarshal([]byte(statusOut), &statusResult); err != nil {
		t.Fatalf("unmarshal git_status output: %v", err)
	}
	if !statusResult.Clean {
		t.Fatalf("expected clean repo")
	}

	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modify a.txt: %v", err)
	}

	statusOut, err = registry.Execute(context.Background(), "git_status", []byte(`{}`))
	if err != nil {
		t.Fatalf("git_status after change failed: %v", err)
	}
	if err := json.Unmarshal([]byte(statusOut), &statusResult); err != nil {
		t.Fatalf("unmarshal git_status changed output: %v", err)
	}
	if statusResult.Clean {
		t.Fatalf("expected dirty repo")
	}

	diffOut, err := registry.Execute(context.Background(), "git_diff", []byte(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatalf("git_diff failed: %v", err)
	}
	var diffResult struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal([]byte(diffOut), &diffResult); err != nil {
		t.Fatalf("unmarshal git_diff output: %v", err)
	}
	if !strings.Contains(diffResult.Diff, "changed") {
		t.Fatalf("expected diff to contain change, got %q", diffResult.Diff)
	}
}

func TestLsRecursiveHiddenAndTruncation(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "nested", "deep"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".hidden.txt"), []byte("h"), 0o644); err != nil {
		t.Fatalf("write hidden file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "nested", "deep", "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write deep file: %v", err)
	}

	registry := NewDefaultRegistry(workspace)
	out, err := registry.Execute(context.Background(), "ls", []byte(`{"path":".","recursive":true,"include_hidden":true,"max_entries":1}`))
	if err != nil {
		t.Fatalf("ls recursive failed: %v", err)
	}
	var result struct {
		Truncated bool `json:"truncated"`
		Entries   []string
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal ls recursive output: %v", err)
	}
	if !result.Truncated {
		t.Fatalf("expected truncation")
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected one entry due truncation, got %d", len(result.Entries))
	}
}

func TestGlobAndGrepValidationAndRegexBranches(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "logs"), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "logs", "app.log"), []byte("Error: boom\ninfo line\n"), 0o644); err != nil {
		t.Fatalf("write app.log: %v", err)
	}
	registry := NewDefaultRegistry(workspace)

	if _, err := registry.Execute(context.Background(), "glob", []byte(`{"pattern":"["}`)); err == nil {
		t.Fatalf("expected invalid glob pattern error")
	}
	if _, err := registry.Execute(context.Background(), "grep", []byte(`{"query":"(","regex":true}`)); err == nil {
		t.Fatalf("expected regex compile error")
	}

	out, err := registry.Execute(context.Background(), "grep", []byte(`{"query":"error:.*","regex":true,"case_sensitive":false}`))
	if err != nil {
		t.Fatalf("grep regex failed: %v", err)
	}
	var result struct {
		Matches []map[string]any `json:"matches"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal grep regex output: %v", err)
	}
	if len(result.Matches) == 0 {
		t.Fatalf("expected regex matches")
	}
}

func TestApplyPatchReplaceAllAndGitDiffTruncation(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	workspace := t.TempDir()
	runGit(t, workspace, "init")
	runGit(t, workspace, "config", "user.email", "test@example.com")
	runGit(t, workspace, "config", "user.name", "Test User")

	content := strings.Repeat("line\n", 20)
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	runGit(t, workspace, "add", "a.txt")
	runGit(t, workspace, "commit", "-m", "init")

	registry := NewDefaultRegistry(workspace)

	patchOut, err := registry.Execute(context.Background(), "apply_patch", []byte(`{"path":"a.txt","find":"line","replace":"LINE","replace_all":true}`))
	if err != nil {
		t.Fatalf("apply_patch replace_all failed: %v", err)
	}
	var patchResult struct {
		Replacements int `json:"replacements"`
	}
	if err := json.Unmarshal([]byte(patchOut), &patchResult); err != nil {
		t.Fatalf("unmarshal patch output: %v", err)
	}
	if patchResult.Replacements < 2 {
		t.Fatalf("expected multiple replacements, got %d", patchResult.Replacements)
	}

	diffOut, err := registry.Execute(context.Background(), "git_diff", []byte(`{"path":"a.txt","max_bytes":40}`))
	if err != nil {
		t.Fatalf("git_diff truncation failed: %v", err)
	}
	var diffResult struct {
		Diff      string `json:"diff"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(diffOut), &diffResult); err != nil {
		t.Fatalf("unmarshal diff output: %v", err)
	}
	if !diffResult.Truncated {
		t.Fatalf("expected truncated diff")
	}
}

func TestInternalHelpersAndRunCommandBranches(t *testing.T) {
	t.Parallel()

	if err := validateWorkspaceRelativePattern("../bad"); err == nil {
		t.Fatalf("expected pattern escape error")
	}
	if err := validateWorkspaceRelativePattern("*.go"); err != nil {
		t.Fatalf("expected valid pattern: %v", err)
	}

	if _, err := buildLineMatcher("(", true, false); err == nil {
		t.Fatalf("expected regex compile error")
	}
	matcher, err := buildLineMatcher("Needle", false, false)
	if err != nil {
		t.Fatalf("build matcher: %v", err)
	}
	if !matcher("contains needle") {
		t.Fatalf("expected case-insensitive match")
	}

	if _, _, timedOut, err := runCommand(context.Background(), 20*time.Millisecond, "bash", "-lc", "sleep 0.2"); err == nil || !timedOut {
		t.Fatalf("expected timeout error branch")
	}
	output, exitCode, timedOut, err := runCommand(context.Background(), 2*time.Second, "bash", "-lc", "echo hi; exit 3")
	if err != nil {
		t.Fatalf("expected non-zero exit to be handled without error: %v", err)
	}
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d", exitCode)
	}
	if timedOut {
		t.Fatalf("did not expect timeout")
	}
	if !strings.Contains(output, "hi") {
		t.Fatalf("expected command output, got %q", output)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}
