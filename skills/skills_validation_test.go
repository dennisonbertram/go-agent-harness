// Package skills_validation tests that the bundled SKILL.md files in this
// directory parse correctly and satisfy required field constraints.
//
// Each SKILL.md file is loaded via the production Loader to ensure any future
// loader changes are caught by these tests automatically.
package skills_validation

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-agent-harness/internal/skills"
)

// skillsDir returns the absolute path to the skills/ directory that contains
// this test file. Using runtime.Caller makes the path independent of the
// working directory from which `go test` is invoked.
func skillsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

// loadAllSkills uses the production Loader to discover all SKILL.md files in
// the skills/ directory and returns them indexed by name.
func loadAllSkills(t *testing.T) map[string]skills.Skill {
	t.Helper()
	dir := skillsDir(t)
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

// --- Issue #59: Git Workflow Skills ---

func TestGitBranchingSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["git-branching"]
	if !ok {
		t.Fatal("git-branching skill not found; expected SKILL.md in skills/git-branching/")
	}
	if s.Name != "git-branching" {
		t.Errorf("Name = %q, want %q", s.Name, "git-branching")
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

func TestGitBranchingSkill_HasBranchingContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["git-branching"]
	if !strings.Contains(s.Body, "feature/") {
		t.Error("git-branching body should document feature/ branch pattern")
	}
	if !strings.Contains(s.Body, "fix/") {
		t.Error("git-branching body should document fix/ branch pattern")
	}
	if !strings.Contains(s.Body, "git checkout") {
		t.Error("git-branching body should include git checkout command")
	}
}

func TestGitBranchingSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["git-branching"]
	if len(s.Triggers) == 0 {
		t.Error("git-branching should have at least one trigger phrase in description")
	}
}

func TestGitPRCreateSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["git-pr-create"]
	if !ok {
		t.Fatal("git-pr-create skill not found; expected SKILL.md in skills/git-pr-create/")
	}
	if s.Name != "git-pr-create" {
		t.Errorf("Name = %q, want %q", s.Name, "git-pr-create")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestGitPRCreateSkill_HasPRContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["git-pr-create"]
	if !strings.Contains(s.Body, "gh pr create") {
		t.Error("git-pr-create body should include 'gh pr create' command")
	}
	if !strings.Contains(s.Body, "--title") {
		t.Error("git-pr-create body should document --title flag")
	}
	if !strings.Contains(s.Body, "--draft") {
		t.Error("git-pr-create body should document --draft flag for WIP PRs")
	}
}

func TestGitPRReviewSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["git-pr-review"]
	if !ok {
		t.Fatal("git-pr-review skill not found; expected SKILL.md in skills/git-pr-review/")
	}
	if s.Name != "git-pr-review" {
		t.Errorf("Name = %q, want %q", s.Name, "git-pr-review")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestGitPRReviewSkill_HasReviewWorkflow(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["git-pr-review"]
	if !strings.Contains(s.Body, "gh pr checkout") {
		t.Error("git-pr-review body should include 'gh pr checkout'")
	}
	if !strings.Contains(s.Body, "go test ./... -race") {
		t.Error("git-pr-review body should include race detector test command")
	}
	if !strings.Contains(s.Body, "gh pr review") {
		t.Error("git-pr-review body should include 'gh pr review' commands")
	}
	if !strings.Contains(s.Body, "--approve") {
		t.Error("git-pr-review body should document --approve flag")
	}
	if !strings.Contains(s.Body, "--request-changes") {
		t.Error("git-pr-review body should document --request-changes flag")
	}
}

func TestGitMergeStrategySkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["git-merge-strategy"]
	if !ok {
		t.Fatal("git-merge-strategy skill not found; expected SKILL.md in skills/git-merge-strategy/")
	}
	if s.Name != "git-merge-strategy" {
		t.Errorf("Name = %q, want %q", s.Name, "git-merge-strategy")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestGitMergeStrategySkill_HasMergeContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["git-merge-strategy"]
	if !strings.Contains(s.Body, "--squash") {
		t.Error("git-merge-strategy body should document squash merge")
	}
	if !strings.Contains(s.Body, "--rebase") {
		t.Error("git-merge-strategy body should document rebase merge")
	}
	if !strings.Contains(s.Body, "--force-with-lease") {
		t.Error("git-merge-strategy body should document --force-with-lease (safe force push)")
	}
}

func TestGitMergeStrategySkill_ForbidsPlainForcePush(t *testing.T) {
	t.Parallel()
	path := filepath.Join(skillsDir(t), "git-merge-strategy", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading SKILL.md: %v", err)
	}
	content := string(data)
	// The skill file must explain the safety rule against plain --force push
	// and recommend --force-with-lease as the alternative.
	if !strings.Contains(content, "--force-with-lease") {
		t.Error("SKILL.md must document --force-with-lease as the safe alternative to --force")
	}
	if !strings.Contains(content, "Never use") && !strings.Contains(content, "never use") {
		t.Error("SKILL.md must explicitly warn against using plain --force push")
	}
}

// --- Issue #61: CI/CD Management Skills ---

func TestGitHubActionsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["github-actions"]
	if !ok {
		t.Fatal("github-actions skill not found; expected SKILL.md in skills/github-actions/")
	}
	if s.Name != "github-actions" {
		t.Errorf("Name = %q, want %q", s.Name, "github-actions")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestGitHubActionsSkill_HasWorkflowCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["github-actions"]
	if !strings.Contains(s.Body, "gh run list") {
		t.Error("github-actions body should include 'gh run list'")
	}
	if !strings.Contains(s.Body, "gh run view") {
		t.Error("github-actions body should include 'gh run view'")
	}
	if !strings.Contains(s.Body, "--log-failed") {
		t.Error("github-actions body should document --log-failed flag")
	}
	if !strings.Contains(s.Body, "gh run rerun") {
		t.Error("github-actions body should include 'gh run rerun'")
	}
	if !strings.Contains(s.Body, "--failed") {
		t.Error("github-actions body should document --failed flag for partial rerun")
	}
}

func TestGitHubActionsSkill_HasWorkflowManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["github-actions"]
	if !strings.Contains(s.Body, "gh workflow list") {
		t.Error("github-actions body should include 'gh workflow list'")
	}
	if !strings.Contains(s.Body, "gh workflow run") {
		t.Error("github-actions body should include 'gh workflow run'")
	}
}

func TestCIDebugSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["ci-debug"]
	if !ok {
		t.Fatal("ci-debug skill not found; expected SKILL.md in skills/ci-debug/")
	}
	if s.Name != "ci-debug" {
		t.Errorf("Name = %q, want %q", s.Name, "ci-debug")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestCIDebugSkill_HasDiagnosticPatterns(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ci-debug"]
	// Must cover key failure types
	if !strings.Contains(s.Body, "Test Failures") && !strings.Contains(s.Body, "test failure") {
		t.Error("ci-debug body should cover test failure patterns")
	}
	if !strings.Contains(s.Body, "DATA RACE") {
		t.Error("ci-debug body should cover race condition detection")
	}
	if !strings.Contains(s.Body, "flaky") || !strings.Contains(s.Body, "Flaky") {
		t.Error("ci-debug body should cover flaky test detection")
	}
	if !strings.Contains(s.Body, "--log-failed") {
		t.Error("ci-debug body should reference --log-failed for fetching CI logs")
	}
}

func TestCIDebugSkill_HasLocalReproductionSteps(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ci-debug"]
	if !strings.Contains(s.Body, "go test") {
		t.Error("ci-debug body should include go test commands for local reproduction")
	}
	if !strings.Contains(s.Body, "-race") {
		t.Error("ci-debug body should include race detector flag")
	}
}

// --- Issue #63: Security Scanning Skills ---

func TestGosecSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["gosec"]
	if !ok {
		t.Fatal("gosec skill not found; expected SKILL.md in skills/gosec/")
	}
	if s.Name != "gosec" {
		t.Errorf("Name = %q, want %q", s.Name, "gosec")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestGosecSkill_HasInstallCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["gosec"]
	if !strings.Contains(s.Body, "go install") {
		t.Error("gosec body should include installation instructions")
	}
	if !strings.Contains(s.Body, "gosec") {
		t.Error("gosec body should reference the gosec command")
	}
}

func TestGosecSkill_HasKeyRules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["gosec"]
	// Must document critical rules
	if !strings.Contains(s.Body, "G101") {
		t.Error("gosec body should document G101 (hardcoded credentials)")
	}
	if !strings.Contains(s.Body, "G201") {
		t.Error("gosec body should document G201 (SQL injection)")
	}
	if !strings.Contains(s.Body, "G204") {
		t.Error("gosec body should document G204 (command injection)")
	}
}

