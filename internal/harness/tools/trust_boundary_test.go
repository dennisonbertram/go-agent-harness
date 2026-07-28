package tools

// Tests for the two trust boundaries this package owns: workspace path
// confinement (the enforcement point for SandboxScopeWorkspace) and
// AskUserQuestion argument validation (LLM-supplied structured input).
//
// Both are written as attacks and malformed-input batteries rather than happy
// paths, because both fail in the direction that matters only on inputs a
// well-behaved caller never produces.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- workspace confinement --------------------------------------------

func TestConfineWorkspacePath_NonWorkspaceScopesArePassThrough(t *testing.T) {
	ws := t.TempDir()
	outside := filepath.Join(t.TempDir(), "elsewhere.txt")

	// Only SandboxScopeWorkspace confines. The other scopes are the explicit
	// opt-in a caller uses when it legitimately needs to reach outside, and
	// this function must not second-guess that.
	for _, scope := range []SandboxScope{SandboxScopeLocal, SandboxScopeUnrestricted, ""} {
		got, err := ConfineWorkspacePath(scope, ws, nil, outside)
		if err != nil {
			t.Errorf("scope %q should pass through, got error %v", scope, err)
		}
		if got != outside {
			t.Errorf("scope %q returned %q, want the input path unchanged", scope, got)
		}
	}
}

func TestConfineWorkspacePath_RejectsEscapes(t *testing.T) {
	base := t.TempDir()
	ws := filepath.Join(base, "ws")
	sibling := filepath.Join(base, "ws-evil")
	for _, d := range []string{ws, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(sibling, "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write sibling: %v", err)
	}

	attacks := []struct {
		name string
		path string
	}{
		{"absolute path outside the workspace", filepath.Join(base, "outside.txt")},
		{"sibling directory sharing a name prefix", filepath.Join(sibling, "secret.txt")},
		{"traversal above the workspace", filepath.Join(ws, "..", "outside.txt")},
		{"filesystem root", "/"},
	}
	for _, a := range attacks {
		t.Run(a.name, func(t *testing.T) {
			if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, nil, a.path); err == nil {
				t.Errorf("%s was not rejected", a.name)
			}
		})
	}
}

func TestConfineWorkspacePath_RejectsSymlinkEscapeIncludingForFilesNotYetCreated(t *testing.T) {
	ws := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(ws, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	// An existing file reached through the symlink.
	existing := filepath.Join(outside, "there.txt")
	if err := os.WriteFile(existing, []byte("x"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, nil, filepath.Join(link, "there.txt")); err == nil {
		t.Error("a symlink escape to an existing file was not rejected")
	}

	// A file that does not exist yet — the `write`-creates-a-new-file case.
	// This is the one a naive EvalSymlinks-only check misses, because the
	// final component cannot be resolved.
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, nil, filepath.Join(link, "not-created-yet.txt")); err == nil {
		t.Error("a symlink escape to a not-yet-created file was not rejected")
	}
}

func TestConfineWorkspacePath_PermitsWorkspaceAndExtraAllowedRoots(t *testing.T) {
	ws := t.TempDir()
	extra := t.TempDir()

	inside := filepath.Join(ws, "sub", "f.txt")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, nil, inside); err != nil {
		t.Errorf("a path inside the workspace must be permitted: %v", err)
	}
	// A brand-new file inside the workspace is permitted too.
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, nil, filepath.Join(ws, "brand-new.txt")); err != nil {
		t.Errorf("a not-yet-created file inside the workspace must be permitted: %v", err)
	}
	// The workspace root itself.
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, nil, ws); err != nil {
		t.Errorf("the workspace root itself must be permitted: %v", err)
	}

	extraFile := filepath.Join(extra, "f.txt")
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, []string{extra}, extraFile); err != nil {
		t.Errorf("an explicitly allowlisted extra root must be permitted: %v", err)
	}
	// A missing or misconfigured allowlist entry must not itself grant access.
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, ws, []string{filepath.Join(extra, "nope")}, extraFile); err == nil {
		t.Error("an unresolvable allowlist entry must not grant access")
	}
}

func TestConfineWorkspacePath_UnresolvableRootIsAnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-workspace")
	if _, err := ConfineWorkspacePath(SandboxScopeWorkspace, missing, nil, filepath.Join(missing, "f.txt")); err == nil {
		t.Error("a workspace root that does not exist must be an error, not a silent pass")
	}
}

func TestNormalizeRelPath(t *testing.T) {
	ws := t.TempDir()
	if got := NormalizeRelPath(ws, filepath.Join(ws, "a", "b.txt")); got != "a/b.txt" {
		t.Errorf("NormalizeRelPath = %q, want forward-slashed workspace-relative path", got)
	}
	if got := NormalizeRelPath(ws, ws); got != "." {
		t.Errorf("the workspace root should normalize to %q, got %q", ".", got)
	}
	// A path outside the workspace has no sensible relative form; the helper
	// returns something rather than failing, which callers rely on for display.
	if got := NormalizeRelPath(ws, "/completely/elsewhere"); got == "" {
		t.Error("NormalizeRelPath should return a displayable value for outside paths")
	}
}

