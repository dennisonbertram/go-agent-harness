package harness

import (
	"encoding/json"
	"testing"

	htools "go-agent-harness/internal/harness/tools"
)

// TestPermissionRules_ApplyPatchUnifiedDiffPathsAreVisible pins the fix for the
// deny-rule bypass: apply_patch in unified-diff mode carries its target paths
// inside the `patch` body and has no top-level path argument, so matching only
// the primary argument left such calls matching NO rule — which
// EvaluatePermissionRules resolves as allow.
func TestPermissionRules_ApplyPatchUnifiedDiffPathsAreVisible(t *testing.T) {
	rules := []PermissionRule{{Pattern: "apply_patch(secrets/**)", Effect: PermissionEffectDeny}}
	args := json.RawMessage(`{"patch":"*** Update File: secrets/keys.txt\n@@\n-a\n+b\n"}`)

	effect, err := EvaluatePermissionRules(rules, "apply_patch", args, t.TempDir())
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if effect != PermissionEffectDeny {
		t.Errorf("patch-mode apply_patch against a deny rule = %q, want deny", effect)
	}
}

// TestPermissionRules_AllowRequiresEveryTargetedPath ensures an allow rule for
// one file cannot be widened by bundling that file into a patch alongside
// others: allow matches only when EVERY targeted path matches.
func TestPermissionRules_AllowRequiresEveryTargetedPath(t *testing.T) {
	rules := []PermissionRule{
		{Pattern: "apply_patch(**)", Effect: PermissionEffectDeny},
		{Pattern: "apply_patch(notes.md)", Effect: PermissionEffectAllow},
	}
	workspace := t.TempDir()

	onlyAllowed := json.RawMessage(`{"patch":"*** Update File: notes.md\n@@\n-a\n+b\n"}`)
	if effect, err := EvaluatePermissionRules(rules, "apply_patch", onlyAllowed, workspace); err != nil || effect != PermissionEffectAllow {
		t.Errorf("patch touching only the allowed file = %q (err %v), want allow", effect, err)
	}

	bundled := json.RawMessage(`{"patch":"*** Update File: notes.md\n@@\n-a\n+b\n*** Update File: secrets/keys.txt\n@@\n-c\n+d\n"}`)
	if effect, err := EvaluatePermissionRules(rules, "apply_patch", bundled, workspace); err != nil || effect != PermissionEffectDeny {
		t.Errorf("patch bundling an unallowed file = %q (err %v), want deny", effect, err)
	}
}

// TestPermissionRules_NonASCIIPatternsMatch pins the byte-vs-rune fix in
// permissionGlobRegexp. Iterating bytes re-encoded each byte of a multi-byte
// rune as its own code point, so a non-ASCII deny rule could never match and
// silently failed open.
func TestPermissionRules_NonASCIIPatternsMatch(t *testing.T) {
	workspace := t.TempDir()

	bashRules := []PermissionRule{{Pattern: "bash(echo café*)", Effect: PermissionEffectDeny}}
	bashArgs := json.RawMessage(`{"command":"echo café hi"}`)
	if effect, err := EvaluatePermissionRules(bashRules, "bash", bashArgs, workspace); err != nil || effect != PermissionEffectDeny {
		t.Errorf("non-ASCII bash deny rule = %q (err %v), want deny", effect, err)
	}

	pathRules := []PermissionRule{{Pattern: "read(документы/**)", Effect: PermissionEffectDeny}}
	pathArgs := json.RawMessage(`{"path":"документы/секрет.txt"}`)
	if effect, err := EvaluatePermissionRules(pathRules, "read", pathArgs, workspace); err != nil || effect != PermissionEffectDeny {
		t.Errorf("non-ASCII path deny rule = %q (err %v), want deny", effect, err)
	}
}

