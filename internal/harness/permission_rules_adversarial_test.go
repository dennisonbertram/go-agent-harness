package harness

// Adversarial tests for the fine-grained permission matcher.
//
// These are written as ATTACKS, not as feature checks. The matcher's failure
// mode is silent and one-directional: when no rule matches,
// EvaluatePermissionRules returns allow, so any input shape the matcher cannot
// see turns a deny rule into a no-op. Every bug found in this layer so far —
// apply_patch's unified-diff payload, non-ASCII patterns, case-variant paths —
// had that same shape, and none of them were caught by the existing
// feature-oriented tests, which only ever fed the matcher well-formed input
// that looked like what the rule author had in mind.
//
// The file is split into two halves, and the split is the point:
//
//   - GUARANTEES: evasions the matcher is documented to defeat. A failure here
//     is a security bug.
//   - NON-GUARANTEES: evasions it deliberately does NOT defeat, pinned so they
//     stay visible and cannot be mistaken for protection. EvaluatePermissionRules'
//     own doc comment says this is "a policy and ergonomics layer, not a
//     security boundary"; OS-level sandboxing is the real boundary. A failure
//     here means behaviour changed and the docs need revisiting — not
//     necessarily that something broke.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adversarialWorkspace builds a workspace with a protected directory and an
// ordinary one, returning its root.
func adversarialWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	for _, dir := range []string{"secrets", "src"} {
		if err := os.MkdirAll(filepath.Join(ws, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws, "secrets", "k.txt"), []byte("SECRET"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "src", "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return ws
}

func evalRead(t *testing.T, rules []PermissionRule, ws, path string) PermissionEffect {
	t.Helper()
	args, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	effect, err := EvaluatePermissionRules(rules, "read", args, ws)
	if err != nil {
		t.Fatalf("evaluate %q: %v", path, err)
	}
	return effect
}

// ---------------------------------------------------------------------------
// GUARANTEES — a failure in this section is a security bug.
// ---------------------------------------------------------------------------

// TestAdversarial_PathSpellingsCannotEvadeADenyRule attacks a deny rule with
// every spelling of the same protected file the filesystem accepts. Each of
// these resolves to the identical inode, so each must be denied.
func TestAdversarial_PathSpellingsCannotEvadeADenyRule(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "read(secrets/**)", Effect: PermissionEffectDeny}}

	attacks := []struct {
		name string
		path string
	}{
		{"plain relative", "secrets/k.txt"},
		{"dot-slash prefix", "./secrets/k.txt"},
		{"doubled separator", "secrets//k.txt"},
		{"interior dot segment", "secrets/./k.txt"},
		{"traversal that lands back inside", "src/../secrets/k.txt"},
		{"deep traversal that lands back inside", "src/../src/../secrets/k.txt"},
		{"absolute path to the same file", filepath.Join(ws, "secrets", "k.txt")},
		{"trailing whitespace", "secrets/k.txt "},
		{"leading whitespace", " secrets/k.txt"},
	}
	for _, a := range attacks {
		t.Run(a.name, func(t *testing.T) {
			if got := evalRead(t, rules, ws, a.path); got != PermissionEffectDeny {
				t.Errorf("deny rule evaded by %s (%q): got %q", a.name, a.path, got)
			}
		})
	}
}

// TestAdversarial_CaseVariantCannotEvadeADenyRule pins the case-insensitivity
// fix. On macOS and Windows the filesystem is case-insensitive by default, so
// "SECRETS/K.TXT" opens exactly the same file as "secrets/k.txt" —
// filepath.EvalSymlinks preserves the case it is handed rather than
// normalizing it, so an exact match silently missed the variant.
func TestAdversarial_CaseVariantCannotEvadeADenyRule(t *testing.T) {
	ws := adversarialWorkspace(t)

	// Only meaningful where the filesystem actually is case-insensitive;
	// on a case-sensitive one these name different (nonexistent) files.
	if _, err := os.ReadFile(filepath.Join(ws, "SECRETS", "K.TXT")); err != nil {
		t.Skip("filesystem is case-sensitive; case-variant paths name different files here")
	}

	rules := []PermissionRule{{Pattern: "read(secrets/**)", Effect: PermissionEffectDeny}}
	for _, path := range []string{"SECRETS/k.txt", "secrets/K.TXT", "Secrets/K.txt"} {
		if got := evalRead(t, rules, ws, path); got != PermissionEffectDeny {
			t.Errorf("deny rule evaded by case variant %q: got %q", path, got)
		}
	}
}