func TestValidateWorkspaceRelativePattern(t *testing.T) {
	for _, bad := range []string{"/abs/pattern", "..", "../escape", "../../escape/**"} {
		if err := ValidateWorkspaceRelativePattern(bad); err == nil {
			t.Errorf("pattern %q should be rejected", bad)
		}
	}
	for _, ok := range []string{"src/**", "*.go", "./src/*.go", "a/../b"} {
		if err := ValidateWorkspaceRelativePattern(ok); err != nil {
			t.Errorf("pattern %q should be accepted, got %v", ok, err)
		}
	}
}

func TestIsDangerousCommand(t *testing.T) {
	for _, cmd := range []string{
		"rm -rf /",
		"RM -RF /",
		"shutdown now",
		"reboot",
		":(){ :|:& };:",
	} {
		if !IsDangerousCommand(cmd) {
			t.Errorf("command %q should be flagged dangerous", cmd)
		}
	}
	for _, cmd := range []string{
		"rm -rf ./build",
		"go test ./...",
		"rebooting the service", // no word boundary after "reboot"
		"",
	} {
		if IsDangerousCommand(cmd) {
			t.Errorf("command %q should NOT be flagged dangerous", cmd)
		}
	}
}

// TestIsDangerousCommand_MatchesAnywhereInTheCommand pins a KNOWN FALSE-POSITIVE
// class, deliberately.
//
// The patterns match their keyword anywhere in the string, not only in command
// position, and core.BashTool turns a match into a hard rejection
// ("command rejected by safety policy"). So an ordinary command that merely
// MENTIONS one of the words is refused. These are recorded rather than fixed
// because narrowing a safety control is a judgement call for the operator, not
// something to tighten silently: anchoring the patterns to command position
// would remove these false positives but would also stop matching genuinely
// dangerous invocations that are not the first word.
func TestIsDangerousCommand_MatchesAnywhereInTheCommand(t *testing.T) {
	falsePositives := []string{
		"echo shutdown",
		"cat reboot.txt",
		"grep shutdown /var/log/app.log",
		`git commit -m "fix the shutdown path"`,
	}
	for _, cmd := range falsePositives {
		if !IsDangerousCommand(cmd) {
			t.Errorf("command %q is currently flagged dangerous; if that changed, the "+
				"false-positive class described above was fixed — update this test and "+
				"the bash tool's docs", cmd)
		}
	}
}

// --- AskUserQuestion argument validation ------------------------------

func askQuestion(text string, labels ...string) AskUserQuestion {
	q := AskUserQuestion{Question: text, Header: "H"}
	for _, l := range labels {
		q.Options = append(q.Options, AskUserQuestionOption{Label: l, Description: "d-" + l})
	}
	return q
}

func TestValidateAskUserQuestions_RejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name      string
		questions []AskUserQuestion
		wantErr   string
	}{
		{"no questions", nil, "1-4 items"},
		{"too many questions", []AskUserQuestion{
			askQuestion("a", "x", "y"), askQuestion("b", "x", "y"),
			askQuestion("c", "x", "y"), askQuestion("d", "x", "y"),
			askQuestion("e", "x", "y"),
		}, "1-4 items"},
		{"blank question text", []AskUserQuestion{askQuestion("   ", "x", "y")}, "question is required"},
		{"duplicate question text", []AskUserQuestion{
			askQuestion("same", "x", "y"), askQuestion("same", "p", "q"),
		}, "unique question text"},
		{"missing header", []AskUserQuestion{{Question: "q", Options: []AskUserQuestionOption{
			{Label: "x", Description: "d"}, {Label: "y", Description: "d"}}}}, "header is required"},
		{"too few options", []AskUserQuestion{askQuestion("q", "only")}, "2-4 items"},
		{"too many options", []AskUserQuestion{askQuestion("q", "a", "b", "c", "d", "e")}, "2-4 items"},
		{"blank option label", []AskUserQuestion{askQuestion("q", "x", "  ")}, "label is required"},
		{"duplicate option labels", []AskUserQuestion{askQuestion("q", "dup", "dup")}, "labels must be unique"},
		{"missing option description", []AskUserQuestion{{Question: "q", Header: "H",
			Options: []AskUserQuestionOption{{Label: "x", Description: "d"}, {Label: "y"}}}}, "description is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAskUserQuestions(tc.questions)
			if err == nil {
				t.Fatalf("expected %s to be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q should mention %q", err.Error(), tc.wantErr)
			}
		})
	}

	if err := ValidateAskUserQuestions([]AskUserQuestion{askQuestion("q", "x", "y")}); err != nil {
		t.Errorf("a well-formed question should validate, got %v", err)
	}
}

