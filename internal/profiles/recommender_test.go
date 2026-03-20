package profiles

import (
	"strings"
	"testing"
)

// TestRecommendProfile_ReviewTask verifies that review-related tasks recommend the reviewer profile.
func TestRecommendProfile_ReviewTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("review the code for issues")
	if rec.ProfileName != "reviewer" {
		t.Errorf("expected profile 'reviewer', got %q", rec.ProfileName)
	}
	if rec.Confidence == "" {
		t.Error("expected non-empty confidence")
	}
	if rec.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

// TestRecommendProfile_AuditTask verifies that audit tasks are routed to reviewer.
func TestRecommendProfile_AuditTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("audit the security of this module")
	if rec.ProfileName != "reviewer" {
		t.Errorf("expected profile 'reviewer', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_ResearchTask verifies that research tasks recommend the researcher profile.
func TestRecommendProfile_ResearchTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("research the API documentation")
	if rec.ProfileName != "researcher" {
		t.Errorf("expected profile 'researcher', got %q", rec.ProfileName)
	}
	if rec.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

// TestRecommendProfile_SearchTask verifies that search tasks are routed to researcher.
func TestRecommendProfile_SearchTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("find all usages of this function")
	if rec.ProfileName != "researcher" {
		t.Errorf("expected profile 'researcher', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_BashTask verifies that bash/shell tasks recommend the bash-runner profile.
func TestRecommendProfile_BashTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("run the bash script to deploy")
	if rec.ProfileName != "bash-runner" {
		t.Errorf("expected profile 'bash-runner', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_ShellTask verifies that shell script tasks are routed to bash-runner.
func TestRecommendProfile_ShellTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("execute the shell script")
	if rec.ProfileName != "bash-runner" {
		t.Errorf("expected profile 'bash-runner', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_WriteFileTask verifies that file write tasks recommend the file-writer profile.
func TestRecommendProfile_WriteFileTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("write file with new content")
	if rec.ProfileName != "file-writer" {
		t.Errorf("expected profile 'file-writer', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_CreateFileTask verifies that create file tasks are routed to file-writer.
func TestRecommendProfile_CreateFileTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("create file config.json")
	if rec.ProfileName != "file-writer" {
		t.Errorf("expected profile 'file-writer', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_EditFileTask verifies that edit file tasks are routed to file-writer.
func TestRecommendProfile_EditFileTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("edit file main.go to fix the bug")
	if rec.ProfileName != "file-writer" {
		t.Errorf("expected profile 'file-writer', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_GithubTask verifies that github-related tasks recommend the github profile.
func TestRecommendProfile_GithubTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("create a github issue for this bug")
	if rec.ProfileName != "github" {
		t.Errorf("expected profile 'github', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_PullRequestTask verifies that PR tasks are routed to github.
func TestRecommendProfile_PullRequestTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("open a pull request with these changes")
	if rec.ProfileName != "github" {
		t.Errorf("expected profile 'github', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_PRShorthand verifies that "pr" shorthand is routed to github.
func TestRecommendProfile_PRShorthand(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("merge the pr after approval")
	if rec.ProfileName != "github" {
		t.Errorf("expected profile 'github', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_DefaultFallback verifies that unmatched tasks fall back to the full profile.
func TestRecommendProfile_DefaultFallback(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("do some work")
	if rec.ProfileName != "full" {
		t.Errorf("expected fallback profile 'full', got %q", rec.ProfileName)
	}
	if rec.Reason == "" {
		t.Error("expected non-empty reason even for fallback")
	}
}

// TestRecommendProfile_EmptyTask verifies that an empty task falls back to the full profile.
func TestRecommendProfile_EmptyTask(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("")
	if rec.ProfileName != "full" {
		t.Errorf("expected fallback profile 'full' for empty task, got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_CaseInsensitive verifies that matching is case-insensitive.
func TestRecommendProfile_CaseInsensitive(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("REVIEW this code carefully")
	if rec.ProfileName != "reviewer" {
		t.Errorf("expected 'reviewer' for uppercase REVIEW, got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_ReasonNonEmpty verifies that every recommendation includes a non-empty reason.
func TestRecommendProfile_ReasonNonEmpty(t *testing.T) {
	t.Parallel()

	tasks := []string{
		"review the code",
		"research the docs",
		"run a shell script",
		"write file output.txt",
		"create a github PR",
		"do something generic",
	}
	for _, task := range tasks {
		rec := RecommendProfile(task)
		if rec.Reason == "" {
			t.Errorf("task %q: expected non-empty reason, got empty", task)
		}
		if rec.ProfileName == "" {
			t.Errorf("task %q: expected non-empty profile name, got empty", task)
		}
	}
}

// TestRecommendProfile_ConfidenceValues verifies that confidence is always a valid value.
func TestRecommendProfile_ConfidenceValues(t *testing.T) {
	t.Parallel()

	validConfidence := map[string]bool{"high": true, "medium": true, "low": true}
	tasks := []string{
		"review the code",
		"research the API",
		"run the bash script",
		"write file config.go",
		"open a github issue",
		"do generic work",
	}
	for _, task := range tasks {
		rec := RecommendProfile(task)
		if !validConfidence[rec.Confidence] {
			t.Errorf("task %q: expected confidence to be 'high', 'medium', or 'low', got %q", task, rec.Confidence)
		}
	}
}

// TestRecommendProfile_ProfileNameIsBuiltIn verifies that all recommended profiles actually exist as built-ins.
func TestRecommendProfile_ProfileNameIsBuiltIn(t *testing.T) {
	t.Parallel()

	tasks := []string{
		"review the code",
		"research the API",
		"run the bash script",
		"write file config.go",
		"open a github issue",
		"do generic work",
	}
	builtins, err := listBuiltinNames()
	if err != nil {
		t.Fatalf("failed to list built-in profiles: %v", err)
	}
	builtinSet := make(map[string]bool)
	for _, name := range builtins {
		builtinSet[name] = true
	}

	for _, task := range tasks {
		rec := RecommendProfile(task)
		if !builtinSet[rec.ProfileName] {
			t.Errorf("task %q: recommended profile %q is not a built-in profile (available: %v)",
				task, rec.ProfileName, builtins)
		}
	}
}

// TestRecommendProfile_IssueKeyword verifies that "issue" routes to github profile.
func TestRecommendProfile_IssueKeyword(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("create an issue for the failing test")
	if rec.ProfileName != "github" {
		t.Errorf("expected 'github', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_AnalyzeKeyword verifies that "analyze" routes to reviewer.
func TestRecommendProfile_AnalyzeKeyword(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("analyze the performance of this function")
	if rec.ProfileName != "reviewer" {
		t.Errorf("expected 'reviewer', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_CheckKeyword verifies that "check" routes to reviewer.
func TestRecommendProfile_CheckKeyword(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("check the output of the last test run")
	if rec.ProfileName != "reviewer" {
		t.Errorf("expected 'reviewer', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_InvestigateKeyword verifies that "investigate" routes to researcher.
func TestRecommendProfile_InvestigateKeyword(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("investigate why the build is failing")
	if rec.ProfileName != "researcher" {
		t.Errorf("expected 'researcher', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_ScriptKeyword verifies that "script" routes to bash-runner.
func TestRecommendProfile_ScriptKeyword(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("execute the migration script")
	if rec.ProfileName != "bash-runner" {
		t.Errorf("expected 'bash-runner', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_RunCommandKeyword verifies that "run command" routes to bash-runner.
func TestRecommendProfile_RunCommandKeyword(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("run command make build")
	if rec.ProfileName != "bash-runner" {
		t.Errorf("expected 'bash-runner', got %q", rec.ProfileName)
	}
}

// TestRecommendProfile_ReasonContainsMatchedKeyword verifies that the reason explains which keyword triggered the match.
func TestRecommendProfile_ReasonContainsMatchedKeyword(t *testing.T) {
	t.Parallel()

	rec := RecommendProfile("review the pull request changes")
	lower := strings.ToLower(rec.Reason)
	if !strings.Contains(lower, "review") && !strings.Contains(lower, "reviewer") {
		t.Errorf("expected reason to reference the matched keyword 'review', got %q", rec.Reason)
	}
}
