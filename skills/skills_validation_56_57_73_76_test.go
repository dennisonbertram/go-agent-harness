// Package skills_validation tests that the bundled SKILL.md files in this
// directory parse correctly and satisfy required field constraints.
//
// Each SKILL.md file is loaded via the production Loader to ensure any future
// loader changes are caught by these tests automatically.
package skills_validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-agent-harness/internal/skills"
)

// --- Issue #56: Docker Skills ---

func TestDockerBuildSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["docker-build"]
	if !ok {
		t.Fatal("docker-build skill not found; expected SKILL.md in skills/docker-build/")
	}
	if s.Name != "docker-build" {
		t.Errorf("Name = %q, want %q", s.Name, "docker-build")
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

func TestDockerBuildSkill_HasBuildCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-build"]
	if !strings.Contains(s.Body, "docker build") {
		t.Error("docker-build body should include 'docker build' command")
	}
	if !strings.Contains(s.Body, "--build-arg") {
		t.Error("docker-build body should document --build-arg flag")
	}
	if !strings.Contains(s.Body, "multi-stage") || !strings.Contains(s.Body, "Multi-Stage") ||
		(!strings.Contains(s.Body, "Stage") && !strings.Contains(s.Body, "stage")) {
		// flexible check: just ensure multi-stage is covered
		if !strings.Contains(strings.ToLower(s.Body), "multi-stage") {
			t.Error("docker-build body should document multi-stage builds")
		}
	}
}

func TestDockerBuildSkill_HasTaggingContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-build"]
	if !strings.Contains(s.Body, "-t ") {
		t.Error("docker-build body should document -t (tag) flag")
	}
	if !strings.Contains(s.Body, "latest") {
		t.Error("docker-build body should document :latest tag convention")
	}
}

func TestDockerBuildSkill_HasCachingContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-build"]
	if !strings.Contains(s.Body, "--no-cache") {
		t.Error("docker-build body should document --no-cache flag")
	}
}

func TestDockerBuildSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-build"]
	if len(s.Triggers) == 0 {
		t.Error("docker-build should have at least one trigger phrase in description")
	}
}

func TestDockerBuildSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-build"]
	if len(s.AllowedTools) == 0 {
		t.Error("docker-build should declare allowed-tools")
	}
}

func TestDockerComposeSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["docker-compose"]
	if !ok {
		t.Fatal("docker-compose skill not found; expected SKILL.md in skills/docker-compose/")
	}
	if s.Name != "docker-compose" {
		t.Errorf("Name = %q, want %q", s.Name, "docker-compose")
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

func TestDockerComposeSkill_HasUpDownCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-compose"]
	if !strings.Contains(s.Body, "docker compose up") {
		t.Error("docker-compose body should include 'docker compose up'")
	}
	if !strings.Contains(s.Body, "docker compose down") {
		t.Error("docker-compose body should include 'docker compose down'")
	}
}

func TestDockerComposeSkill_HasLogsAndExec(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-compose"]
	if !strings.Contains(s.Body, "docker compose logs") {
		t.Error("docker-compose body should include 'docker compose logs'")
	}
	if !strings.Contains(s.Body, "docker compose exec") {
		t.Error("docker-compose body should include 'docker compose exec'")
	}
}

func TestDockerComposeSkill_HasNetworkingContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-compose"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "network") {
		t.Error("docker-compose body should document networking")
	}
}

func TestDockerComposeSkill_HasVolumesContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-compose"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "volume") {
		t.Error("docker-compose body should document volumes")
	}
}

func TestDockerPushSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["docker-push"]
	if !ok {
		t.Fatal("docker-push skill not found; expected SKILL.md in skills/docker-push/")
	}
	if s.Name != "docker-push" {
		t.Errorf("Name = %q, want %q", s.Name, "docker-push")
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

func TestDockerPushSkill_HasRegistries(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-push"]
	if !strings.Contains(s.Body, "DockerHub") && !strings.Contains(s.Body, "docker login") {
		t.Error("docker-push body should cover DockerHub")
	}
	if !strings.Contains(s.Body, "ghcr.io") {
		t.Error("docker-push body should cover GitHub Container Registry (ghcr.io)")
	}
	if !strings.Contains(s.Body, "ECR") {
		t.Error("docker-push body should cover AWS ECR")
	}
}

func TestDockerPushSkill_HasPushCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-push"]
	if !strings.Contains(s.Body, "docker push") {
		t.Error("docker-push body should include 'docker push' command")
	}
}

func TestDockerPushSkill_HasTaggingConventions(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-push"]
	if !strings.Contains(s.Body, "docker tag") {
		t.Error("docker-push body should include 'docker tag' command")
	}
	if !strings.Contains(s.Body, "latest") {
		t.Error("docker-push body should document :latest tag convention")
	}
}

func TestDockerDebugSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["docker-debug"]
	if !ok {
		t.Fatal("docker-debug skill not found; expected SKILL.md in skills/docker-debug/")
	}
	if s.Name != "docker-debug" {
		t.Errorf("Name = %q, want %q", s.Name, "docker-debug")
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

func TestDockerDebugSkill_HasLogsCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-debug"]
	if !strings.Contains(s.Body, "docker logs") {
		t.Error("docker-debug body should include 'docker logs' command")
	}
}

func TestDockerDebugSkill_HasExecCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-debug"]
	if !strings.Contains(s.Body, "docker exec") {
		t.Error("docker-debug body should include 'docker exec' command")
	}
}

func TestDockerDebugSkill_HasInspectCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-debug"]
	if !strings.Contains(s.Body, "docker inspect") {
		t.Error("docker-debug body should include 'docker inspect' command")
	}
}

func TestDockerDebugSkill_HasStatsCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-debug"]
	if !strings.Contains(s.Body, "docker stats") {
		t.Error("docker-debug body should include 'docker stats' command")
	}
}

func TestDockerDebugSkill_HasTroubleshootingPatterns(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["docker-debug"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "troubleshoot") && !strings.Contains(bodyLower, "crash") &&
		!strings.Contains(bodyLower, "oom") && !strings.Contains(bodyLower, "exit") {
		t.Error("docker-debug body should cover troubleshooting patterns (crashes, OOM, exit codes)")
	}
}

// --- Issue #57: Railway Deployment Skill ---

func TestRailwayDeploySkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["railway-deploy"]
	if !ok {
		t.Fatal("railway-deploy skill not found; expected SKILL.md in skills/railway-deploy/")
	}
	if s.Name != "railway-deploy" {
		t.Errorf("Name = %q, want %q", s.Name, "railway-deploy")
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

func TestRailwayDeploySkill_HasDeployCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["railway-deploy"]
	if !strings.Contains(s.Body, "railway up") {
		t.Error("railway-deploy body should include 'railway up' command")
	}
}

func TestRailwayDeploySkill_HasEnvironmentManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["railway-deploy"]
	if !strings.Contains(s.Body, "railway variables") {
		t.Error("railway-deploy body should include 'railway variables' for environment management")
	}
}

func TestRailwayDeploySkill_HasDomainManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["railway-deploy"]
	if !strings.Contains(s.Body, "railway domain") {
		t.Error("railway-deploy body should include 'railway domain' for domain management")
	}
}

func TestRailwayDeploySkill_HasLogsCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["railway-deploy"]
	if !strings.Contains(s.Body, "railway logs") {
		t.Error("railway-deploy body should include 'railway logs' command")
	}
}

func TestRailwayDeploySkill_HasSecretsContent(t *testing.T) {
	t.Parallel()
	dir := skillsDir(t)
	path := filepath.Join(dir, "railway-deploy", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading railway-deploy SKILL.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "RAILWAY_TOKEN") {
		t.Error("railway-deploy SKILL.md should document RAILWAY_TOKEN for CI/CD authentication")
	}
}