func TestGosecSkill_HasSeverityLevels(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["gosec"]
	if !strings.Contains(s.Body, "HIGH") {
		t.Error("gosec body should document HIGH severity level")
	}
	if !strings.Contains(s.Body, "MEDIUM") {
		t.Error("gosec body should document MEDIUM severity level")
	}
}

func TestGosecSkill_HasJsonOutput(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["gosec"]
	if !strings.Contains(s.Body, "-fmt json") {
		t.Error("gosec body should document JSON output format flag")
	}
}

func TestGoVulnCheckSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["go-vuln-check"]
	if !ok {
		t.Fatal("go-vuln-check skill not found; expected SKILL.md in skills/go-vuln-check/")
	}
	if s.Name != "go-vuln-check" {
		t.Errorf("Name = %q, want %q", s.Name, "go-vuln-check")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestGoVulnCheckSkill_HasInstallCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-vuln-check"]
	if !strings.Contains(s.Body, "go install") {
		t.Error("go-vuln-check body should include installation instructions")
	}
	if !strings.Contains(s.Body, "govulncheck") {
		t.Error("go-vuln-check body should reference the govulncheck command")
	}
}

func TestGoVulnCheckSkill_HasBinaryMode(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-vuln-check"]
	if !strings.Contains(s.Body, "-mode=binary") {
		t.Error("go-vuln-check body should document binary scanning mode")
	}
}

func TestGoVulnCheckSkill_HasJsonOutput(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-vuln-check"]
	if !strings.Contains(s.Body, "-json") {
		t.Error("go-vuln-check body should document JSON output flag")
	}
}

func TestGoVulnCheckSkill_HasFixInstructions(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-vuln-check"]
	if !strings.Contains(s.Body, "go get") {
		t.Error("go-vuln-check body should include 'go get' for updating dependencies")
	}
	if !strings.Contains(s.Body, "go mod tidy") {
		t.Error("go-vuln-check body should include 'go mod tidy'")
	}
}

func TestDotenvAuditSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["dotenv-audit"]
	if !ok {
		t.Fatal("dotenv-audit skill not found; expected SKILL.md in skills/dotenv-audit/")
	}
	if s.Name != "dotenv-audit" {
		t.Errorf("Name = %q, want %q", s.Name, "dotenv-audit")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestDotenvAuditSkill_HasGitignoreCheck(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["dotenv-audit"]
	if !strings.Contains(s.Body, ".gitignore") {
		t.Error("dotenv-audit body should check .gitignore for .env files")
	}
}

func TestDotenvAuditSkill_HasCommittedFilesCheck(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["dotenv-audit"]
	if !strings.Contains(s.Body, "git ls-files") {
		t.Error("dotenv-audit body should check for committed .env files with git ls-files")
	}
	if !strings.Contains(s.Body, "git rm --cached") {
		t.Error("dotenv-audit body should document how to untrack committed .env files")
	}
}

func TestDotenvAuditSkill_HasHardcodedSecretCheck(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["dotenv-audit"]
	if !strings.Contains(s.Body, "grep") {
		t.Error("dotenv-audit body should use grep to scan for hardcoded secrets")
	}
}

func TestDotenvAuditSkill_HasEnvExampleCheck(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["dotenv-audit"]
	if !strings.Contains(s.Body, ".env.example") {
		t.Error("dotenv-audit body should check for .env.example file")
	}
}

func TestDotenvAuditSkill_HasOsGetenvCheck(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["dotenv-audit"]
	if !strings.Contains(s.Body, "os.Getenv") {
		t.Error("dotenv-audit body should audit os.Getenv calls")
	}
}

func TestNpmAuditSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["npm-audit"]
	if !ok {
		t.Fatal("npm-audit skill not found; expected SKILL.md in skills/npm-audit/")
	}
	if s.Name != "npm-audit" {
		t.Errorf("Name = %q, want %q", s.Name, "npm-audit")
	}
	if s.Description == "" {
		t.Error("Description must not be empty")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Body == "" {
		t.Error("Body must not be empty")
	}
}

func TestNpmAuditSkill_HasAuditCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-audit"]
	if !strings.Contains(s.Body, "npm audit") {
		t.Error("npm-audit body should include 'npm audit' command")
	}
	if !strings.Contains(s.Body, "--json") {
		t.Error("npm-audit body should document JSON output flag")
	}
}

