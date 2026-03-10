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

// --- Issue #65: Database Management Skills ---

func TestPostgresOpsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["postgres-ops"]
	if !ok {
		t.Fatal("postgres-ops skill not found; expected SKILL.md in skills/postgres-ops/")
	}
	if s.Name != "postgres-ops" {
		t.Errorf("Name = %q, want %q", s.Name, "postgres-ops")
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

func TestPostgresOpsSkill_HasDockerCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["postgres-ops"]
	if !strings.Contains(s.Body, "docker run") {
		t.Error("postgres-ops body should document docker run command to start PostgreSQL")
	}
	if !strings.Contains(s.Body, "psql") {
		t.Error("postgres-ops body should document psql connection command")
	}
}

func TestPostgresOpsSkill_HasConnectionString(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["postgres-ops"]
	if !strings.Contains(s.Body, "postgresql://") {
		t.Error("postgres-ops body should document postgresql:// connection string format")
	}
}

func TestPostgresOpsSkill_HasSchemaInspection(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["postgres-ops"]
	if !strings.Contains(s.Body, `\dt`) && !strings.Contains(s.Body, `\\dt`) {
		t.Error("postgres-ops body should document \\dt for listing tables")
	}
}

func TestPostgresOpsSkill_HasBackupRestore(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["postgres-ops"]
	if !strings.Contains(s.Body, "pg_dump") {
		t.Error("postgres-ops body should document pg_dump for backups")
	}
	if !strings.Contains(s.Body, "pg_restore") {
		t.Error("postgres-ops body should document pg_restore for restores")
	}
}

func TestSQLiteOpsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["sqlite-ops"]
	if !ok {
		t.Fatal("sqlite-ops skill not found; expected SKILL.md in skills/sqlite-ops/")
	}
	if s.Name != "sqlite-ops" {
		t.Errorf("Name = %q, want %q", s.Name, "sqlite-ops")
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

func TestSQLiteOpsSkill_HasCLICommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sqlite-ops"]
	if !strings.Contains(s.Body, "sqlite3") {
		t.Error("sqlite-ops body should document sqlite3 CLI usage")
	}
	if !strings.Contains(s.Body, ".tables") {
		t.Error("sqlite-ops body should document .tables command")
	}
	if !strings.Contains(s.Body, ".schema") {
		t.Error("sqlite-ops body should document .schema command")
	}
}

func TestSQLiteOpsSkill_HasWALMode(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sqlite-ops"]
	if !strings.Contains(s.Body, "WAL") {
		t.Error("sqlite-ops body should document WAL mode for concurrent access")
	}
	if !strings.Contains(s.Body, "journal_mode") {
		t.Error("sqlite-ops body should document journal_mode pragma")
	}
}

func TestSQLiteOpsSkill_HasDumpRestore(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sqlite-ops"]
	if !strings.Contains(s.Body, ".dump") {
		t.Error("sqlite-ops body should document .dump for backups")
	}
}

func TestSQLiteOpsSkill_HasGoUsage(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sqlite-ops"]
	if !strings.Contains(s.Body, "modernc.org/sqlite") {
		t.Error("sqlite-ops body should document modernc.org/sqlite Go driver")
	}
	if !strings.Contains(s.Body, ":memory:") {
		t.Error("sqlite-ops body should document :memory: database for testing")
	}
}

func TestDBMigrationsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["db-migrations"]
	if !ok {
		t.Fatal("db-migrations skill not found; expected SKILL.md in skills/db-migrations/")
	}
	if s.Name != "db-migrations" {
		t.Errorf("Name = %q, want %q", s.Name, "db-migrations")
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

func TestDBMigrationsSkill_HasGooseCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["db-migrations"]
	if !strings.Contains(s.Body, "goose") {
		t.Error("db-migrations body should document goose migration tool")
	}
	if !strings.Contains(s.Body, "goose") && !strings.Contains(s.Body, "up") {
		t.Error("db-migrations body should document goose up command")
	}
	if !strings.Contains(s.Body, "down") {
		t.Error("db-migrations body should document rollback (down) command")
	}
	if !strings.Contains(s.Body, "status") {
		t.Error("db-migrations body should document migration status command")
	}
}

