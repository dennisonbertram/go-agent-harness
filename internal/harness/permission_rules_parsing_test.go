package harness

// Parsing and validation tests for permission rules.
//
// Rule patterns come from operator configuration, so a malformed one should be
// rejected loudly at parse time rather than silently compiling into a rule that
// matches nothing — a rule that matches nothing is indistinguishable from no
// rule at all, which is exactly how a deny fails open.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParsePermissionRule_RejectsMalformedPatterns(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		effect  PermissionEffect
		wantErr string
	}{
		{"empty pattern", "", PermissionEffectDeny, "required"},
		{"whitespace-only pattern", "   ", PermissionEffectDeny, "required"},
		{"unknown effect", "read", PermissionEffect("maybe"), "invalid permission rule effect"},
		{"empty effect", "read", PermissionEffect(""), "invalid permission rule effect"},
		{"open paren with no close", "read(src/**", PermissionEffectDeny, "invalid permission rule pattern"},
		{"leading paren", "(src/**)", PermissionEffectDeny, "invalid permission rule pattern"},
		{"empty argument pattern", "read()", PermissionEffectDeny, "empty argument pattern"},
		{"nested parens", "bash(echo (hi))", PermissionEffectDeny, "invalid permission rule pattern"},
		{"whitespace in tool name", "read tool(src/**)", PermissionEffectDeny, "invalid tool name"},
		{"absolute path pattern", "read(/etc/passwd)", PermissionEffectDeny, "absolute path patterns are not allowed"},
		{"traversing path pattern", "read(../outside/**)", PermissionEffectDeny, "escapes the workspace"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePermissionRule(tc.pattern, tc.effect)
			if err == nil {
				t.Fatalf("pattern %q with effect %q should be rejected", tc.pattern, tc.effect)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q should mention %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestParsePermissionRule_AcceptsValidPatterns(t *testing.T) {
	for _, pattern := range []string{
		"read",
		"read(src/**)",
		"bash(git status:*)",
		"  read(src/**)  ", // trimmed
		"apply_patch(docs/*.md)",
		"web_fetch(https://example.com/*)", // non-path tool: no path validation
	} {
		for _, effect := range []PermissionEffect{PermissionEffectAllow, PermissionEffectAsk, PermissionEffectDeny} {
			if _, err := ParsePermissionRule(pattern, effect); err != nil {
				t.Errorf("pattern %q with effect %q should be accepted: %v", pattern, effect, err)
			}
		}
	}
}

func TestValidatePermissionRules_ReportsTheOffendingIndex(t *testing.T) {
	rules := []PermissionRule{
		{Pattern: "read(src/**)", Effect: PermissionEffectAllow},
		{Pattern: "read(src/**)", Effect: PermissionEffectDeny},
		{Pattern: "bad(", Effect: PermissionEffectDeny},
	}
	err := ValidatePermissionRules(rules)
	if err == nil {
		t.Fatal("a malformed rule must fail validation")
	}
	// The index matters: an operator with a long rule list needs to know which
	// entry is wrong.
	if !strings.Contains(err.Error(), "permission rule 2") {
		t.Errorf("error %q should identify rule index 2", err.Error())
	}

	if err := ValidatePermissionRules(rules[:2]); err != nil {
		t.Errorf("valid rules should pass validation: %v", err)
	}
	if err := ValidatePermissionRules(nil); err != nil {
		t.Errorf("an empty rule set should pass validation: %v", err)
	}
}

func TestEvaluatePermissionRules_EdgeCases(t *testing.T) {
	ws := t.TempDir()

	if effect, err := EvaluatePermissionRules(nil, "read", json.RawMessage(`{}`), ws); err != nil || effect != PermissionEffectAllow {
		t.Errorf("no rules should allow, got %q (err %v)", effect, err)
	}
	if effect, err := EvaluatePermissionRules(
		[]PermissionRule{{Pattern: "read", Effect: PermissionEffectDeny}}, "", json.RawMessage(`{}`), ws,
	); err != nil || effect != PermissionEffectAllow {
		t.Errorf("an empty tool name should allow, got %q (err %v)", effect, err)
	}

	// A malformed rule surfaces as an error rather than being skipped, so a
	// broken config cannot quietly disable enforcement.
	_, err := EvaluatePermissionRules(
		[]PermissionRule{{Pattern: "read(", Effect: PermissionEffectDeny}}, "read", json.RawMessage(`{}`), ws)
	if err == nil {
		t.Error("a malformed rule should surface as an evaluation error")
	}

	// A bare tool rule matches every invocation of that tool regardless of args.
	bare := []PermissionRule{{Pattern: "bash", Effect: PermissionEffectDeny}}
	if effect, _ := EvaluatePermissionRules(bare, "bash", json.RawMessage(`{"command":"anything"}`), ws); effect != PermissionEffectDeny {
		t.Error("a bare tool rule should match any invocation")
	}
	if effect, _ := EvaluatePermissionRules(bare, "read", json.RawMessage(`{"path":"x"}`), ws); effect != PermissionEffectAllow {
		t.Error("a bare tool rule must not match a different tool")
	}
}

func TestPermissionRulePrecedence(t *testing.T) {
	ws := t.TempDir()

	t.Run("a more specific literal beats a broader glob", func(t *testing.T) {
		rules := []PermissionRule{
			{Pattern: "bash(*)", Effect: PermissionEffectDeny},
			{Pattern: "bash(git status)", Effect: PermissionEffectAllow},
		}
		args := json.RawMessage(`{"command":"git status"}`)
		if effect, _ := EvaluatePermissionRules(rules, "bash", args, ws); effect != PermissionEffectAllow {
			t.Errorf("the exact rule should win, got %q", effect)
		}
	})

	t.Run("at equal specificity deny beats ask beats allow", func(t *testing.T) {
		args := json.RawMessage(`{"command":"git status"}`)
		all := []PermissionRule{
			{Pattern: "bash(git status)", Effect: PermissionEffectAllow},
			{Pattern: "bash(git status)", Effect: PermissionEffectAsk},
			{Pattern: "bash(git status)", Effect: PermissionEffectDeny},
		}
		if effect, _ := EvaluatePermissionRules(all, "bash", args, ws); effect != PermissionEffectDeny {
			t.Errorf("deny should win a tie, got %q", effect)
		}
		if effect, _ := EvaluatePermissionRules(all[:2], "bash", args, ws); effect != PermissionEffectAsk {
			t.Errorf("ask should beat allow, got %q", effect)
		}
	})

	t.Run("a bare tool rule is less specific than any argument rule", func(t *testing.T) {
		rules := []PermissionRule{
			{Pattern: "bash", Effect: PermissionEffectDeny},
			{Pattern: "bash(git status:*)", Effect: PermissionEffectAllow},
		}
		args := json.RawMessage(`{"command":"git status --short"}`)
		if effect, _ := EvaluatePermissionRules(rules, "bash", args, ws); effect != PermissionEffectAllow {
			t.Errorf("the argument rule should win over the bare tool rule, got %q", effect)
		}
	})
}

func TestPermissionEffectPriorityOrdering(t *testing.T) {
	if permissionEffectPriority(PermissionEffectDeny) <= permissionEffectPriority(PermissionEffectAsk) {
		t.Error("deny must outrank ask")
	}
	if permissionEffectPriority(PermissionEffectAsk) <= permissionEffectPriority(PermissionEffectAllow) {
		t.Error("ask must outrank allow")
	}
	if permissionEffectPriority(PermissionEffect("nonsense")) != 0 {
		t.Error("an unrecognized effect must rank lowest")
	}
}

func TestPermissionRuleSetJSON(t *testing.T) {
	set := NewPermissionRuleSet([]PermissionRule{
		{Pattern: "read(src/**)", Effect: PermissionEffectAllow},
		{Pattern: "bash", Effect: PermissionEffectDeny},
	})
	raw, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// It encodes as a plain array, not as an object wrapping one.
	if !strings.HasPrefix(string(raw), "[") {
		t.Errorf("rule set should encode as a JSON array, got %s", raw)
	}

	var decoded PermissionRuleSet
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Items) != 2 || decoded.Items[0].Pattern != "read(src/**)" {
		t.Errorf("round trip lost data: %+v", decoded.Items)
	}
	if err := json.Unmarshal([]byte(`{"not":"an array"}`), &decoded); err == nil {
		t.Error("a non-array payload should fail to decode")
	}

	// NewPermissionRuleSet copies, so later mutation of the caller's slice
	// cannot reach into the stored rules.
	original := []PermissionRule{{Pattern: "read", Effect: PermissionEffectDeny}}
	owned := NewPermissionRuleSet(original)
	original[0].Pattern = "mutated"
	if owned.Items[0].Pattern != "read" {
		t.Error("NewPermissionRuleSet must take an owned copy")
	}
	if NewPermissionRuleSet(nil) != nil {
		t.Error("a nil rule slice should produce a nil set")
	}
}

func TestPermissionConfigsEqual(t *testing.T) {
	base := PermissionConfig{
		Sandbox:  "workspace",
		Approval: "permissions",
		Rules:    NewPermissionRuleSet([]PermissionRule{{Pattern: "read", Effect: PermissionEffectDeny}}),
	}
	same := PermissionConfig{
		Sandbox:  "workspace",
		Approval: "permissions",
		Rules:    NewPermissionRuleSet([]PermissionRule{{Pattern: "read", Effect: PermissionEffectDeny}}),
	}
	if !permissionConfigsEqual(base, same) {
		t.Error("identical configs should compare equal")
	}

	differentRule := PermissionConfig{
		Sandbox: "workspace", Approval: "permissions",
		Rules: NewPermissionRuleSet([]PermissionRule{{Pattern: "read", Effect: PermissionEffectAllow}}),
	}
	if permissionConfigsEqual(base, differentRule) {
		t.Error("a differing rule effect should compare unequal")
	}

	differentSandbox := base
	differentSandbox.Sandbox = "local"
	if permissionConfigsEqual(base, differentSandbox) {
		t.Error("a differing sandbox should compare unequal")
	}

	extraRule := PermissionConfig{
		Sandbox: "workspace", Approval: "permissions",
		Rules: NewPermissionRuleSet([]PermissionRule{
			{Pattern: "read", Effect: PermissionEffectDeny},
			{Pattern: "bash", Effect: PermissionEffectDeny},
		}),
	}
	if permissionConfigsEqual(base, extraRule) {
		t.Error("differing rule counts should compare unequal")
	}
	if !permissionConfigsEqual(PermissionConfig{}, PermissionConfig{}) {
		t.Error("two zero configs should compare equal")
	}
}

func TestShellWordSplitting(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  []string
		valid bool
	}{
		{"plain", "git status", []string{"git", "status"}, true},
		{"collapsed whitespace", "git   status", []string{"git", "status"}, true},
		{"single quotes", "echo 'hello world'", []string{"echo", "hello world"}, true},
		{"double quotes", `echo "hello world"`, []string{"echo", "hello world"}, true},
		{"escaped space", `echo hello\ world`, []string{"echo", "hello world"}, true},
		{"escape inside double quotes", `echo "a\"b"`, []string{"echo", `a"b`}, true},
		{"tabs and newlines", "echo\ta\nb", []string{"echo", "a", "b"}, true},
		{"empty", "", nil, true},
		{"unterminated single quote", "echo 'oops", nil, false},
		{"unterminated double quote", `echo "oops`, nil, false},
		{"trailing escape", `echo oops\`, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := shellWords(tc.in)
			if ok != tc.valid {
				t.Fatalf("shellWords(%q) ok = %v, want %v", tc.in, ok, tc.valid)
			}
			if !tc.valid {
				return
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("shellWords(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeShellCommandAndPattern(t *testing.T) {
	got, ok := normalizeShellCommand("/usr/local/bin/git   status")
	if !ok || got != "git status" {
		t.Errorf("normalizeShellCommand = %q (ok=%v), want %q", got, ok, "git status")
	}
	if _, ok := normalizeShellCommand("   "); ok {
		t.Error("an empty command should not normalize")
	}
	if _, ok := normalizeShellCommand("echo 'unterminated"); ok {
		t.Error("an unparseable command should not normalize")
	}

	// The ":*" suffix convention becomes a trailing wildcard argument.
	if got := normalizeShellPattern("git status:*"); got != "git status *" {
		t.Errorf("normalizeShellPattern = %q, want %q", got, "git status *")
	}
	// A pattern that cannot be shell-split still collapses its whitespace.
	if got := normalizeShellPattern("echo   'unterminated"); !strings.Contains(got, "echo") {
		t.Errorf("normalizeShellPattern on an unparseable pattern = %q", got)
	}
}
