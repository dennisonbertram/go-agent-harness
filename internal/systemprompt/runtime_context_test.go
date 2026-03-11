package systemprompt

import (
	"strings"
	"testing"
	"time"
)

func baseRuntimeContextInput() RuntimeContextInput {
	t := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	return RuntimeContextInput{
		RunStartedAt: t,
		Now:          t,
		Step:         1,
	}
}

func TestBuildRuntimeContext_WithEnvironment(t *testing.T) {
	in := baseRuntimeContextInput()
	in.Environment = EnvironmentInfo{
		OS:         "darwin",
		Arch:       "arm64",
		Hostname:   "myhost.local",
		Username:   "dennis",
		WorkingDir: "/Users/dennis/Develop/go-agent-harness",
		Shell:      "/bin/zsh",
		GoVersion:  "go1.22.1",
	}

	out := BuildRuntimeContext(in)

	if !strings.Contains(out, "<environment>") {
		t.Error("expected <environment> block to be present")
	}
	if !strings.Contains(out, "</environment>") {
		t.Error("expected </environment> closing tag to be present")
	}
	// Verify all fields appear
	cases := []string{
		"os: darwin",
		"arch: arm64",
		"hostname: myhost.local",
		"user: dennis",
		"working_dir: /Users/dennis/Develop/go-agent-harness",
		"shell: /bin/zsh",
		"go_version: go1.22.1",
	}
	for _, want := range cases {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	// Environment block must be inside runtime_context
	rtStart := strings.Index(out, "<runtime_context>")
	rtEnd := strings.Index(out, "</runtime_context>")
	envStart := strings.Index(out, "<environment>")
	if rtStart < 0 || rtEnd < 0 || envStart < 0 {
		t.Fatal("missing expected tags")
	}
	if envStart < rtStart || envStart > rtEnd {
		t.Error("expected <environment> block to be inside <runtime_context>")
	}
}

func TestBuildRuntimeContext_EmptyEnvironment(t *testing.T) {
	in := baseRuntimeContextInput()
	// Environment is zero value — all fields empty strings

	out := BuildRuntimeContext(in)

	if strings.Contains(out, "<environment>") {
		t.Error("expected no <environment> block when EnvironmentInfo is zero value")
	}
}

func TestBuildRuntimeContext_PartialEnvironment(t *testing.T) {
	in := baseRuntimeContextInput()
	in.Environment = EnvironmentInfo{
		OS:   "linux",
		Arch: "amd64",
		// Hostname, Username, WorkingDir, Shell, GoVersion left empty
	}

	out := BuildRuntimeContext(in)

	if !strings.Contains(out, "<environment>") {
		t.Error("expected <environment> block to be present")
	}
	if !strings.Contains(out, "os: linux") {
		t.Errorf("expected 'os: linux' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "arch: amd64") {
		t.Errorf("expected 'arch: amd64' in output, got:\n%s", out)
	}
	// Empty fields must NOT appear
	emptyFields := []string{
		"hostname:",
		"user:",
		"working_dir:",
		"shell:",
		"go_version:",
	}
	for _, field := range emptyFields {
		if strings.Contains(out, field) {
			t.Errorf("expected field %q to be absent when empty, got:\n%s", field, out)
		}
	}
}