func TestRailwayDeploySkill_HasPostDeployVerification(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["railway-deploy"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "verif") && !strings.Contains(bodyLower, "health") {
		t.Error("railway-deploy body should include post-deploy verification patterns")
	}
}

func TestRailwayDeploySkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["railway-deploy"]
	if len(s.Triggers) == 0 {
		t.Error("railway-deploy should have at least one trigger phrase in description")
	}
}

// --- Issue #73: Terraform Skill ---

func TestTerraformSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["terraform"]
	if !ok {
		t.Fatal("terraform skill not found; expected SKILL.md in skills/terraform/")
	}
	if s.Name != "terraform" {
		t.Errorf("Name = %q, want %q", s.Name, "terraform")
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

func TestTerraformSkill_HasCoreWorkflow(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	if !strings.Contains(s.Body, "terraform init") {
		t.Error("terraform body should include 'terraform init'")
	}
	if !strings.Contains(s.Body, "terraform plan") {
		t.Error("terraform body should include 'terraform plan'")
	}
	if !strings.Contains(s.Body, "terraform apply") {
		t.Error("terraform body should include 'terraform apply'")
	}
	if !strings.Contains(s.Body, "terraform destroy") {
		t.Error("terraform body should include 'terraform destroy'")
	}
}

func TestTerraformSkill_HasStateManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	if !strings.Contains(s.Body, "terraform state") {
		t.Error("terraform body should include 'terraform state' commands")
	}
	if !strings.Contains(s.Body, "terraform state list") {
		t.Error("terraform body should include 'terraform state list'")
	}
}

func TestTerraformSkill_HasVariables(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	if !strings.Contains(s.Body, "-var") {
		t.Error("terraform body should document -var flag for variables")
	}
	if !strings.Contains(s.Body, "-var-file") {
		t.Error("terraform body should document -var-file flag")
	}
}

func TestTerraformSkill_HasWorkspaces(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	if !strings.Contains(s.Body, "terraform workspace") {
		t.Error("terraform body should document 'terraform workspace' commands")
	}
	if !strings.Contains(s.Body, "workspace new") {
		t.Error("terraform body should document 'workspace new' command")
	}
}

func TestTerraformSkill_HasModules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "module") {
		t.Error("terraform body should document modules")
	}
}

func TestTerraformSkill_HasRemoteBackend(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "backend") && !strings.Contains(bodyLower, "remote state") {
		t.Error("terraform body should document remote state backend")
	}
}

func TestTerraformSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	if len(s.Triggers) == 0 {
		t.Error("terraform should have at least one trigger phrase in description")
	}
}

func TestTerraformSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["terraform"]
	if len(s.AllowedTools) == 0 {
		t.Error("terraform should declare allowed-tools")
	}
}

// --- Issue #76: AI/ML Skills ---

func TestOllamaOpsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["ollama-ops"]
	if !ok {
		t.Fatal("ollama-ops skill not found; expected SKILL.md in skills/ollama-ops/")
	}
	if s.Name != "ollama-ops" {
		t.Errorf("Name = %q, want %q", s.Name, "ollama-ops")
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

func TestOllamaOpsSkill_HasModelManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ollama-ops"]
	if !strings.Contains(s.Body, "ollama pull") {
		t.Error("ollama-ops body should include 'ollama pull' command")
	}
	if !strings.Contains(s.Body, "ollama list") {
		t.Error("ollama-ops body should include 'ollama list' command")
	}
	if !strings.Contains(s.Body, "ollama rm") {
		t.Error("ollama-ops body should include 'ollama rm' command")
	}
}

func TestOllamaOpsSkill_HasRunCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ollama-ops"]
	if !strings.Contains(s.Body, "ollama run") {
		t.Error("ollama-ops body should include 'ollama run' command")
	}
}

func TestOllamaOpsSkill_HasServeCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ollama-ops"]
	if !strings.Contains(s.Body, "ollama serve") {
		t.Error("ollama-ops body should include 'ollama serve' command")
	}
}