func TestDBMigrationsSkill_HasMigrationFileFormat(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["db-migrations"]
	if !strings.Contains(s.Body, "+goose Up") {
		t.Error("db-migrations body should document +goose Up annotation in SQL files")
	}
	if !strings.Contains(s.Body, "+goose Down") {
		t.Error("db-migrations body should document +goose Down annotation in SQL files")
	}
}

func TestDBMigrationsSkill_HasInstallInstructions(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["db-migrations"]
	if !strings.Contains(s.Body, "go install") {
		t.Error("db-migrations body should include installation instructions")
	}
}

// --- Issue #66: Monitoring & Observability Skills ---

func TestHealthCheckSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["health-check"]
	if !ok {
		t.Fatal("health-check skill not found; expected SKILL.md in skills/health-check/")
	}
	if s.Name != "health-check" {
		t.Errorf("Name = %q, want %q", s.Name, "health-check")
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

func TestHealthCheckSkill_HasCurlCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["health-check"]
	if !strings.Contains(s.Body, "curl") {
		t.Error("health-check body should document curl command for health checks")
	}
	if !strings.Contains(s.Body, "http_code") {
		t.Error("health-check body should document HTTP status code extraction with curl")
	}
}

func TestHealthCheckSkill_HasResponseTimeMeasurement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["health-check"]
	if !strings.Contains(s.Body, "time_total") {
		t.Error("health-check body should document response time measurement with curl")
	}
}

func TestHealthCheckSkill_HasKubernetesProbes(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["health-check"]
	if !strings.Contains(s.Body, "livenessProbe") && !strings.Contains(s.Body, "readinessProbe") {
		t.Error("health-check body should document Kubernetes liveness/readiness probe patterns")
	}
}

func TestHealthCheckSkill_HasStatusCodeInterpretation(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["health-check"]
	if !strings.Contains(s.Body, "503") {
		t.Error("health-check body should document 503 Service Unavailable status code")
	}
}

func TestGoProfilingSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["go-profiling"]
	if !ok {
		t.Fatal("go-profiling skill not found; expected SKILL.md in skills/go-profiling/")
	}
	if s.Name != "go-profiling" {
		t.Errorf("Name = %q, want %q", s.Name, "go-profiling")
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

func TestGoProfilingSkill_HasPprofProfiles(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-profiling"]
	if !strings.Contains(s.Body, "heap") {
		t.Error("go-profiling body should document heap profile")
	}
	if !strings.Contains(s.Body, "goroutine") {
		t.Error("go-profiling body should document goroutine profile")
	}
	if !strings.Contains(s.Body, "profile") {
		t.Error("go-profiling body should document CPU profile")
	}
}

func TestGoProfilingSkill_HasPprofURLs(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-profiling"]
	if !strings.Contains(s.Body, "debug/pprof") {
		t.Error("go-profiling body should document pprof HTTP endpoint paths")
	}
	if !strings.Contains(s.Body, "go tool pprof") {
		t.Error("go-profiling body should document 'go tool pprof' command")
	}
}

func TestGoProfilingSkill_HasImportInstructions(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-profiling"]
	if !strings.Contains(s.Body, "net/http/pprof") {
		t.Error("go-profiling body should document net/http/pprof import")
	}
}

func TestGoProfilingSkill_HasProfileTypes(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-profiling"]
	if !strings.Contains(s.Body, "block") {
		t.Error("go-profiling body should document block profile type")
	}
	if !strings.Contains(s.Body, "mutex") {
		t.Error("go-profiling body should document mutex profile type")
	}
}

func TestLogAnalysisSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["log-analysis"]
	if !ok {
		t.Fatal("log-analysis skill not found; expected SKILL.md in skills/log-analysis/")
	}
	if s.Name != "log-analysis" {
		t.Errorf("Name = %q, want %q", s.Name, "log-analysis")
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

func TestLogAnalysisSkill_HasGrepPatterns(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["log-analysis"]
	if !strings.Contains(s.Body, "grep") {
		t.Error("log-analysis body should document grep for log searching")
	}
}

func TestLogAnalysisSkill_HasJQPatterns(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["log-analysis"]
	if !strings.Contains(s.Body, "jq") {
		t.Error("log-analysis body should document jq for JSON log processing")
	}
	if !strings.Contains(s.Body, "select(") {
		t.Error("log-analysis body should document jq select() filter for log filtering")
	}
}

func TestLogAnalysisSkill_HasErrorFrequencyAnalysis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["log-analysis"]
	if !strings.Contains(s.Body, "uniq -c") {
		t.Error("log-analysis body should document uniq -c for frequency counting")
	}
}

func TestLogAnalysisSkill_HasTailCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["log-analysis"]
	if !strings.Contains(s.Body, "tail -f") {
		t.Error("log-analysis body should document tail -f for live log monitoring")
	}
}

