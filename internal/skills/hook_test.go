package skills

import (
	"testing"
)

func TestAutoInvokeHook_ExplicitNoArgs(t *testing.T) {
	reg := NewRegistry()
	reg.skills["deploy"] = &Skill{
		Name:       "deploy",
		Body:       "Deploy to $ARGUMENTS",
		FilePath:   "/skills/deploy/SKILL.md",
		AutoInvoke: true,
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("/deploy")

	if name != "deploy" {
		t.Errorf("expected name %q, got %q", "deploy", name)
	}
	if content != "Deploy to " {
		t.Errorf("expected content %q, got %q", "Deploy to ", content)
	}
}

func TestAutoInvokeHook_ExplicitWithArgs(t *testing.T) {
	reg := NewRegistry()
	reg.skills["deploy"] = &Skill{
		Name:       "deploy",
		Body:       "Deploy $1 to $2. Full: $ARGUMENTS",
		FilePath:   "/skills/deploy/SKILL.md",
		AutoInvoke: true,
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("/deploy staging eu-west")

	if name != "deploy" {
		t.Errorf("expected name %q, got %q", "deploy", name)
	}
	want := "Deploy staging to eu-west. Full: staging eu-west"
	if content != want {
		t.Errorf("expected content %q, got %q", want, content)
	}
}

func TestAutoInvokeHook_ExplicitUnknownSkill(t *testing.T) {
	reg := NewRegistry()

	hook := AutoInvokeHook(reg)
	name, content := hook("/nonexistent some args")

	if name != "" {
		t.Errorf("expected empty name, got %q", name)
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestAutoInvokeHook_AutoInvokeSingleMatch(t *testing.T) {
	reg := NewRegistry()
	reg.skills["review"] = &Skill{
		Name:       "review",
		Body:       "Review the code: $ARGUMENTS",
		FilePath:   "/skills/review/SKILL.md",
		AutoInvoke: true,
		Triggers:   []string{"review my code"},
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("please review my code carefully")

	if name != "review" {
		t.Errorf("expected name %q, got %q", "review", name)
	}
	want := "Review the code: please review my code carefully"
	if content != want {
		t.Errorf("expected content %q, got %q", want, content)
	}
}

func TestAutoInvokeHook_AutoInvokeMultipleMatchesReturnsEmpty(t *testing.T) {
	reg := NewRegistry()
	reg.skills["review"] = &Skill{
		Name:       "review",
		Body:       "Review body",
		FilePath:   "/skills/review/SKILL.md",
		AutoInvoke: true,
		Triggers:   []string{"review code"},
	}
	reg.skills["lint"] = &Skill{
		Name:       "lint",
		Body:       "Lint body",
		FilePath:   "/skills/lint/SKILL.md",
		AutoInvoke: true,
		Triggers:   []string{"review code"},
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("review code please")

	if name != "" {
		t.Errorf("expected empty name for ambiguous match, got %q", name)
	}
	if content != "" {
		t.Errorf("expected empty content for ambiguous match, got %q", content)
	}
}

func TestAutoInvokeHook_SkipsNonAutoInvoke(t *testing.T) {
	reg := NewRegistry()
	reg.skills["secret"] = &Skill{
		Name:       "secret",
		Body:       "Secret body",
		FilePath:   "/skills/secret/SKILL.md",
		AutoInvoke: false,
		Triggers:   []string{"do secret thing"},
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("do secret thing now")

	if name != "" {
		t.Errorf("expected empty name for non-auto-invoke skill, got %q", name)
	}
	if content != "" {
		t.Errorf("expected empty content for non-auto-invoke skill, got %q", content)
	}
}

func TestAutoInvokeHook_EmptyMessage(t *testing.T) {
	reg := NewRegistry()
	reg.skills["test"] = &Skill{
		Name:       "test",
		Body:       "Test body",
		FilePath:   "/skills/test/SKILL.md",
		AutoInvoke: true,
		Triggers:   []string{"test"},
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("")

	if name != "" {
		t.Errorf("expected empty name, got %q", name)
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestAutoInvokeHook_WhitespaceOnlyMessage(t *testing.T) {
	reg := NewRegistry()
	hook := AutoInvokeHook(reg)
	name, content := hook("   \t\n  ")

	if name != "" {
		t.Errorf("expected empty name, got %q", name)
	}
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestAutoInvokeHook_SlashCommandFallsThrough(t *testing.T) {
	// /unknown-cmd is not a registered skill, but trigger matching may pick it up
	reg := NewRegistry()
	reg.skills["helper"] = &Skill{
		Name:       "helper",
		Body:       "Helper: $ARGUMENTS",
		FilePath:   "/skills/helper/SKILL.md",
		AutoInvoke: true,
		Triggers:   []string{"/unknown-cmd"},
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("/unknown-cmd do stuff")

	// "/unknown-cmd" is not a skill, so explicit lookup fails.
	// But the message contains the trigger "/unknown-cmd", so auto-invoke matches.
	if name != "helper" {
		t.Errorf("expected name %q, got %q", "helper", name)
	}
	if content == "" {
		t.Error("expected non-empty content from trigger fallthrough")
	}
}

func TestAutoInvokeHook_ExplicitTakesPrecedenceOverTrigger(t *testing.T) {
	reg := NewRegistry()
	reg.skills["deploy"] = &Skill{
		Name:       "deploy",
		Body:       "Deploy: $ARGUMENTS",
		FilePath:   "/skills/deploy/SKILL.md",
		AutoInvoke: true,
		Triggers:   []string{"deploy"},
	}

	hook := AutoInvokeHook(reg)
	name, content := hook("/deploy production")

	if name != "deploy" {
		t.Errorf("expected name %q, got %q", "deploy", name)
	}
	// Explicit invocation: args = "production"
	want := "Deploy: production"
	if content != want {
		t.Errorf("expected content %q, got %q", want, content)
	}
}

func TestBuildVars(t *testing.T) {
	skill := &Skill{
		FilePath: "/home/user/skills/my-skill/SKILL.md",
	}

	vars := buildVars(skill, "alpha beta gamma", "/workspace")

	tests := []struct {
		key  string
		want string
	}{
		{"$ARGUMENTS", "alpha beta gamma"},
		{"$WORKSPACE", "/workspace"},
		{"$SKILL_DIR", "/home/user/skills/my-skill"},
		{"$1", "alpha"},
		{"$2", "beta"},
		{"$3", "gamma"},
		{"$4", ""},
		{"$9", ""},
	}

	for _, tt := range tests {
		got := vars[tt.key]
		if got != tt.want {
			t.Errorf("buildVars[%s] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestBuildVars_NoArgs(t *testing.T) {
	skill := &Skill{
		FilePath: "/skills/test/SKILL.md",
	}

	vars := buildVars(skill, "", "")

	if vars["$ARGUMENTS"] != "" {
		t.Errorf("expected empty $ARGUMENTS, got %q", vars["$ARGUMENTS"])
	}
	if vars["$WORKSPACE"] != "" {
		t.Errorf("expected empty $WORKSPACE, got %q", vars["$WORKSPACE"])
	}
	if vars["$1"] != "" {
		t.Errorf("expected empty $1, got %q", vars["$1"])
	}
}