func TestParseAskUserQuestionArgs(t *testing.T) {
	if _, err := ParseAskUserQuestionArgs(json.RawMessage(`not json`)); err == nil {
		t.Error("unparseable args must error")
	}
	// Structurally valid JSON that fails validation must surface the
	// validation error, not a parse error.
	_, err := ParseAskUserQuestionArgs(json.RawMessage(`{"questions":[]}`))
	if err == nil || !strings.Contains(err.Error(), "1-4 items") {
		t.Errorf("empty question list should fail validation, got %v", err)
	}

	qs, err := ParseAskUserQuestionArgs(json.RawMessage(
		`{"questions":[{"question":"Pick","header":"P","options":[{"label":"a","description":"A"},{"label":"b","description":"B"}]}]}`))
	if err != nil {
		t.Fatalf("valid args should parse: %v", err)
	}
	if len(qs) != 1 || qs[0].Question != "Pick" || len(qs[0].Options) != 2 {
		t.Errorf("parsed questions are wrong: %+v", qs)
	}
}

func TestNormalizeAskUserAnswers(t *testing.T) {
	single := []AskUserQuestion{askQuestion("Pick one", "a", "b")}
	multi := []AskUserQuestion{{
		Question: "Pick many", Header: "H", MultiSelect: true,
		Options: []AskUserQuestionOption{
			{Label: "a", Description: "A"}, {Label: "b", Description: "B"}, {Label: "c", Description: "C"},
		},
	}}

	t.Run("single select accepts a known label", func(t *testing.T) {
		got, err := NormalizeAskUserAnswers(single, map[string]string{"Pick one": " a "})
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if got["Pick one"] != "a" {
			t.Errorf("answer = %q, want the trimmed label", got["Pick one"])
		}
	})

	t.Run("single select rejects an unknown label", func(t *testing.T) {
		if _, err := NormalizeAskUserAnswers(single, map[string]string{"Pick one": "zzz"}); err == nil {
			t.Error("an answer outside the offered options must be rejected")
		}
	})

	t.Run("multi select dedupes and sorts", func(t *testing.T) {
		got, err := NormalizeAskUserAnswers(multi, map[string]string{"Pick many": "c, a ,c,b"})
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if got["Pick many"] != "a,b,c" {
			t.Errorf("multi-select answer = %q, want deduped and sorted \"a,b,c\"", got["Pick many"])
		}
	})

	t.Run("multi select rejects an unknown label among valid ones", func(t *testing.T) {
		if _, err := NormalizeAskUserAnswers(multi, map[string]string{"Pick many": "a,nope"}); err == nil {
			t.Error("an unknown label anywhere in a multi-select answer must be rejected")
		}
	})

	t.Run("multi select rejects an all-empty answer", func(t *testing.T) {
		if _, err := NormalizeAskUserAnswers(multi, map[string]string{"Pick many": " , , "}); err == nil {
			t.Error("an answer that resolves to no labels must be rejected")
		}
	})

	t.Run("rejects an unexpected question", func(t *testing.T) {
		if _, err := NormalizeAskUserAnswers(single, map[string]string{"Not asked": "a"}); err == nil {
			t.Error("an answer to a question that was not asked must be rejected")
		}
	})

	t.Run("rejects a blank answer", func(t *testing.T) {
		if _, err := NormalizeAskUserAnswers(single, map[string]string{"Pick one": "   "}); err == nil {
			t.Error("a blank answer must be rejected")
		}
	})

	t.Run("rejects a wrong answer count", func(t *testing.T) {
		if _, err := NormalizeAskUserAnswers(single, map[string]string{}); err == nil {
			t.Error("a missing answer must be rejected")
		}
	})

	t.Run("propagates question validation failures", func(t *testing.T) {
		if _, err := NormalizeAskUserAnswers(nil, map[string]string{}); err == nil {
			t.Error("invalid questions must be rejected before answers are considered")
		}
	})
}

func TestAskUserQuestionTimeoutError(t *testing.T) {
	var nilErr *AskUserQuestionTimeoutError
	if nilErr.Error() == "" {
		t.Error("a nil timeout error must still render a message")
	}
	zero := &AskUserQuestionTimeoutError{}
	if !strings.Contains(zero.Error(), "timed out") {
		t.Errorf("zero-deadline message = %q, want it to mention the timeout", zero.Error())
	}
	if !IsAskUserQuestionTimeout(zero) {
		t.Error("IsAskUserQuestionTimeout should recognize its own type")
	}
	if IsAskUserQuestionTimeout(nil) {
		t.Error("IsAskUserQuestionTimeout(nil) must be false")
	}
}