// --- Issue #68: Testing & QA Skills ---

func TestAPITestingSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["api-testing"]
	if !ok {
		t.Fatal("api-testing skill not found; expected SKILL.md in skills/api-testing/")
	}
	if s.Name != "api-testing" {
		t.Errorf("Name = %q, want %q", s.Name, "api-testing")
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

func TestAPITestingSkill_HasHTTPMethods(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["api-testing"]
	if !strings.Contains(s.Body, "-X POST") {
		t.Error("api-testing body should document POST request pattern")
	}
	if !strings.Contains(s.Body, "-X PUT") {
		t.Error("api-testing body should document PUT request pattern")
	}
	if !strings.Contains(s.Body, "-X DELETE") {
		t.Error("api-testing body should document DELETE request pattern")
	}
}

func TestAPITestingSkill_HasStatusCodeValidation(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["api-testing"]
	if !strings.Contains(s.Body, "http_code") {
		t.Error("api-testing body should document HTTP status code extraction")
	}
}

func TestAPITestingSkill_HasJQValidation(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["api-testing"]
	if !strings.Contains(s.Body, "jq") {
		t.Error("api-testing body should document jq for JSON response validation")
	}
}

func TestAPITestingSkill_HasAuthPattern(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["api-testing"]
	if !strings.Contains(s.Body, "Authorization: Bearer") {
		t.Error("api-testing body should document Bearer token authentication pattern")
	}
}

func TestAPITestingSkill_HasResponseTimeMeasurement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["api-testing"]
	if !strings.Contains(s.Body, "time_total") {
		t.Error("api-testing body should document response time measurement")
	}
}

func TestAPITestingSkill_HasSecurityNote(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["api-testing"]
	// Must warn against logging tokens
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "token") || (!strings.Contains(bodyLower, "never log") && !strings.Contains(bodyLower, "do not") && !strings.Contains(bodyLower, "never")) {
		t.Error("api-testing body should warn against logging sensitive auth tokens")
	}
}

func TestE2ETestingSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["e2e-testing"]
	if !ok {
		t.Fatal("e2e-testing skill not found; expected SKILL.md in skills/e2e-testing/")
	}
	if s.Name != "e2e-testing" {
		t.Errorf("Name = %q, want %q", s.Name, "e2e-testing")
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

func TestE2ETestingSkill_HasPlaywrightCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["e2e-testing"]
	if !strings.Contains(s.Body, "npx playwright test") {
		t.Error("e2e-testing body should document 'npx playwright test' command")
	}
	if !strings.Contains(s.Body, "--headed") {
		t.Error("e2e-testing body should document --headed flag for visible browser")
	}
	if !strings.Contains(s.Body, "--grep") {
		t.Error("e2e-testing body should document --grep flag for test filtering")
	}
}

func TestE2ETestingSkill_HasBrowserProjects(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["e2e-testing"]
	if !strings.Contains(s.Body, "--project") {
		t.Error("e2e-testing body should document --project flag for browser selection")
	}
	if !strings.Contains(s.Body, "chromium") {
		t.Error("e2e-testing body should document chromium browser project")
	}
}

func TestE2ETestingSkill_HasReporterFlag(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["e2e-testing"]
	if !strings.Contains(s.Body, "--reporter") {
		t.Error("e2e-testing body should document --reporter flag for output formats")
	}
	if !strings.Contains(s.Body, "html") {
		t.Error("e2e-testing body should document HTML reporter")
	}
}

func TestE2ETestingSkill_HasTraceCapture(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["e2e-testing"]
	if !strings.Contains(s.Body, "--trace") {
		t.Error("e2e-testing body should document --trace flag for capturing traces")
	}
}

func TestE2ETestingSkill_HasCodegen(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["e2e-testing"]
	if !strings.Contains(s.Body, "codegen") {
		t.Error("e2e-testing body should document playwright codegen for recording tests")
	}
}

// --- Issue #69: Collaboration Skills ---

func TestGitHubIssuesSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["github-issues"]
	if !ok {
		t.Fatal("github-issues skill not found; expected SKILL.md in skills/github-issues/")
	}
	if s.Name != "github-issues" {
		t.Errorf("Name = %q, want %q", s.Name, "github-issues")
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

func TestGitHubIssuesSkill_HasCreateCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["github-issues"]
	if !strings.Contains(s.Body, "gh issue create") {
		t.Error("github-issues body should document 'gh issue create' command")
	}
	if !strings.Contains(s.Body, "--title") {
		t.Error("github-issues body should document --title flag")
	}
	if !strings.Contains(s.Body, "--label") {
		t.Error("github-issues body should document --label flag")
	}
}

func TestGitHubIssuesSkill_HasListCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["github-issues"]
	if !strings.Contains(s.Body, "gh issue list") {
		t.Error("github-issues body should document 'gh issue list' command")
	}
	if !strings.Contains(s.Body, "--json") {
		t.Error("github-issues body should document JSON output for issue listing")
	}
}

func TestGitHubIssuesSkill_HasCloseCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["github-issues"]
	if !strings.Contains(s.Body, "gh issue close") {
		t.Error("github-issues body should document 'gh issue close' command")
	}
}

func TestGitHubIssuesSkill_HasCommentCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["github-issues"]
	if !strings.Contains(s.Body, "gh issue comment") {
		t.Error("github-issues body should document 'gh issue comment' command")
	}
}

func TestGitHubIssuesSkill_HasPRLinking(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["github-issues"]
	if !strings.Contains(s.Body, "Closes #") {
		t.Error("github-issues body should document 'Closes #N' pattern for linking PRs to issues")
	}
}

func TestDocGenerationSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["doc-generation"]
	if !ok {
		t.Fatal("doc-generation skill not found; expected SKILL.md in skills/doc-generation/")
	}
	if s.Name != "doc-generation" {
		t.Errorf("Name = %q, want %q", s.Name, "doc-generation")
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

func TestDocGenerationSkill_HasGoDocCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["doc-generation"]
	if !strings.Contains(s.Body, "go doc") {
		t.Error("doc-generation body should document 'go doc' command")
	}
	if !strings.Contains(s.Body, "-all") {
		t.Error("doc-generation body should document -all flag for full package docs")
	}
}

func TestDocGenerationSkill_HasSwaggoInstructions(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["doc-generation"]
	if !strings.Contains(s.Body, "swag") {
		t.Error("doc-generation body should document swag/swaggo for OpenAPI generation")
	}
	if !strings.Contains(s.Body, "swag init") {
		t.Error("doc-generation body should document 'swag init' command")
	}
}

func TestDocGenerationSkill_HasGodocCommentFormat(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["doc-generation"]
	// Should document the godoc comment style
	if !strings.Contains(s.Body, "// Package") && !strings.Contains(s.Body, "Package ") {
		t.Error("doc-generation body should document godoc package comment format")
	}
}

func TestDocGenerationSkill_HasVetCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["doc-generation"]
	if !strings.Contains(s.Body, "go vet") {
		t.Error("doc-generation body should document 'go vet' for checking doc format issues")
	}
}

// TestExpectedSkillsForIssues65_66_68_69 validates that all 10 new skills
// for issues #65, #66, #68, and #69 are present.
func TestExpectedSkillsForIssues65_66_68_69(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	expected := []string{
		// Issue #65: Database Management
		"postgres-ops",
		"sqlite-ops",
		"db-migrations",
		// Issue #66: Monitoring & Observability
		"health-check",
		"go-profiling",
		"log-analysis",
		// Issue #68: Testing & QA
		"api-testing",
		"e2e-testing",
		// Issue #69: Collaboration
		"github-issues",
		"doc-generation",
	}
	for _, name := range expected {
		if _, ok := all[name]; !ok {
			t.Errorf("expected skill %q not found (required for issues #65/#66/#68/#69)", name)
		}
	}
}

// --- Issue #46: Cloudflare Workers Deployment Skill ---

func TestCloudflareWorkersSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["cloudflare-workers"]
	if !ok {
		t.Fatal("cloudflare-workers skill not found; expected SKILL.md in skills/cloudflare-workers/")
	}
	if s.Name != "cloudflare-workers" {
		t.Errorf("Name = %q, want %q", s.Name, "cloudflare-workers")
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

func TestCloudflareWorkersSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if len(s.Triggers) == 0 {
		t.Error("cloudflare-workers should have at least one trigger phrase in description")
	}
}

