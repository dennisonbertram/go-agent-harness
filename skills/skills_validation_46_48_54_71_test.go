// Package skills_validation tests that the bundled SKILL.md files in this
// directory parse correctly and satisfy required field constraints.
//
// This file covers skills added in GitHub Issues #46 (Cloudflare Workers),
// #48 (Vercel), #54 (Fly.io), and #71 (Kubernetes).
package skills_validation

import (
	"strings"
	"testing"
)

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