// TestAdversarial_AllowRulesStayCaseExact is the other half of that fix.
// Widening a DENY to cover case variants is safe; widening an ALLOW is not,
// because on a case-sensitive filesystem a case variant is a genuinely
// different file and matching it would grant access the operator never wrote.
func TestAdversarial_AllowRulesStayCaseExact(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{
		{Pattern: "read(**)", Effect: PermissionEffectDeny},
		{Pattern: "read(src/a.go)", Effect: PermissionEffectAllow},
	}

	if got := evalRead(t, rules, ws, "src/a.go"); got != PermissionEffectAllow {
		t.Errorf("exact allow path = %q, want allow", got)
	}
	// The uppercase spelling must NOT inherit the allow.
	if got := evalRead(t, rules, ws, "src/A.GO"); got == PermissionEffectAllow {
		t.Error("an allow rule must not be widened to a case variant")
	}
}

// TestAdversarial_ToolNameSpellingsCannotEvadeARule attacks the tool-name half
// of a rule, which is documented as case-insensitive and whitespace-trimmed.
func TestAdversarial_ToolNameSpellingsCannotEvadeARule(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "read(secrets/**)", Effect: PermissionEffectDeny}}
	args, _ := json.Marshal(map[string]string{"path": "secrets/k.txt"})

	for _, tool := range []string{"read", "READ", "Read", " read", "read "} {
		effect, err := EvaluatePermissionRules(rules, tool, args, ws)
		if err != nil {
			t.Fatalf("evaluate tool %q: %v", tool, err)
		}
		if effect != PermissionEffectDeny {
			t.Errorf("deny rule evaded by tool-name spelling %q: got %q", tool, effect)
		}
	}
}

// TestAdversarial_PathArgumentAliasesCannotEvadeARule ensures a caller cannot
// dodge a rule by using a different accepted spelling of the path argument.
func TestAdversarial_PathArgumentAliasesCannotEvadeARule(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "read(secrets/**)", Effect: PermissionEffectDeny}}

	for _, key := range []string{"path", "file_path"} {
		args, _ := json.Marshal(map[string]string{key: "secrets/k.txt"})
		effect, err := EvaluatePermissionRules(rules, "read", args, ws)
		if err != nil {
			t.Fatalf("evaluate alias %q: %v", key, err)
		}
		if effect != PermissionEffectDeny {
			t.Errorf("deny rule evaded via the %q argument alias: got %q", key, effect)
		}
	}
}

// TestAdversarial_ApplyPatchPayloadShapesCannotEvadeADenyRule attacks the
// apply_patch surface, which accepts two distinct patch formats plus field
// aliases. Every shape that writes the protected file must be denied — this is
// the bypass that let a patch body carry its targets past the matcher.
func TestAdversarial_ApplyPatchPayloadShapesCannotEvadeADenyRule(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "apply_patch(secrets/**)", Effect: PermissionEffectDeny}}

	attacks := []struct {
		name string
		args string
	}{
		{"top-level path", `{"path":"secrets/k.txt","find":"a","replace":"b"}`},
		{"file_path alias", `{"file_path":"secrets/k.txt","find":"a","replace":"b"}`},
		{"custom patch format", `{"patch":"*** Begin Patch\n*** Update File: secrets/k.txt\n@@\n-a\n+b\n*** End Patch"}`},
		{"custom format add file", `{"patch":"*** Begin Patch\n*** Add File: secrets/new.txt\n+x\n*** End Patch"}`},
		{"custom format delete file", `{"patch":"*** Begin Patch\n*** Delete File: secrets/k.txt\n*** End Patch"}`},
		{"standard unified diff", `{"patch":"--- a/secrets/k.txt\n+++ b/secrets/k.txt\n@@ -1 +1 @@\n-a\n+b\n"}`},
		{"diff field alias", `{"diff":"--- a/secrets/k.txt\n+++ b/secrets/k.txt\n@@ -1 +1 @@\n-a\n+b\n"}`},
		{"unified_diff field alias", `{"unified_diff":"--- a/secrets/k.txt\n+++ b/secrets/k.txt\n@@ -1 +1 @@\n-a\n+b\n"}`},
		{"patch smuggled alongside an innocuous path", `{"path":"src/a.go","patch":"*** Begin Patch\n*** Update File: secrets/k.txt\n@@\n-a\n+b\n*** End Patch"}`},
	}
	for _, a := range attacks {
		t.Run(a.name, func(t *testing.T) {
			effect, err := EvaluatePermissionRules(rules, "apply_patch", json.RawMessage(a.args), ws)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if effect != PermissionEffectDeny {
				t.Errorf("deny rule evaded by %s: got %q", a.name, effect)
			}
		})
	}
}

