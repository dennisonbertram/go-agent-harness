package training

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	initGitRepoWithBranch(t, dir, "main")
}

func initGitRepoWithBranch(t *testing.T, dir, branch string) {
	t.Helper()
	initArgs := []string{"init"}
	if branch != "" {
		initArgs = []string{"init", "-b", branch}
	}
	for _, args := range [][]string{
		initArgs,
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// Create an initial commit so we have a valid HEAD
	dummy := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(dummy, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", ".gitkeep"},
		{"commit", "-m", "initial"},
		{"branch", "-M", "main"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func currentGitBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --show-current: %v\n%s", err, out)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		t.Fatal("expected non-empty current git branch")
	}
	return branch
}

func TestCanAutoApply_CertainHighEvidence(t *testing.T) {
	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
	}, nil)

	f := Finding{
		Type:          "system_prompt",
		Priority:      "high",
		Confidence:    ConfidenceCertain,
		EvidenceCount: 5,
	}
	ok, reason := a.canAutoApply(f)
	if !ok {
		t.Errorf("expected canAutoApply=true, got false: %s", reason)
	}
}

func TestCanAutoApply_LowConfidenceSkipped(t *testing.T) {
	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
	}, nil)

	f := Finding{
		Type:          "system_prompt",
		Priority:      "high",
		Confidence:    ConfidenceProbable,
		EvidenceCount: 5,
	}
	ok, reason := a.canAutoApply(f)
	if ok {
		t.Error("expected canAutoApply=false for PROBABLE confidence")
	}
	if !strings.Contains(reason, "confidence") {
		t.Errorf("expected reason to mention confidence, got: %s", reason)
	}
}

func TestCanAutoApply_CriticalPriorityNeedsHumanReview(t *testing.T) {
	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
	}, nil)

	f := Finding{
		Type:          "system_prompt",
		Priority:      "critical",
		Confidence:    ConfidenceCertain,
		EvidenceCount: 10,
	}
	ok, reason := a.canAutoApply(f)
	if ok {
		t.Error("expected canAutoApply=false for critical priority")
	}
	if !strings.Contains(reason, "critical") {
		t.Errorf("expected reason to mention critical, got: %s", reason)
	}
}

func TestCanAutoApply_InsufficientEvidence(t *testing.T) {
	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
	}, nil)

	f := Finding{
		Type:          "tool_description",
		Priority:      "medium",
		Confidence:    ConfidenceCertain,
		EvidenceCount: 1,
	}
	ok, reason := a.canAutoApply(f)
	if ok {
		t.Error("expected canAutoApply=false with only 1 evidence")
	}
	if !strings.Contains(reason, "evidence") {
		t.Errorf("expected reason to mention evidence, got: %s", reason)
	}
}

func TestCanAutoApply_BehaviorTypeSkipped(t *testing.T) {
	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
	}, nil)

	f := Finding{
		Type:          "behavior",
		Priority:      "high",
		Confidence:    ConfidenceCertain,
		EvidenceCount: 5,
	}
	ok, reason := a.canAutoApply(f)
	if ok {
		t.Error("expected canAutoApply=false for behavior type")
	}
	if !strings.Contains(reason, "type") {
		t.Errorf("expected reason to mention type, got: %s", reason)
	}
}

func TestCanAutoApply_ToolDescriptionAllowed(t *testing.T) {
	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
	}, nil)

	f := Finding{
		Type:          "tool_description",
		Priority:      "medium",
		Confidence:    ConfidenceCertain,
		EvidenceCount: 3,
	}
	ok, _ := a.canAutoApply(f)
	if !ok {
		t.Error("expected canAutoApply=true for tool_description with sufficient evidence")
	}
}

func TestApply_DryRunNoGitOps(t *testing.T) {
	dir := t.TempDir()

	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
		DryRun:              true,
		RepoPath:            dir,
	}, nil)

	findings := []Finding{
		{
			Type:          "system_prompt",
			Priority:      "high",
			Target:        "retry_behavior",
			Issue:         "no backoff",
			Proposed:      "Add exponential backoff instruction",
			Rationale:     "Seen in 5 runs",
			Confidence:    ConfidenceCertain,
			EvidenceCount: 5,
		},
		{
			Type:          "behavior",
			Priority:      "low",
			Target:        "greeting",
			Issue:         "too verbose",
			Proposed:      "shorten",
			Rationale:     "wastes tokens",
			Confidence:    ConfidenceCertain,
			EvidenceCount: 5,
		},
	}

	result, err := a.Apply(context.Background(), findings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if result.FindingsConsidered != 2 {
		t.Errorf("FindingsConsidered = %d, want 2", result.FindingsConsidered)
	}
	// system_prompt should be eligible, behavior should be skipped
	if result.FindingsApplied != 1 {
		t.Errorf("FindingsApplied = %d, want 1", result.FindingsApplied)
	}
	if result.FindingsSkipped != 1 {
		t.Errorf("FindingsSkipped = %d, want 1", result.FindingsSkipped)
	}
}