func TestOllamaOpsSkill_HasModelfileContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ollama-ops"]
	if !strings.Contains(s.Body, "Modelfile") {
		t.Error("ollama-ops body should document Modelfile creation")
	}
}

func TestOllamaOpsSkill_HasAPIUsage(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ollama-ops"]
	if !strings.Contains(s.Body, "api/generate") && !strings.Contains(s.Body, "/v1/chat") {
		t.Error("ollama-ops body should document the Ollama REST API")
	}
	if !strings.Contains(s.Body, "localhost:11434") {
		t.Error("ollama-ops body should include the default Ollama server address")
	}
}

func TestOllamaOpsSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["ollama-ops"]
	if len(s.Triggers) == 0 {
		t.Error("ollama-ops should have at least one trigger phrase in description")
	}
}

func TestVectorDBSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["vector-db"]
	if !ok {
		t.Fatal("vector-db skill not found; expected SKILL.md in skills/vector-db/")
	}
	if s.Name != "vector-db" {
		t.Errorf("Name = %q, want %q", s.Name, "vector-db")
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

func TestVectorDBSkill_HasChromaContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vector-db"]
	if !strings.Contains(s.Body, "Chroma") && !strings.Contains(s.Body, "chroma") {
		t.Error("vector-db body should cover Chroma vector database")
	}
}

func TestVectorDBSkill_HasQdrantContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vector-db"]
	if !strings.Contains(s.Body, "Qdrant") && !strings.Contains(s.Body, "qdrant") {
		t.Error("vector-db body should cover Qdrant vector database")
	}
}

func TestVectorDBSkill_HasEmbeddingsContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vector-db"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "embedding") {
		t.Error("vector-db body should document embeddings")
	}
}

func TestVectorDBSkill_HasSimilaritySearch(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vector-db"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "search") && !strings.Contains(bodyLower, "similarity") {
		t.Error("vector-db body should document similarity search")
	}
}

func TestVectorDBSkill_HasUpsertPattern(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vector-db"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "upsert") && !strings.Contains(bodyLower, "add") {
		t.Error("vector-db body should document how to add/upsert vectors")
	}
}

func TestVectorDBSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["vector-db"]
	if len(s.Triggers) == 0 {
		t.Error("vector-db should have at least one trigger phrase in description")
	}
}

// --- Cross-cutting validation for issues #56, #57, #73, #76 ---

// TestNewSkillsHaveVersion ensures every new SKILL.md has version: 1.
func TestNewSkillsHaveVersion(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"docker-build", "docker-compose", "docker-push", "docker-debug",
		"railway-deploy",
		"terraform",
		"ollama-ops", "vector-db",
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

// TestNewSkillsHaveAllowedTools ensures all new skills declare allowed-tools.
func TestNewSkillsHaveAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"docker-build", "docker-compose", "docker-push", "docker-debug",
		"railway-deploy",
		"terraform",
		"ollama-ops", "vector-db",
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

// TestNewSkillsExpectedCount validates all 8 skills for issues #56, #57, #73, #76
// are present. This catches accidentally deleted or renamed skill directories.
func TestNewSkillsExpectedCount(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	expected := []string{
		// Issue #56: Docker Skills
		"docker-build",
		"docker-compose",
		"docker-push",
		"docker-debug",
		// Issue #57: Railway Deployment
		"railway-deploy",
		// Issue #73: Terraform
		"terraform",
		// Issue #76: AI/ML Skills
		"ollama-ops",
		"vector-db",
	}
	for _, name := range expected {
		if _, ok := all[name]; !ok {
			t.Errorf("expected skill %q not found", name)
		}
	}
}

// TestNewSkillsDefaultToConversationContext ensures none of the new skills
// accidentally use the fork context, which requires a runner.
func TestNewSkillsDefaultToConversationContext(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"docker-build", "docker-compose", "docker-push", "docker-debug",
		"railway-deploy",
		"terraform",
		"ollama-ops", "vector-db",
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
