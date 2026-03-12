// Package skills_validation tests that the bundled SKILL.md files in this
// directory parse correctly and satisfy required field constraints.
//
// This file covers skills added in GitHub Issues #83 (Security), #84 (Testing),
// and #85 (Database).
package skills_validation

import (
	"strings"
	"testing"

	"go-agent-harness/internal/skills"
)

// --- Issue #83: Security Skills ---

func TestVaultOpsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["vault-ops"]
	if !ok {
		t.Fatal("vault-ops skill not found; expected SKILL.md in skills/vault-ops/")
	}
	if s.Name != "vault-ops" {
		t.Errorf("Name = %q, want %q", s.Name, "vault-ops")
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

func TestVaultOpsSkill_HasReadWriteCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vault-ops"]
	if !strings.Contains(s.Body, "vault kv get") {
		t.Error("vault-ops body should include 'vault kv get' command")
	}
	if !strings.Contains(s.Body, "vault kv put") {
		t.Error("vault-ops body should include 'vault kv put' command")
	}
}

func TestVaultOpsSkill_HasDynamicCredentials(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vault-ops"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "dynamic") {
		t.Error("vault-ops body should document dynamic credentials")
	}
	if !strings.Contains(s.Body, "vault read database/creds") {
		t.Error("vault-ops body should show how to read dynamic database credentials")
	}
}

func TestVaultOpsSkill_HasPolicyManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vault-ops"]
	if !strings.Contains(s.Body, "vault policy") {
		t.Error("vault-ops body should include 'vault policy' commands")
	}
	if !strings.Contains(s.Body, "vault policy write") {
		t.Error("vault-ops body should include 'vault policy write' command")
	}
}

func TestVaultOpsSkill_HasTokenRenewal(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vault-ops"]
	if !strings.Contains(s.Body, "vault token renew") {
		t.Error("vault-ops body should document token renewal")
	}
}

func TestVaultOpsSkill_HasAuthMethods(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vault-ops"]
	if !strings.Contains(s.Body, "vault login") {
		t.Error("vault-ops body should document authentication with 'vault login'")
	}
}

func TestVaultOpsSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vault-ops"]
	if len(s.Triggers) == 0 {
		t.Error("vault-ops should have at least one trigger phrase in description")
	}
}

func TestVaultOpsSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vault-ops"]
	if len(s.AllowedTools) == 0 {
		t.Error("vault-ops should declare allowed-tools")
	}
}

func TestSnykScanSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["snyk-scan"]
	if !ok {
		t.Fatal("snyk-scan skill not found; expected SKILL.md in skills/snyk-scan/")
	}
	if s.Name != "snyk-scan" {
		t.Errorf("Name = %q, want %q", s.Name, "snyk-scan")
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

func TestSnykScanSkill_HasTestCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	if !strings.Contains(s.Body, "snyk test") {
		t.Error("snyk-scan body should include 'snyk test' command")
	}
}

func TestSnykScanSkill_HasMonitorCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	if !strings.Contains(s.Body, "snyk monitor") {
		t.Error("snyk-scan body should include 'snyk monitor' command")
	}
}

func TestSnykScanSkill_HasCodeTestCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	if !strings.Contains(s.Body, "snyk code test") {
		t.Error("snyk-scan body should include 'snyk code test' for SAST scanning")
	}
}

func TestSnykScanSkill_HasSeverityThresholds(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	if !strings.Contains(s.Body, "--severity-threshold") {
		t.Error("snyk-scan body should document --severity-threshold flag")
	}
}

func TestSnykScanSkill_HasCIIntegration(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "github actions") && !strings.Contains(bodyLower, "ci") {
		t.Error("snyk-scan body should document CI integration")
	}
}

func TestSnykScanSkill_HasFixGuidance(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	if !strings.Contains(s.Body, "snyk fix") {
		t.Error("snyk-scan body should document 'snyk fix' command")
	}
}

func TestSnykScanSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	if len(s.Triggers) == 0 {
		t.Error("snyk-scan should have at least one trigger phrase in description")
	}
}

func TestSnykScanSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["snyk-scan"]
	if len(s.AllowedTools) == 0 {
		t.Error("snyk-scan should declare allowed-tools")
	}
}

func TestTrivyScanSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["trivy-scan"]
	if !ok {
		t.Fatal("trivy-scan skill not found; expected SKILL.md in skills/trivy-scan/")
	}
	if s.Name != "trivy-scan" {
		t.Errorf("Name = %q, want %q", s.Name, "trivy-scan")
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

func TestTrivyScanSkill_HasImageScan(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if !strings.Contains(s.Body, "trivy image") {
		t.Error("trivy-scan body should include 'trivy image' command")
	}
}

func TestTrivyScanSkill_HasFsScan(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if !strings.Contains(s.Body, "trivy fs") {
		t.Error("trivy-scan body should include 'trivy fs' command")
	}
}

func TestTrivyScanSkill_HasRepoScan(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if !strings.Contains(s.Body, "trivy repo") {
		t.Error("trivy-scan body should include 'trivy repo' command")
	}
}

func TestTrivyScanSkill_HasSeverityFiltering(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if !strings.Contains(s.Body, "--severity") {
		t.Error("trivy-scan body should document --severity filtering flag")
	}
	if !strings.Contains(s.Body, "HIGH,CRITICAL") {
		t.Error("trivy-scan body should show HIGH,CRITICAL severity example")
	}
}

func TestTrivyScanSkill_HasSARIFOutput(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if !strings.Contains(s.Body, "sarif") && !strings.Contains(s.Body, "SARIF") {
		t.Error("trivy-scan body should document SARIF output format")
	}
}

func TestTrivyScanSkill_HasCIIntegration(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if !strings.Contains(s.Body, "--exit-code 1") {
		t.Error("trivy-scan body should document --exit-code 1 for CI integration")
	}
}

func TestTrivyScanSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if len(s.Triggers) == 0 {
		t.Error("trivy-scan should have at least one trigger phrase in description")
	}
}

func TestTrivyScanSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["trivy-scan"]
	if len(s.AllowedTools) == 0 {
		t.Error("trivy-scan should declare allowed-tools")
	}
}

// --- Issue #84: Testing Skills ---

func TestCypressTestingSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["cypress-testing"]
	if !ok {
		t.Fatal("cypress-testing skill not found; expected SKILL.md in skills/cypress-testing/")
	}
	if s.Name != "cypress-testing" {
		t.Errorf("Name = %q, want %q", s.Name, "cypress-testing")
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

func TestCypressTestingSkill_HasOpenAndRunCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cypress-testing"]
	if !strings.Contains(s.Body, "cypress open") {
		t.Error("cypress-testing body should include 'cypress open' command")
	}
	if !strings.Contains(s.Body, "cypress run") {
		t.Error("cypress-testing body should include 'cypress run' command")
	}
}

func TestCypressTestingSkill_HasComponentTesting(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cypress-testing"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "component") {
		t.Error("cypress-testing body should document component testing")
	}
}

func TestCypressTestingSkill_HasNetworkIntercepting(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cypress-testing"]
	if !strings.Contains(s.Body, "cy.intercept") {
		t.Error("cypress-testing body should document cy.intercept for network interception")
	}
}

func TestCypressTestingSkill_HasCustomCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cypress-testing"]
	if !strings.Contains(s.Body, "Cypress.Commands.add") {
		t.Error("cypress-testing body should document Cypress.Commands.add for custom commands")
	}
}

func TestCypressTestingSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cypress-testing"]
	if len(s.Triggers) == 0 {
		t.Error("cypress-testing should have at least one trigger phrase in description")
	}
}

func TestCypressTestingSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["cypress-testing"]
	if len(s.AllowedTools) == 0 {
		t.Error("cypress-testing should declare allowed-tools")
	}
}

func TestLoadTestingSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["load-testing"]
	if !ok {
		t.Fatal("load-testing skill not found; expected SKILL.md in skills/load-testing/")
	}
	if s.Name != "load-testing" {
		t.Errorf("Name = %q, want %q", s.Name, "load-testing")
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

func TestLoadTestingSkill_HasK6Content(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["load-testing"]
	if !strings.Contains(s.Body, "k6 run") {
		t.Error("load-testing body should include 'k6 run' command")
	}
}

func TestLoadTestingSkill_HasVegetaContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["load-testing"]
	if !strings.Contains(s.Body, "vegeta attack") {
		t.Error("load-testing body should include 'vegeta attack' command")
	}
}

func TestLoadTestingSkill_HasRampUpPatterns(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["load-testing"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "ramp") {
		t.Error("load-testing body should document ramp-up patterns")
	}
	if !strings.Contains(s.Body, "stages") {
		t.Error("load-testing body should document k6 stages for ramp-up")
	}
}

func TestLoadTestingSkill_HasThresholds(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["load-testing"]
	if !strings.Contains(s.Body, "thresholds") {
		t.Error("load-testing body should document threshold configuration")
	}
}

func TestLoadTestingSkill_HasResultsAnalysis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["load-testing"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "p(95)") && !strings.Contains(bodyLower, "p95") {
		t.Error("load-testing body should document p95 latency metric in results analysis")
	}
}

func TestLoadTestingSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["load-testing"]
	if len(s.Triggers) == 0 {
		t.Error("load-testing should have at least one trigger phrase in description")
	}
}

func TestLoadTestingSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["load-testing"]
	if len(s.AllowedTools) == 0 {
		t.Error("load-testing should declare allowed-tools")
	}
}

// --- Issue #85: Database Skills ---

func TestRedisOpsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["redis-ops"]
	if !ok {
		t.Fatal("redis-ops skill not found; expected SKILL.md in skills/redis-ops/")
	}
	if s.Name != "redis-ops" {
		t.Errorf("Name = %q, want %q", s.Name, "redis-ops")
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

func TestRedisOpsSkill_HasRedisCLI(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if !strings.Contains(s.Body, "redis-cli") {
		t.Error("redis-ops body should include 'redis-cli' commands")
	}
}

func TestRedisOpsSkill_HasKeyPatterns(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if !strings.Contains(s.Body, "SCAN") {
		t.Error("redis-ops body should document SCAN for safe key iteration")
	}
}

func TestRedisOpsSkill_HasPubSub(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if !strings.Contains(s.Body, "SUBSCRIBE") {
		t.Error("redis-ops body should document SUBSCRIBE for pub/sub")
	}
	if !strings.Contains(s.Body, "PUBLISH") {
		t.Error("redis-ops body should document PUBLISH for pub/sub")
	}
}

func TestRedisOpsSkill_HasLuaScripting(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if !strings.Contains(s.Body, "EVAL") {
		t.Error("redis-ops body should document EVAL for Lua scripting")
	}
}

func TestRedisOpsSkill_HasMemoryAnalysis(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if !strings.Contains(s.Body, "MEMORY USAGE") {
		t.Error("redis-ops body should document MEMORY USAGE command")
	}
	if !strings.Contains(s.Body, "--bigkeys") {
		t.Error("redis-ops body should document --bigkeys flag for memory analysis")
	}
}

func TestRedisOpsSkill_HasClusterOperations(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if !strings.Contains(s.Body, "CLUSTER INFO") {
		t.Error("redis-ops body should document CLUSTER INFO for cluster operations")
	}
}

func TestRedisOpsSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if len(s.Triggers) == 0 {
		t.Error("redis-ops should have at least one trigger phrase in description")
	}
}

func TestRedisOpsSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["redis-ops"]
	if len(s.AllowedTools) == 0 {
		t.Error("redis-ops should declare allowed-tools")
	}
}

func TestNeonBranchingSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["neon-branching"]
	if !ok {
		t.Fatal("neon-branching skill not found; expected SKILL.md in skills/neon-branching/")
	}
	if s.Name != "neon-branching" {
		t.Errorf("Name = %q, want %q", s.Name, "neon-branching")
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

func TestNeonBranchingSkill_HasBranchCreate(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["neon-branching"]
	if !strings.Contains(s.Body, "neonctl branches create") {
		t.Error("neon-branching body should include 'neonctl branches create' command")
	}
}

func TestNeonBranchingSkill_HasBranchDelete(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["neon-branching"]
	if !strings.Contains(s.Body, "neonctl branches delete") {
		t.Error("neon-branching body should include 'neonctl branches delete' command")
	}
}

func TestNeonBranchingSkill_HasConnectionStrings(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["neon-branching"]
	if !strings.Contains(s.Body, "connection-string") {
		t.Error("neon-branching body should document connection string retrieval")
	}
}

func TestNeonBranchingSkill_HasBranchPerPR(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["neon-branching"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "branch-per-pr") && !strings.Contains(bodyLower, "pull request") {
		t.Error("neon-branching body should document the branch-per-PR workflow")
	}
}

func TestNeonBranchingSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["neon-branching"]
	if len(s.Triggers) == 0 {
		t.Error("neon-branching should have at least one trigger phrase in description")
	}
}

func TestNeonBranchingSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["neon-branching"]
	if len(s.AllowedTools) == 0 {
		t.Error("neon-branching should declare allowed-tools")
	}
}

// --- Cross-cutting validation for issues #83, #84, #85 ---

// TestIssue838485SkillsHaveVersion ensures every new SKILL.md has version: 1.
func TestIssue838485SkillsHaveVersion(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		// Issue #83: Security
		"vault-ops",
		"snyk-scan",
		"trivy-scan",
		// Issue #84: Testing
		"cypress-testing",
		"load-testing",
		// Issue #85: Database
		"redis-ops",
		"neon-branching",
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

// TestIssue838485SkillsHaveAllowedTools ensures all new skills declare allowed-tools.
func TestIssue838485SkillsHaveAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"vault-ops", "snyk-scan", "trivy-scan",
		"cypress-testing", "load-testing",
		"redis-ops", "neon-branching",
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

// TestIssue838485ExpectedCount validates all 7 skills for issues #83, #84, #85 are present.
func TestIssue838485ExpectedCount(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	expected := []string{
		// Issue #83: Security Skills
		"vault-ops",
		"snyk-scan",
		"trivy-scan",
		// Issue #84: Testing Skills
		"cypress-testing",
		"load-testing",
		// Issue #85: Database Skills
		"redis-ops",
		"neon-branching",
	}
	for _, name := range expected {
		if _, ok := all[name]; !ok {
			t.Errorf("expected skill %q not found", name)
		}
	}
}

// TestIssue838485SkillsDefaultToConversationContext ensures none of the new skills
// accidentally use the fork context, which requires a runner.
func TestIssue838485SkillsDefaultToConversationContext(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"vault-ops", "snyk-scan", "trivy-scan",
		"cypress-testing", "load-testing",
		"redis-ops", "neon-branching",
	}
	for _, name := range newSkills {
		s, ok := all[name]
		if !ok {
			continue
		}
		if s.Context == skills.ContextFork {
			t.Errorf("skill %q uses context=fork; these skills should use conversation context", name)
		}
	}
}

// TestIssue838485SkillsHaveTriggers ensures all new skills define trigger phrases.
func TestIssue838485SkillsHaveTriggers(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"vault-ops", "snyk-scan", "trivy-scan",
		"cypress-testing", "load-testing",
		"redis-ops", "neon-branching",
	}
	for _, name := range newSkills {
		s, ok := all[name]
		if !ok {
			continue
		}
		if len(s.Triggers) == 0 {
			t.Errorf("skill %q: should have at least one trigger phrase in description", name)
		}
	}
}