// TestPlanModeGate_DeniesUndeterminablePaths is the enforcement-side companion:
// the plan-mode gate must fail closed rather than inherit
// EvaluatePermissionRules' allow-by-default when no rule matches.
func TestPlanModeGate_DeniesUndeterminablePaths(t *testing.T) {
	workspace := t.TempDir()
	runner := &Runner{
		runs:             map[string]*runState{},
		skillConstraints: NewSkillConstraintTracker(),
	}
	runner.runs["run-1"] = &runState{
		planMode:                PlanModeActive,
		planFile:                defaultPlanFile,
		permissionWorkspaceRoot: workspace,
	}
	gate := runPlanModeGate{runner: runner, runID: "run-1"}

	cases := []struct {
		name string
		tool string
		args string
		want bool
	}{
		{"plan file write is allowed", "write", `{"path":".harness/plan.md","content":"x"}`, true},
		{"other file write is denied", "write", `{"path":"src/main.go","content":"x"}`, false},
		{"patch-mode apply_patch to another file is denied", "apply_patch", `{"patch":"*** Update File: src/main.go\n@@\n-a\n+b\n"}`, false},
		{"patch-mode apply_patch bundling the plan file is denied", "apply_patch", `{"patch":"*** Update File: .harness/plan.md\n@@\n-a\n+b\n*** Update File: src/main.go\n@@\n-c\n+d\n"}`, false},
		{"args with no determinable path are denied", "apply_patch", `{}`, false},
		{"non-path mutating tool is denied", "bash", `{"command":"echo hi"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gate.AllowMutation(htools.Definition{Name: tc.tool, Mutating: true}, json.RawMessage(tc.args))
			if got != tc.want {
				t.Errorf("AllowMutation(%s, %s) = %v, want %v", tc.tool, tc.args, got, tc.want)
			}
		})
	}
}

// TestNormalizedPlanFile_RejectsUnmatchablePaths ensures a plan file the
// permission matcher cannot express falls back to the default instead of
// making the fail-closed gate deny every write for the whole run.
func TestNormalizedPlanFile_RejectsUnmatchablePaths(t *testing.T) {
	for _, in := range []string{"", "   ", "/etc/plan.md", "../outside/plan.md"} {
		if got := normalizedPlanFile(in); got != defaultPlanFile {
			t.Errorf("normalizedPlanFile(%q) = %q, want %q", in, got, defaultPlanFile)
		}
	}
	if got := normalizedPlanFile("docs/my-plan.md"); got != "docs/my-plan.md" {
		t.Errorf("normalizedPlanFile kept a valid relative path as %q", got)
	}
}

// TestPermissionGlobRegexp covers the glob-to-regexp compiler directly,
// including the character-class scan rewritten for the rune fix. Coverage
// analysis showed the `**`, `?`, and `[...]` branches were never exercised —
// a gap that matters because an unmatched pattern makes a deny rule fail open.
func TestPermissionGlobRegexp(t *testing.T) {
	cases := []struct {
		name        string
		pattern     string
		pathPattern bool
		matches     []string
		rejects     []string
	}{
		{
			name: "path doublestar spans separators", pattern: "src/**", pathPattern: true,
			matches: []string{"src/a.go", "src/nested/deep/b.go"},
			rejects: []string{"other/a.go"},
		},
		{
			name: "path single star stops at separator", pattern: "src/*", pathPattern: true,
			matches: []string{"src/a.go"},
			rejects: []string{"src/nested/b.go"},
		},
		{
			name: "path question mark matches one non-separator", pattern: "a?.go", pathPattern: true,
			matches: []string{"ab.go"},
			rejects: []string{"a.go", "abc.go", "a/.go"},
		},
		{
			name: "command question mark matches any single char", pattern: "ls ?", pathPattern: false,
			matches: []string{"ls a", "ls /"},
			rejects: []string{"ls ab"},
		},
		{
			name: "character class", pattern: "file[abc].go", pathPattern: true,
			matches: []string{"filea.go", "filec.go"},
			rejects: []string{"filed.go", "fileab.go"},
		},
		{
			// A class containing regex metacharacters is escaped WHOLE rather
			// than emitted as a character class, so it matches the literal
			// bracket text. That is deliberate: it keeps attacker-influenced
			// metacharacters out of the compiled expression.
			name: "character class with regex metacharacters is escaped literally", pattern: "f[.*].go", pathPattern: true,
			matches: []string{"f[.*].go"},
			rejects: []string{"f..go", "f*.go", "fx.go"},
		},
		{
			name: "non-ascii inside a character class", pattern: "f[éx].go", pathPattern: true,
			matches: []string{"fé.go", "fx.go"},
			rejects: []string{"fy.go"},
		},
		{
			name: "literal dot is not a wildcard", pattern: "a.go", pathPattern: true,
			matches: []string{"a.go"},
			rejects: []string{"axgo"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, in := range tc.matches {
				if !permissionGlobMatch(in, tc.pattern, tc.pathPattern) {
					t.Errorf("pattern %q should match %q", tc.pattern, in)
				}
			}
			for _, in := range tc.rejects {
				if permissionGlobMatch(in, tc.pattern, tc.pathPattern) {
					t.Errorf("pattern %q should NOT match %q", tc.pattern, in)
				}
			}
		})
	}

	// An unterminated character class compiles to nothing and must therefore
	// match nothing, rather than producing a regexp that matches everything.
	if got := permissionGlobRegexp("file[abc.go", true); got != "" {
		t.Errorf("unterminated character class produced %q, want empty", got)
	}
	if permissionGlobMatch("file[abc.go", "file[abc.go", true) {
		t.Error("an unterminated character class must not match")
	}
}