func TestNpmAuditSkill_HasFixCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-audit"]
	if !strings.Contains(s.Body, "npm audit fix") {
		t.Error("npm-audit body should include 'npm audit fix' command")
	}
	if !strings.Contains(s.Body, "--force") {
		t.Error("npm-audit body should document --force flag for audit fix")
	}
}

func TestNpmAuditSkill_HasSeverityFilter(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-audit"]
	if !strings.Contains(s.Body, "--audit-level") {
		t.Error("npm-audit body should document --audit-level flag")
	}
}

func TestNpmAuditSkill_HasSeverityLevels(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-audit"]
	if !strings.Contains(s.Body, "critical") {
		t.Error("npm-audit body should document 'critical' severity level")
	}
	// "high" may appear in any case
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "high") {
		t.Error("npm-audit body should document 'high' severity level")
	}
}

// --- Cross-cutting validation ---

// TestAllSkillsHaveVersion ensures every SKILL.md in the skills/ directory
// has version: 1 as required by the loader.
func TestAllSkillsHaveVersion(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	for name, s := range all {
		if s.Version != 1 {
			t.Errorf("skill %q: Version = %d, want 1", name, s.Version)
		}
	}
}

// TestAllSkillsHaveDescription ensures every skill has a non-empty description.
func TestAllSkillsHaveDescription(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	for name, s := range all {
		if s.Description == "" {
			t.Errorf("skill %q: Description must not be empty", name)
		}
	}
}

// TestAllSkillsHaveBody ensures every skill has non-empty markdown body content.
func TestAllSkillsHaveBody(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	for name, s := range all {
		if s.Body == "" {
			t.Errorf("skill %q: Body (markdown content) must not be empty", name)
		}
	}
}

// TestExpectedSkillCount validates that all 10 skills for issues #59, #61, #63
// are present. This catches accidentally deleted or renamed skill directories.
func TestExpectedSkillCount(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	expected := []string{
		// Issue #59: Git Workflow
		"git-branching",
		"git-pr-create",
		"git-pr-review",
		"git-merge-strategy",
		// Issue #61: CI/CD Management
		"github-actions",
		"ci-debug",
		// Issue #63: Security Scanning
		"gosec",
		"go-vuln-check",
		"dotenv-audit",
		"npm-audit",
	}
	for _, name := range expected {
		if _, ok := all[name]; !ok {
			t.Errorf("expected skill %q not found", name)
		}
	}
}

// TestAllSkillsDefaultToConversationContext ensures none of the new skills
// accidentally use the fork context, which requires a runner.
func TestAllSkillsDefaultToConversationContext(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	for name, s := range all {
		if s.Context == skills.ContextFork {
			t.Errorf("skill %q uses context=fork; these skills should use conversation context", name)
		}
	}
}

// TestAllSkillsHaveAllowedTools ensures each skill declares allowed-tools
// to constrain the tool set available during execution.
func TestAllSkillsHaveAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	for name, s := range all {
		if len(s.AllowedTools) == 0 {
			t.Errorf("skill %q: allowed-tools should be declared to constrain execution scope", name)
		}
	}
}