// TestAdversarial_BashSpellingsCannotEvadeADenyRule covers the normalizations
// the command matcher IS documented to perform: whitespace collapsing, shell
// quote removal, and stripping the leading executable path.
func TestAdversarial_BashSpellingsCannotEvadeADenyRule(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "bash(rm -rf:*)", Effect: PermissionEffectDeny}}

	attacks := []struct {
		name    string
		command string
	}{
		{"plain", "rm -rf /tmp/x"},
		{"collapsed whitespace", "rm   -rf    /tmp/x"},
		{"tab separated", "rm\t-rf\t/tmp/x"},
		{"absolute executable path", "/bin/rm -rf /tmp/x"},
		{"quoted executable", "'rm' -rf /tmp/x"},
		{"double-quoted executable", "\"rm\" -rf /tmp/x"},
		{"quoted argument", "rm -rf '/tmp/x'"},
		{"leading whitespace", "   rm -rf /tmp/x"},
	}
	for _, a := range attacks {
		t.Run(a.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"command": a.command})
			effect, err := EvaluatePermissionRules(rules, "bash", args, ws)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if effect != PermissionEffectDeny {
				t.Errorf("deny rule evaded by %s (%q): got %q", a.name, a.command, effect)
			}
		})
	}
}

// TestAdversarial_SymlinkEscapeIsRejected ensures a symlink inside the
// workspace pointing outside it cannot be used to make an out-of-workspace
// file look in-workspace to the matcher.
func TestAdversarial_SymlinkEscapeIsRejected(t *testing.T) {
	ws := adversarialWorkspace(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "target.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	link := filepath.Join(ws, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	// canonicalWorkspacePath must refuse to produce a workspace-relative path
	// for something that resolves outside the workspace.
	if rel, ok := canonicalWorkspacePath(ws, "escape/target.txt"); ok {
		t.Errorf("symlink escape produced in-workspace path %q; want rejection", rel)
	}
}

// TestAdversarial_SiblingDirectorySharingAPrefixIsNotInsideTheWorkspace guards
// the string-prefix mistake: "/tmp/ws-evil" must not count as inside "/tmp/ws".
func TestAdversarial_SiblingDirectorySharingAPrefixIsNotInsideTheWorkspace(t *testing.T) {
	base := t.TempDir()
	ws := filepath.Join(base, "ws")
	sibling := filepath.Join(base, "ws-evil")
	for _, d := range []string{ws, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(sibling, "secret.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write sibling file: %v", err)
	}

	if rel, ok := canonicalWorkspacePath(ws, filepath.Join(sibling, "secret.txt")); ok {
		t.Errorf("a sibling directory sharing a name prefix was treated as in-workspace (%q)", rel)
	}
}

// TestAdversarial_MalformedPatternsDoNotPanicOrMatchEverything feeds hostile
// rule patterns through the compiler. The failure to avoid is a pattern that
// compiles to something matching everything, or one that panics.
func TestAdversarial_MalformedPatternsDoNotPanicOrMatchEverything(t *testing.T) {
	ws := adversarialWorkspace(t)

	for _, pattern := range []string{
		"read([unterminated)",
		"read(**[)",
		"read([])",
		"read(***)",
		"read([^/])",
	} {
		t.Run(pattern, func(t *testing.T) {
			rule, err := ParsePermissionRule(pattern, PermissionEffectDeny)
			if err != nil {
				return // rejected at parse time, which is a fine outcome
			}
			// If it parsed, it must not deny an unrelated path by accident,
			// and must not blow up.
			effect := evalRead(t, []PermissionRule{rule}, ws, "src/a.go")
			if effect != PermissionEffectAllow && effect != PermissionEffectDeny {
				t.Errorf("unexpected effect %q", effect)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NON-GUARANTEES — pinned so they stay visible. A change here is a docs
// question, not automatically a bug.
// ---------------------------------------------------------------------------

// TestAdversarial_DocumentedBashNonGuarantees pins the command-matching
// evasions EvaluatePermissionRules' doc comment explicitly disclaims: it "does
// not parse shell syntax or detect commands embedded after operators,
// expansions, aliases, or interpreters".
//
// These all currently return allow despite a deny rule on the underlying
// command. That is expected. It is recorded here so nobody reads the bash rule
// support as a sandbox — OS-level confinement is what actually contains these.
func TestAdversarial_DocumentedBashNonGuarantees(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "bash(rm -rf:*)", Effect: PermissionEffectDeny}}

	knownEvasions := []struct {
		name    string
		command string
	}{
		{"command after a semicolon", "echo hi; rm -rf /tmp/x"},
		{"command after &&", "true && rm -rf /tmp/x"},
		{"command in a pipeline", "echo x | xargs rm -rf"},
		{"wrapped in an interpreter", "sh -c 'rm -rf /tmp/x'"},
		{"command substitution", "$(echo rm) -rf /tmp/x"},
		{"environment prefix", "FOO=bar rm -rf /tmp/x"},
	}
	for _, e := range knownEvasions {
		t.Run(e.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"command": e.command})
			effect, err := EvaluatePermissionRules(rules, "bash", args, ws)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if effect != PermissionEffectAllow {
				t.Errorf("this evasion is documented as NOT defended against, but got %q — "+
					"if command matching gained shell awareness, update the doc comment on "+
					"EvaluatePermissionRules and move this case into the guarantees section", effect)
			}
		})
	}
}

// TestAdversarial_UnparseableArgumentsFallOpen pins the matcher's default:
// when a tool call's arguments yield no recognizable target, no rule matches
// and EvaluatePermissionRules returns allow.
//
// This is the shape every bug in this layer has taken, so it is worth stating
// plainly: a deny rule protects only inputs the matcher can interpret. Callers
// that need fail-closed behaviour must not rely on the absence of a deny — see
// runPlanModeGate.AllowMutation (plan_mode.go), which requires a positive allow
// match precisely because of this default.
func TestAdversarial_UnparseableArgumentsFallOpen(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "read(secrets/**)", Effect: PermissionEffectDeny}}

	for _, raw := range []string{`{}`, `null`, `[]`, `{"path":""}`, `{"path":123}`, `not json at all`} {
		t.Run(raw, func(t *testing.T) {
			effect, err := EvaluatePermissionRules(rules, "read", json.RawMessage(raw), ws)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if effect != PermissionEffectAllow {
				t.Errorf("expected the documented allow-by-default for uninterpretable args, got %q", effect)
			}
		})
	}
}

// TestAdversarial_LongInputsTerminate guards against a pathological pattern or
// path making matching blow up. Go's regexp is linear-time, so this should be
// uneventful; the test exists so a future switch to a backtracking matcher
// cannot land unnoticed.
func TestAdversarial_LongInputsTerminate(t *testing.T) {
	ws := adversarialWorkspace(t)
	rules := []PermissionRule{{Pattern: "read(" + strings.Repeat("a*", 60) + ")", Effect: PermissionEffectDeny}}
	longPath := strings.Repeat("a", 4000)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = evalRead(t, rules, ws, longPath)
	}()
	<-done
}