func TestApply_AllSkippedReturnsNoBranch(t *testing.T) {
	dir := t.TempDir()

	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
		DryRun:              false,
		RepoPath:            dir,
	}, nil)

	findings := []Finding{
		{
			Type:          "behavior",
			Priority:      "low",
			Confidence:    ConfidenceTentative,
			EvidenceCount: 1,
		},
	}

	result, err := a.Apply(context.Background(), findings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.FindingsApplied != 0 {
		t.Errorf("FindingsApplied = %d, want 0", result.FindingsApplied)
	}
	if result.BranchName != "" {
		t.Errorf("BranchName = %q, want empty", result.BranchName)
	}
}

func TestApplyFinding_SystemPrompt(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create prompts/behaviors/ directory
	behaviorsDir := filepath.Join(dir, "prompts", "behaviors")
	if err := os.MkdirAll(behaviorsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := NewApplier(ApplierConfig{
		RepoPath: dir,
	}, nil)

	f := Finding{
		Type:     "system_prompt",
		Target:   "retry_behavior",
		Proposed: "Always use exponential backoff when retrying.",
	}

	if err := a.applyFinding(f); err != nil {
		t.Fatalf("applyFinding: %v", err)
	}

	// Check that a file was created in prompts/behaviors/
	entries, err := os.ReadDir(behaviorsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected file in prompts/behaviors/")
	}

	content, err := os.ReadFile(filepath.Join(behaviorsDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "exponential backoff") {
		t.Errorf("file content does not contain proposed text: %s", content)
	}
}

func TestApplyFinding_ToolDescription(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create tool descriptions directory with a target file
	descDir := filepath.Join(dir, "internal", "harness", "tools", "descriptions")
	if err := os.MkdirAll(descDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(descDir, "bash.md")
	if err := os.WriteFile(target, []byte("# Bash\nRuns shell commands.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewApplier(ApplierConfig{
		RepoPath: dir,
	}, nil)

	f := Finding{
		Type:     "tool_description",
		Target:   "bash",
		Proposed: "Always validate exit codes before proceeding.",
	}

	if err := a.applyFinding(f); err != nil {
		t.Fatalf("applyFinding: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "validate exit codes") {
		t.Errorf("file content does not contain proposed text: %s", content)
	}
	if !strings.Contains(string(content), "<!-- training:") {
		t.Error("file content does not contain training marker")
	}
}

func TestApplyFinding_ToolDescriptionMissing(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create descriptions dir but no target file
	descDir := filepath.Join(dir, "internal", "harness", "tools", "descriptions")
	if err := os.MkdirAll(descDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := NewApplier(ApplierConfig{
		RepoPath: dir,
	}, nil)

	f := Finding{
		Type:     "tool_description",
		Target:   "nonexistent_tool",
		Proposed: "some improvement",
	}

	err := a.applyFinding(f)
	if err == nil {
		t.Error("expected error for nonexistent tool description file")
	}
}

func TestApply_WithGitBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create prompts/behaviors/ directory
	behaviorsDir := filepath.Join(dir, "prompts", "behaviors")
	if err := os.MkdirAll(behaviorsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
		DryRun:              false,
		RepoPath:            dir,
	}, nil)

	findings := []Finding{
		{
			Type:          "system_prompt",
			Priority:      "high",
			Target:        "retry_behavior",
			Issue:         "no backoff",
			Proposed:      "Add exponential backoff instruction",
			Rationale:     "Seen in 5 runs",
			Confidence:    ConfidenceCertain,
			EvidenceCount: 5,
		},
	}

	result, err := a.Apply(context.Background(), findings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.FindingsApplied != 1 {
		t.Errorf("FindingsApplied = %d, want 1", result.FindingsApplied)
	}
	if !strings.HasPrefix(result.BranchName, "training/auto-") {
		t.Errorf("BranchName = %q, want prefix training/auto-", result.BranchName)
	}
	if result.CommitHash == "" {
		t.Error("CommitHash should not be empty after real apply")
	}

	// Verify the branch exists in git
	cmd := exec.Command("git", "branch", "--list", result.BranchName)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), result.BranchName) {
		t.Errorf("branch %s not found in git", result.BranchName)
	}
}

func TestApply_WithStore(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Create prompts/behaviors/ directory
	behaviorsDir := filepath.Join(dir, "prompts", "behaviors")
	if err := os.MkdirAll(behaviorsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	a := NewApplier(ApplierConfig{
		ConfidenceThreshold: ConfidenceCertain,
		MinEvidenceCount:    3,
		DryRun:              false,
		RepoPath:            dir,
	}, store)

	findings := []Finding{
		{
			Type:          "system_prompt",
			Priority:      "high",
			Target:        "retry_behavior",
			Issue:         "no backoff",
			Proposed:      "Add exponential backoff instruction",
			Rationale:     "Seen in 5 runs",
			Confidence:    ConfidenceCertain,
			EvidenceCount: 5,
		},
	}

	result, err := a.Apply(context.Background(), findings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.FindingsApplied != 1 {
		t.Errorf("FindingsApplied = %d, want 1", result.FindingsApplied)
	}
}

func TestCanAutoApply_Defaults(t *testing.T) {
	// Test that defaults are applied when config values are zero
	a := NewApplier(ApplierConfig{}, nil)

	f := Finding{
		Type:          "system_prompt",
		Priority:      "high",
		Confidence:    ConfidenceCertain,
		EvidenceCount: 3,
	}
	ok, _ := a.canAutoApply(f)
	if !ok {
		t.Error("expected canAutoApply=true with defaults and 3 evidence")
	}
}
