// Package skills_validation tests that the bundled SKILL.md files in this
// directory parse correctly and satisfy required field constraints.
//
// This file covers skills added in GitHub Issues #65 (Database Management),
// #66 (Monitoring & Observability), #68 (Testing & QA), and #69 (Collaboration).
package skills_validation

import (
	"strings"
	"testing"
)

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
