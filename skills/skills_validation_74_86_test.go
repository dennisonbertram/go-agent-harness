// Package skills_validation tests that the bundled SKILL.md files in this
// directory parse correctly and satisfy required field constraints.
//
// This file covers Issue #74 (Cloudflare Containers Skill).
// Issue #86 is documentation-only and requires no test.
package skills_validation

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-agent-harness/internal/skills"
)

// skillsDir7486 returns the absolute path to the skills/ directory that
// contains this test file. Using runtime.Caller makes the path independent
// of the working directory from which `go test` is invoked.
func skillsDir7486(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

// loadAllSkills7486 uses the production Loader to discover all SKILL.md files
// in the skills/ directory and returns them indexed by name.
func loadAllSkills7486(t *testing.T) map[string]skills.Skill {
	t.Helper()
	dir := skillsDir7486(t)
	loader := skills.NewLoader(skills.LoaderConfig{GlobalDir: dir})
	all, err := loader.Load()
	if err != nil {
		t.Fatalf("skills.Loader.Load() error: %v", err)
	}
	m := make(map[string]skills.Skill, len(all))
	for _, s := range all {
		m[s.Name] = s
	}
	return m
}

// --- Issue #74: Cloudflare Containers Skill ---

func TestCloudflareContainersSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s, ok := all["cloudflare-containers"]
	if !ok {
		t.Fatal("cloudflare-containers skill not found; expected SKILL.md in skills/cloudflare-containers/")
	}
	if s.Name != "cloudflare-containers" {
		t.Errorf("Name = %q, want %q", s.Name, "cloudflare-containers")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body (markdown content) must not be empty")
	}
}

func TestCloudflareContainersSkill_HasBetaNotice(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	bodyLower := strings.ToLower(s.Body)
	// The skill must note that it is not yet GA
	hasBeta := strings.Contains(bodyLower, "beta") || strings.Contains(bodyLower, "preview")
	if !hasBeta {
		t.Error("cloudflare-containers body should note beta/preview status")
	}
}

func TestCloudflareContainersSkill_HasWranglerCLI(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	if !strings.Contains(s.Body, "wrangler") {
		t.Error("cloudflare-containers body should reference wrangler CLI")
	}
}

func TestCloudflareContainersSkill_HasDeployCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	if !strings.Contains(s.Body, "wrangler deploy") {
		t.Error("cloudflare-containers body should include 'wrangler deploy' command")
	}
}

func TestCloudflareContainersSkill_HasInstanceTypes(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	if !strings.Contains(s.Body, "standard-1") {
		t.Error("cloudflare-containers body should document instance types including standard-1")
	}
}

func TestCloudflareContainersSkill_HasContainerConfig(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	// Should document wrangler.jsonc or wrangler.toml container configuration
	hasConfig := strings.Contains(s.Body, "wrangler.jsonc") || strings.Contains(s.Body, "wrangler.toml")
	if !hasConfig {
		t.Error("cloudflare-containers body should document wrangler.jsonc or wrangler.toml container configuration")
	}
}

func TestCloudflareContainersSkill_HasScaleToZero(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	bodyLower := strings.ToLower(s.Body)
	hasScaleToZero := strings.Contains(bodyLower, "scale-to-zero") || strings.Contains(bodyLower, "sleep_after") || strings.Contains(s.Body, "sleepAfter")
	if !hasScaleToZero {
		t.Error("cloudflare-containers body should document scale-to-zero / sleep_after configuration")
	}
}

func TestCloudflareContainersSkill_HasDockerfileContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "dockerfile") {
		t.Error("cloudflare-containers body should reference Dockerfile")
	}
}

func TestCloudflareContainersSkill_HasWorkersComparison(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	bodyLower := strings.ToLower(s.Body)
	// Should explain difference from Workers
	if !strings.Contains(bodyLower, "worker") {
		t.Error("cloudflare-containers body should document how containers differ from Workers")
	}
}

func TestCloudflareContainersSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	if len(s.Triggers) == 0 {
		t.Error("cloudflare-containers should have at least one trigger phrase in description")
	}
}

func TestCloudflareContainersSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	if len(s.AllowedTools) == 0 {
		t.Error("cloudflare-containers should declare allowed-tools")
	}
}

func TestCloudflareContainersSkill_HasFirecrackerOrMicroVM(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	bodyLower := strings.ToLower(s.Body)
	hasMicroVM := strings.Contains(bodyLower, "firecracker") || strings.Contains(bodyLower, "microvm")
	if !hasMicroVM {
		t.Error("cloudflare-containers body should reference Firecracker microVMs")
	}
}

func TestCloudflareContainersSkill_HasPricingInfo(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "pric") {
		t.Error("cloudflare-containers body should include pricing information")
	}
}

func TestCloudflareContainersSkill_NotYetGA(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	s := all["cloudflare-containers"]
	bodyLower := strings.ToLower(s.Body)
	// The skill should explicitly note it will be updated at GA
	hasGANote := strings.Contains(bodyLower, "ga") || strings.Contains(bodyLower, "generally available") || strings.Contains(bodyLower, "general availability")
	if !hasGANote {
		t.Error("cloudflare-containers body should note pending GA status")
	}
}

// --- Cross-cutting validation for issues #74, #86 ---

// TestIssue74Skills_HaveVersion ensures the new SKILL.md has version: 1.
func TestIssue74Skills_HaveVersion(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	newSkills := []string{
		"cloudflare-containers",
	}
	for _, name := range newSkills {
		s, ok := all[name]
		if !ok {
			continue // covered by individual parse tests
		}
		if s.Version != 1 {
			t.Errorf("skill %q: Version = %d, want 1", name, s.Version)
		}
	}
}

// TestIssue74Skills_HaveAllowedTools ensures the new skill declares allowed-tools.
func TestIssue74Skills_HaveAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	newSkills := []string{
		"cloudflare-containers",
	}
	for _, name := range newSkills {
		s, ok := all[name]
		if !ok {
			continue // covered by individual parse tests
		}
		if len(s.AllowedTools) == 0 {
			t.Errorf("skill %q: allowed-tools should be declared", name)
		}
	}
}

// TestIssue74Skills_HaveTriggers ensures the new skill defines at least one trigger phrase.
func TestIssue74Skills_HaveTriggers(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	newSkills := []string{
		"cloudflare-containers",
	}
	for _, name := range newSkills {
		s, ok := all[name]
		if !ok {
			continue // covered by individual parse tests
		}
		if len(s.Triggers) == 0 {
			t.Errorf("skill %q: should have at least one trigger phrase in description", name)
		}
	}
}

// TestIssue74Skills_DefaultToConversationContext ensures the new skill does not
// accidentally use the fork context.
func TestIssue74Skills_DefaultToConversationContext(t *testing.T) {
	t.Parallel()
	all := loadAllSkills7486(t)
	newSkills := []string{
		"cloudflare-containers",
	}
	for _, name := range newSkills {
		s, ok := all[name]
		if !ok {
			continue
		}
		if s.Context == skills.ContextFork {
			t.Errorf("skill %q uses context=fork; this skill should use conversation context", name)
		}
	}
}