func TestCloudflareWorkersSkill_HasWranglerDeploy(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if !strings.Contains(s.Body, "wrangler deploy") {
		t.Error("cloudflare-workers body should include 'wrangler deploy' command")
	}
	if !strings.Contains(s.Body, "wrangler dev") {
		t.Error("cloudflare-workers body should include 'wrangler dev' command")
	}
	if !strings.Contains(s.Body, "wrangler tail") {
		t.Error("cloudflare-workers body should include 'wrangler tail' for log monitoring")
	}
}

func TestCloudflareWorkersSkill_HasRollback(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if !strings.Contains(s.Body, "wrangler rollback") {
		t.Error("cloudflare-workers body should include 'wrangler rollback' command")
	}
}

func TestCloudflareWorkersSkill_HasStorageBindings(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if !strings.Contains(s.Body, "KV") {
		t.Error("cloudflare-workers body should document KV namespace binding")
	}
	if !strings.Contains(s.Body, "R2") {
		t.Error("cloudflare-workers body should document R2 bucket binding")
	}
	if !strings.Contains(s.Body, "D1") {
		t.Error("cloudflare-workers body should document D1 database binding")
	}
}

func TestCloudflareWorkersSkill_HasSecretManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if !strings.Contains(s.Body, "wrangler secret put") {
		t.Error("cloudflare-workers body should include 'wrangler secret put' command")
	}
}

func TestCloudflareWorkersSkill_HasMultiEnvironment(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if !strings.Contains(s.Body, "--env") {
		t.Error("cloudflare-workers body should document --env flag for multi-environment deployments")
	}
}

func TestCloudflareWorkersSkill_HasSafetyRules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if !strings.Contains(s.Body, "Safety") {
		t.Error("cloudflare-workers body should include safety rules section")
	}
}

func TestCloudflareWorkersSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cloudflare-workers"]
	if len(s.AllowedTools) == 0 {
		t.Error("cloudflare-workers should declare allowed-tools")
	}
}

// --- Issue #48: Vercel Deployment Skill ---

func TestVercelDeploySkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["vercel-deploy"]
	if !ok {
		t.Fatal("vercel-deploy skill not found; expected SKILL.md in skills/vercel-deploy/")
	}
	if s.Name != "vercel-deploy" {
		t.Errorf("Name = %q, want %q", s.Name, "vercel-deploy")
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

func TestVercelDeploySkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if len(s.Triggers) == 0 {
		t.Error("vercel-deploy should have at least one trigger phrase in description")
	}
}

func TestVercelDeploySkill_HasPreviewDeploy(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if !strings.Contains(s.Body, "vercel --yes") {
		t.Error("vercel-deploy body should include 'vercel --yes' for non-interactive preview deploy")
	}
}

func TestVercelDeploySkill_HasProductionDeploy(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if !strings.Contains(s.Body, "--prod") {
		t.Error("vercel-deploy body should include --prod flag for production deployment")
	}
}

func TestVercelDeploySkill_HasEnvManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if !strings.Contains(s.Body, "vercel env") {
		t.Error("vercel-deploy body should include 'vercel env' commands")
	}
	if !strings.Contains(s.Body, "vercel env pull") {
		t.Error("vercel-deploy body should include 'vercel env pull' for syncing env vars locally")
	}
}

func TestVercelDeploySkill_HasLogs(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if !strings.Contains(s.Body, "vercel logs") {
		t.Error("vercel-deploy body should include 'vercel logs' for deployment verification")
	}
}

func TestVercelDeploySkill_HasDomainManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if !strings.Contains(s.Body, "vercel domains") {
		t.Error("vercel-deploy body should include 'vercel domains' commands")
	}
}

func TestVercelDeploySkill_HasSafetyRules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if !strings.Contains(s.Body, "Safety") {
		t.Error("vercel-deploy body should include safety rules section")
	}
}

func TestVercelDeploySkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vercel-deploy"]
	if len(s.AllowedTools) == 0 {
		t.Error("vercel-deploy should declare allowed-tools")
	}
}

// --- Issue #54: Fly.io Deployment Skill ---

func TestFlyDeploySkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["fly-deploy"]
	if !ok {
		t.Fatal("fly-deploy skill not found; expected SKILL.md in skills/fly-deploy/")
	}
	if s.Name != "fly-deploy" {
		t.Errorf("Name = %q, want %q", s.Name, "fly-deploy")
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

func TestFlyDeploySkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if len(s.Triggers) == 0 {
		t.Error("fly-deploy should have at least one trigger phrase in description")
	}
}

func TestFlyDeploySkill_HasLaunchAndDeploy(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "fly launch") {
		t.Error("fly-deploy body should include 'fly launch' command")
	}
	if !strings.Contains(s.Body, "fly deploy") {
		t.Error("fly-deploy body should include 'fly deploy' command")
	}
}

func TestFlyDeploySkill_HasStatusAndLogs(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "fly status") {
		t.Error("fly-deploy body should include 'fly status' for health verification")
	}
	if !strings.Contains(s.Body, "fly logs") {
		t.Error("fly-deploy body should include 'fly logs' command")
	}
}

func TestFlyDeploySkill_HasScaling(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "fly scale") {
		t.Error("fly-deploy body should include 'fly scale' commands")
	}
}

func TestFlyDeploySkill_HasMultiRegion(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "fly regions") {
		t.Error("fly-deploy body should include 'fly regions' commands for multi-region deployment")
	}
}

func TestFlyDeploySkill_HasSecretManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "fly secrets set") {
		t.Error("fly-deploy body should include 'fly secrets set' command")
	}
}

func TestFlyDeploySkill_HasSSHAccess(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "fly ssh") {
		t.Error("fly-deploy body should include 'fly ssh' for debugging access")
	}
}

func TestFlyDeploySkill_HasManagedPostgres(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "fly postgres") {
		t.Error("fly-deploy body should include 'fly postgres' commands")
	}
	if !strings.Contains(s.Body, "fly postgres attach") {
		t.Error("fly-deploy body should include 'fly postgres attach' for connecting DB to app")
	}
}

func TestFlyDeploySkill_HasSafetyRules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if !strings.Contains(s.Body, "Safety") {
		t.Error("fly-deploy body should include safety rules section")
	}
}

func TestFlyDeploySkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["fly-deploy"]
	if len(s.AllowedTools) == 0 {
		t.Error("fly-deploy should declare allowed-tools")
	}
}

// --- Issue #71: Kubernetes Operations Skill (kubectl-ops) ---

func TestKubectlOpsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["kubectl-ops"]
	if !ok {
		t.Fatal("kubectl-ops skill not found; expected SKILL.md in skills/kubectl-ops/")
	}
	if s.Name != "kubectl-ops" {
		t.Errorf("Name = %q, want %q", s.Name, "kubectl-ops")
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

func TestKubectlOpsSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if len(s.Triggers) == 0 {
		t.Error("kubectl-ops should have at least one trigger phrase in description")
	}
}

func TestKubectlOpsSkill_HasApplyAndGet(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if !strings.Contains(s.Body, "kubectl apply") {
		t.Error("kubectl-ops body should include 'kubectl apply' command")
	}
	if !strings.Contains(s.Body, "kubectl get") {
		t.Error("kubectl-ops body should include 'kubectl get' command")
	}
}

func TestKubectlOpsSkill_HasRollout(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if !strings.Contains(s.Body, "kubectl rollout status") {
		t.Error("kubectl-ops body should include 'kubectl rollout status' command")
	}
	if !strings.Contains(s.Body, "kubectl rollout undo") {
		t.Error("kubectl-ops body should include 'kubectl rollout undo' for rollbacks")
	}
}

func TestKubectlOpsSkill_HasScaling(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if !strings.Contains(s.Body, "kubectl scale") {
		t.Error("kubectl-ops body should include 'kubectl scale' command")
	}
}

func TestKubectlOpsSkill_HasPortForward(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if !strings.Contains(s.Body, "kubectl port-forward") {
		t.Error("kubectl-ops body should include 'kubectl port-forward' command")
	}
}

func TestKubectlOpsSkill_HasSecretsManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if !strings.Contains(s.Body, "kubectl create secret") {
		t.Error("kubectl-ops body should include 'kubectl create secret' command")
	}
}

func TestKubectlOpsSkill_HasContextManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if !strings.Contains(s.Body, "kubectl config use-context") {
		t.Error("kubectl-ops body should include context switching command")
	}
}

func TestKubectlOpsSkill_HasSafetyRules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if !strings.Contains(s.Body, "Safety") {
		t.Error("kubectl-ops body should include safety rules section")
	}
}

func TestKubectlOpsSkill_SecretsNeverLogged(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	// Must warn about not exposing secret values
	if !strings.Contains(s.Body, "never") && !strings.Contains(s.Body, "Never") {
		t.Error("kubectl-ops body should warn about never exposing secret values")
	}
}

func TestKubectlOpsSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["kubectl-ops"]
	if len(s.AllowedTools) == 0 {
		t.Error("kubectl-ops should declare allowed-tools")
	}
}

// --- Issue #71: Kubernetes Debugging Skill (k8s-debug) ---

func TestK8sDebugSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["k8s-debug"]
	if !ok {
		t.Fatal("k8s-debug skill not found; expected SKILL.md in skills/k8s-debug/")
	}
	if s.Name != "k8s-debug" {
		t.Errorf("Name = %q, want %q", s.Name, "k8s-debug")
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

func TestK8sDebugSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if len(s.Triggers) == 0 {
		t.Error("k8s-debug should have at least one trigger phrase in description")
	}
}

func TestK8sDebugSkill_HasCrashLoopDiagnosis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "CrashLoopBackOff") {
		t.Error("k8s-debug body should cover CrashLoopBackOff diagnosis")
	}
	if !strings.Contains(s.Body, "--previous") {
		t.Error("k8s-debug body should use kubectl logs --previous for crash diagnosis")
	}
}

func TestK8sDebugSkill_HasOOMKilledDiagnosis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "OOMKilled") {
		t.Error("k8s-debug body should cover OOMKilled diagnosis")
	}
}

func TestK8sDebugSkill_HasImagePullDiagnosis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "ImagePullBackOff") {
		t.Error("k8s-debug body should cover ImagePullBackOff diagnosis")
	}
}

func TestK8sDebugSkill_HasPendingPodDiagnosis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "Pending") {
		t.Error("k8s-debug body should cover Pending pod diagnosis")
	}
}

func TestK8sDebugSkill_HasNetworkingDiagnosis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "nslookup") || !strings.Contains(s.Body, "curl") {
		t.Error("k8s-debug body should include networking diagnosis with nslookup or curl")
	}
}

func TestK8sDebugSkill_HasDebugWorkflow(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "kubectl describe pod") {
		t.Error("k8s-debug body should include 'kubectl describe pod' in debug workflow")
	}
	if !strings.Contains(s.Body, "kubectl logs") {
		t.Error("k8s-debug body should include 'kubectl logs' in debug workflow")
	}
}

func TestK8sDebugSkill_HasEventInspection(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "kubectl get events") {
		t.Error("k8s-debug body should include 'kubectl get events' for cluster-level diagnostics")
	}
}

func TestK8sDebugSkill_HasSafetyRules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if !strings.Contains(s.Body, "Safety") {
		t.Error("k8s-debug body should include safety rules section")
	}
}

func TestK8sDebugSkill_SecretsNeverExposed(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	// Must warn about not exposing secret values
	if !strings.Contains(s.Body, "never") && !strings.Contains(s.Body, "Never") {
		t.Error("k8s-debug body should warn about never exposing secret values")
	}
}

func TestK8sDebugSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["k8s-debug"]
	if len(s.AllowedTools) == 0 {
		t.Error("k8s-debug should declare allowed-tools")
	}
}

// --- Cross-cutting: Issues #46, #48, #54, #71 ---

// TestExpectedSkillsForIssues46_48_54_71 validates that all 5 new skills
// for issues #46, #48, #54, and #71 are present.
func TestExpectedSkillsForIssues46_48_54_71(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	expected := []string{
		// Issue #46: Cloudflare Workers
		"cloudflare-workers",
		// Issue #48: Vercel Deployment
		"vercel-deploy",
		// Issue #54: Fly.io Deployment
		"fly-deploy",
		// Issue #71: Kubernetes Operations
		"kubectl-ops",
		"k8s-debug",
	}
	for _, name := range expected {
		if _, ok := all[name]; !ok {
			t.Errorf("expected skill %q not found (required for issues #46/#48/#54/#71)", name)
		}
	}
}
