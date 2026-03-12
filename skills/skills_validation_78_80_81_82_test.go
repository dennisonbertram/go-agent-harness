// Package skills_validation tests that the bundled SKILL.md files in this
// directory parse correctly and satisfy required field constraints.
//
// This file covers Issues #78 (Dependency Management), #80 (Deployment),
// #81 (Monitoring), and #82 (Collaboration) skill additions.
package skills_validation

import (
	"strings"
	"testing"

	"go-agent-harness/internal/skills"
)

// --- Issue #78: Go Dependency Management ---

func TestGoDepsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["go-deps"]
	if !ok {
		t.Fatal("go-deps skill not found; expected SKILL.md in skills/go-deps/")
	}
	if s.Name != "go-deps" {
		t.Errorf("Name = %q, want %q", s.Name, "go-deps")
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

func TestGoDepsSkill_HasTidyCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if !strings.Contains(s.Body, "go mod tidy") {
		t.Error("go-deps body should include 'go mod tidy' command")
	}
}

func TestGoDepsSkill_HasUpdateCommands(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if !strings.Contains(s.Body, "go get -u") {
		t.Error("go-deps body should include 'go get -u' for updating dependencies")
	}
	if !strings.Contains(s.Body, "go list -m") {
		t.Error("go-deps body should include 'go list -m' for listing updates")
	}
}

func TestGoDepsSkill_HasVendorCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if !strings.Contains(s.Body, "go mod vendor") {
		t.Error("go-deps body should include 'go mod vendor' command")
	}
}

func TestGoDepsSkill_HasSecurityAudit(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if !strings.Contains(s.Body, "govulncheck") {
		t.Error("go-deps body should include 'govulncheck' for security auditing")
	}
}

func TestGoDepsSkill_HasModuleGraph(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if !strings.Contains(s.Body, "go mod graph") {
		t.Error("go-deps body should include 'go mod graph' for dependency analysis")
	}
}

func TestGoDepsSkill_HasWhyCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if !strings.Contains(s.Body, "go mod why") {
		t.Error("go-deps body should include 'go mod why' command")
	}
}

func TestGoDepsSkill_HasUpdateStrategy(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "conservative") && !strings.Contains(bodyLower, "patch") {
		t.Error("go-deps body should document conservative/patch update strategy")
	}
}

func TestGoDepsSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if len(s.Triggers) == 0 {
		t.Error("go-deps should have at least one trigger phrase in description")
	}
}

func TestGoDepsSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["go-deps"]
	if len(s.AllowedTools) == 0 {
		t.Error("go-deps should declare allowed-tools")
	}
}

// --- Issue #78: NPM Dependency Management ---

func TestNpmDepsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["npm-deps"]
	if !ok {
		t.Fatal("npm-deps skill not found; expected SKILL.md in skills/npm-deps/")
	}
	if s.Name != "npm-deps" {
		t.Errorf("Name = %q, want %q", s.Name, "npm-deps")
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

func TestNpmDepsSkill_HasOutdatedCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if !strings.Contains(s.Body, "npm outdated") {
		t.Error("npm-deps body should include 'npm outdated' command")
	}
}

func TestNpmDepsSkill_HasUpdateCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if !strings.Contains(s.Body, "npm update") {
		t.Error("npm-deps body should include 'npm update' command")
	}
}

func TestNpmDepsSkill_HasAuditCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if !strings.Contains(s.Body, "npm audit") {
		t.Error("npm-deps body should include 'npm audit' command")
	}
}

func TestNpmDepsSkill_HasDedupeCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if !strings.Contains(s.Body, "npm dedupe") {
		t.Error("npm-deps body should include 'npm dedupe' command")
	}
}

func TestNpmDepsSkill_HasCiCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if !strings.Contains(s.Body, "npm ci") {
		t.Error("npm-deps body should include 'npm ci' command for clean installs")
	}
}

func TestNpmDepsSkill_HasLockfileContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if !strings.Contains(s.Body, "package-lock.json") {
		t.Error("npm-deps body should document package-lock.json lockfile management")
	}
}

func TestNpmDepsSkill_HasWorkspaceContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "workspace") {
		t.Error("npm-deps body should document npm workspaces")
	}
}

func TestNpmDepsSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if len(s.Triggers) == 0 {
		t.Error("npm-deps should have at least one trigger phrase in description")
	}
}

func TestNpmDepsSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["npm-deps"]
	if len(s.AllowedTools) == 0 {
		t.Error("npm-deps should declare allowed-tools")
	}
}

// --- Issue #80: Helm Deployment ---

func TestHelmDeploySkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["helm-deploy"]
	if !ok {
		t.Fatal("helm-deploy skill not found; expected SKILL.md in skills/helm-deploy/")
	}
	if s.Name != "helm-deploy" {
		t.Errorf("Name = %q, want %q", s.Name, "helm-deploy")
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

func TestHelmDeploySkill_HasInstallCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if !strings.Contains(s.Body, "helm install") {
		t.Error("helm-deploy body should include 'helm install' command")
	}
}

func TestHelmDeploySkill_HasUpgradeCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if !strings.Contains(s.Body, "helm upgrade") {
		t.Error("helm-deploy body should include 'helm upgrade' command")
	}
	if !strings.Contains(s.Body, "helm upgrade --install") {
		t.Error("helm-deploy body should include idempotent 'helm upgrade --install' pattern")
	}
}

func TestHelmDeploySkill_HasRollbackCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if !strings.Contains(s.Body, "helm rollback") {
		t.Error("helm-deploy body should include 'helm rollback' command")
	}
}

func TestHelmDeploySkill_HasListCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if !strings.Contains(s.Body, "helm list") {
		t.Error("helm-deploy body should include 'helm list' command")
	}
}

func TestHelmDeploySkill_HasUninstallSafetyNote(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if !strings.Contains(s.Body, "helm uninstall") {
		t.Error("helm-deploy body should document 'helm uninstall' command")
	}
	// Safety: uninstall is a destructive operation and should have a note
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "destructive") && !strings.Contains(bodyLower, "confirm") &&
		!strings.Contains(bodyLower, "deletes") && !strings.Contains(bodyLower, "removes") {
		t.Error("helm-deploy body should note that uninstall is destructive")
	}
}

func TestHelmDeploySkill_HasValuesContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if !strings.Contains(s.Body, "values.yaml") {
		t.Error("helm-deploy body should document values.yaml")
	}
	if !strings.Contains(s.Body, "-f values") {
		t.Error("helm-deploy body should document -f flag for values file")
	}
}

func TestHelmDeploySkill_HasSecretsContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "secret") {
		t.Error("helm-deploy body should document secrets management")
	}
}

func TestHelmDeploySkill_HasTemplateCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if !strings.Contains(s.Body, "helm template") {
		t.Error("helm-deploy body should include 'helm template' for debugging")
	}
}

func TestHelmDeploySkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if len(s.Triggers) == 0 {
		t.Error("helm-deploy should have at least one trigger phrase in description")
	}
}

func TestHelmDeploySkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["helm-deploy"]
	if len(s.AllowedTools) == 0 {
		t.Error("helm-deploy should declare allowed-tools")
	}
}

// --- Issue #80: Netlify Deployment ---

func TestNetlifyDeploySkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["netlify-deploy"]
	if !ok {
		t.Fatal("netlify-deploy skill not found; expected SKILL.md in skills/netlify-deploy/")
	}
	if s.Name != "netlify-deploy" {
		t.Errorf("Name = %q, want %q", s.Name, "netlify-deploy")
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

func TestNetlifyDeploySkill_HasDeployProdCommand(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	if !strings.Contains(s.Body, "netlify deploy --prod") {
		t.Error("netlify-deploy body should include 'netlify deploy --prod' command")
	}
}

func TestNetlifyDeploySkill_HasEnvVariables(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	if !strings.Contains(s.Body, "netlify env") {
		t.Error("netlify-deploy body should include 'netlify env' commands for environment variables")
	}
}

func TestNetlifyDeploySkill_HasRedirectsContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "redirect") {
		t.Error("netlify-deploy body should document redirects")
	}
}

func TestNetlifyDeploySkill_HasFunctionsContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "function") {
		t.Error("netlify-deploy body should document Netlify Functions")
	}
}

func TestNetlifyDeploySkill_HasNetlifyToml(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	if !strings.Contains(s.Body, "netlify.toml") {
		t.Error("netlify-deploy body should document netlify.toml configuration")
	}
}

func TestNetlifyDeploySkill_HasPostDeployVerification(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "verif") && !strings.Contains(bodyLower, "health") {
		t.Error("netlify-deploy body should include post-deploy verification patterns")
	}
}

func TestNetlifyDeploySkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	if len(s.Triggers) == 0 {
		t.Error("netlify-deploy should have at least one trigger phrase in description")
	}
}

func TestNetlifyDeploySkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["netlify-deploy"]
	if len(s.AllowedTools) == 0 {
		t.Error("netlify-deploy should declare allowed-tools")
	}
}

// --- Issue #81: Sentry Setup ---

func TestSentrySetupSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["sentry-setup"]
	if !ok {
		t.Fatal("sentry-setup skill not found; expected SKILL.md in skills/sentry-setup/")
	}
	if s.Name != "sentry-setup" {
		t.Errorf("Name = %q, want %q", s.Name, "sentry-setup")
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

func TestSentrySetupSkill_HasSentryCliReleases(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	if !strings.Contains(s.Body, "sentry-cli releases") {
		t.Error("sentry-setup body should include 'sentry-cli releases' commands")
	}
}

func TestSentrySetupSkill_HasSourceMaps(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "sourcemap") && !strings.Contains(bodyLower, "source map") {
		t.Error("sentry-setup body should document source maps upload")
	}
}

func TestSentrySetupSkill_HasGoSDK(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	if !strings.Contains(s.Body, "sentry-go") && !strings.Contains(s.Body, "getsentry/sentry-go") {
		t.Error("sentry-setup body should document Go SDK integration")
	}
}

func TestSentrySetupSkill_HasJavaScriptSDK(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	if !strings.Contains(s.Body, "@sentry/browser") && !strings.Contains(s.Body, "@sentry/react") {
		t.Error("sentry-setup body should document JavaScript SDK integration")
	}
}

func TestSentrySetupSkill_HasPerformanceTracing(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "tracing") && !strings.Contains(bodyLower, "transaction") {
		t.Error("sentry-setup body should document performance tracing")
	}
}

func TestSentrySetupSkill_HasErrorGrouping(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "fingerprint") && !strings.Contains(bodyLower, "grouping") {
		t.Error("sentry-setup body should document error grouping/fingerprinting")
	}
}

func TestSentrySetupSkill_HasDSNContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	if !strings.Contains(s.Body, "Dsn") && !strings.Contains(s.Body, "DSN") {
		t.Error("sentry-setup body should include DSN configuration")
	}
}

func TestSentrySetupSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	if len(s.Triggers) == 0 {
		t.Error("sentry-setup should have at least one trigger phrase in description")
	}
}

func TestSentrySetupSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["sentry-setup"]
	if len(s.AllowedTools) == 0 {
		t.Error("sentry-setup should declare allowed-tools")
	}
}

// --- Issue #81: Prometheus Operations ---

func TestPrometheusOpsSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["prometheus-ops"]
	if !ok {
		t.Fatal("prometheus-ops skill not found; expected SKILL.md in skills/prometheus-ops/")
	}
	if s.Name != "prometheus-ops" {
		t.Errorf("Name = %q, want %q", s.Name, "prometheus-ops")
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

func TestPrometheusOpsSkill_HasPromQL(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	if !strings.Contains(s.Body, "rate(") {
		t.Error("prometheus-ops body should include PromQL rate() function")
	}
	if !strings.Contains(s.Body, "histogram_quantile") {
		t.Error("prometheus-ops body should include histogram_quantile for latency")
	}
}

func TestPrometheusOpsSkill_HasPromtool(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	if !strings.Contains(s.Body, "promtool") {
		t.Error("prometheus-ops body should include 'promtool' commands")
	}
	if !strings.Contains(s.Body, "promtool check") {
		t.Error("prometheus-ops body should include 'promtool check' for validation")
	}
}

func TestPrometheusOpsSkill_HasMetricsExposition(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "exposition") && !strings.Contains(s.Body, "/metrics") {
		t.Error("prometheus-ops body should document metrics exposition format or /metrics endpoint")
	}
}

func TestPrometheusOpsSkill_HasAlertingRules(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "alert") {
		t.Error("prometheus-ops body should document alerting rules")
	}
	if !strings.Contains(s.Body, "for:") {
		t.Error("prometheus-ops body should show 'for:' duration in alert rules")
	}
}

func TestPrometheusOpsSkill_HasScrapeConfig(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "scrape") {
		t.Error("prometheus-ops body should document scrape configuration")
	}
}

func TestPrometheusOpsSkill_HasAPIQuery(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	if !strings.Contains(s.Body, "/api/v1/query") {
		t.Error("prometheus-ops body should document Prometheus HTTP API query endpoint")
	}
}

func TestPrometheusOpsSkill_HasGoClientLibrary(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	if !strings.Contains(s.Body, "prometheus/client_golang") && !strings.Contains(s.Body, "promhttp") {
		t.Error("prometheus-ops body should include Go client library example")
	}
}

func TestPrometheusOpsSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	if len(s.Triggers) == 0 {
		t.Error("prometheus-ops should have at least one trigger phrase in description")
	}
}

func TestPrometheusOpsSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["prometheus-ops"]
	if len(s.AllowedTools) == 0 {
		t.Error("prometheus-ops should declare allowed-tools")
	}
}

// --- Issue #82: Linear Workflow ---

func TestLinearWorkflowSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["linear-workflow"]
	if !ok {
		t.Fatal("linear-workflow skill not found; expected SKILL.md in skills/linear-workflow/")
	}
	if s.Name != "linear-workflow" {
		t.Errorf("Name = %q, want %q", s.Name, "linear-workflow")
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

func TestLinearWorkflowSkill_HasAPIAuthentication(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	if !strings.Contains(s.Body, "LINEAR_API_KEY") {
		t.Error("linear-workflow body should include LINEAR_API_KEY for authentication")
	}
}

func TestLinearWorkflowSkill_HasCreateIssue(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	if !strings.Contains(s.Body, "issueCreate") {
		t.Error("linear-workflow body should include issueCreate GraphQL mutation")
	}
}

func TestLinearWorkflowSkill_HasUpdateIssue(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	if !strings.Contains(s.Body, "issueUpdate") {
		t.Error("linear-workflow body should include issueUpdate GraphQL mutation")
	}
}

func TestLinearWorkflowSkill_HasCycleManagement(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "cycle") {
		t.Error("linear-workflow body should document cycle management")
	}
}

func TestLinearWorkflowSkill_HasGitHubIntegration(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "github") {
		t.Error("linear-workflow body should document GitHub integration")
	}
}

func TestLinearWorkflowSkill_HasGraphQLEndpoint(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	if !strings.Contains(s.Body, "api.linear.app/graphql") {
		t.Error("linear-workflow body should include the Linear GraphQL API endpoint")
	}
}

func TestLinearWorkflowSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	if len(s.Triggers) == 0 {
		t.Error("linear-workflow should have at least one trigger phrase in description")
	}
}

func TestLinearWorkflowSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["linear-workflow"]
	if len(s.AllowedTools) == 0 {
		t.Error("linear-workflow should declare allowed-tools")
	}
}

// --- Issue #82: Changelog Generation ---

func TestChangelogGenSkill_Parses(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s, ok := all["changelog-gen"]
	if !ok {
		t.Fatal("changelog-gen skill not found; expected SKILL.md in skills/changelog-gen/")
	}
	if s.Name != "changelog-gen" {
		t.Errorf("Name = %q, want %q", s.Name, "changelog-gen")
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

func TestChangelogGenSkill_HasConventionalCommits(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "conventional commit") {
		t.Error("changelog-gen body should document Conventional Commits format")
	}
}

func TestChangelogGenSkill_HasGitCliff(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	if !strings.Contains(s.Body, "git-cliff") {
		t.Error("changelog-gen body should document git-cliff tool")
	}
}

func TestChangelogGenSkill_HasKeepAChangelog(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "keep a changelog") && !strings.Contains(bodyLower, "keepachangelog") {
		t.Error("changelog-gen body should reference Keep a Changelog format")
	}
}

func TestChangelogGenSkill_HasCommitTypes(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	if !strings.Contains(s.Body, "feat") {
		t.Error("changelog-gen body should document 'feat' commit type")
	}
	if !strings.Contains(s.Body, "fix") {
		t.Error("changelog-gen body should document 'fix' commit type")
	}
}

func TestChangelogGenSkill_HasBreakingChangeContent(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	if !strings.Contains(s.Body, "BREAKING CHANGE") {
		t.Error("changelog-gen body should document BREAKING CHANGE footer")
	}
}

func TestChangelogGenSkill_HasReleaseNotes(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	bodyLower := strings.ToLower(s.Body)
	if !strings.Contains(bodyLower, "release note") {
		t.Error("changelog-gen body should cover release notes generation")
	}
}

func TestChangelogGenSkill_HasTrigger(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	if len(s.Triggers) == 0 {
		t.Error("changelog-gen should have at least one trigger phrase in description")
	}
}

func TestChangelogGenSkill_HasAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	s := all["changelog-gen"]
	if len(s.AllowedTools) == 0 {
		t.Error("changelog-gen should declare allowed-tools")
	}
}

// --- Cross-cutting validation for issues #78, #80, #81, #82 ---

// TestIssue78To82Skills_HaveVersion ensures every new SKILL.md has version: 1.
func TestIssue78To82Skills_HaveVersion(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"go-deps", "npm-deps",
		"helm-deploy", "netlify-deploy",
		"sentry-setup", "prometheus-ops",
		"linear-workflow", "changelog-gen",
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

// TestIssue78To82Skills_HaveAllowedTools ensures all new skills declare allowed-tools.
func TestIssue78To82Skills_HaveAllowedTools(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"go-deps", "npm-deps",
		"helm-deploy", "netlify-deploy",
		"sentry-setup", "prometheus-ops",
		"linear-workflow", "changelog-gen",
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

// TestIssue78To82Skills_ExpectedCount validates all 8 skills for issues
// #78, #80, #81, #82 are present. This catches accidentally deleted or
// renamed skill directories.
func TestIssue78To82Skills_ExpectedCount(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	expected := []string{
		// Issue #78: Dependency Management
		"go-deps",
		"npm-deps",
		// Issue #80: Deployment Skills
		"helm-deploy",
		"netlify-deploy",
		// Issue #81: Monitoring Skills
		"sentry-setup",
		"prometheus-ops",
		// Issue #82: Collaboration Skills
		"linear-workflow",
		"changelog-gen",
	}
	for _, name := range expected {
		if _, ok := all[name]; !ok {
			t.Errorf("expected skill %q not found", name)
		}
	}
}

// TestIssue78To82Skills_DefaultToConversationContext ensures none of the new
// skills accidentally use the fork context.
func TestIssue78To82Skills_DefaultToConversationContext(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"go-deps", "npm-deps",
		"helm-deploy", "netlify-deploy",
		"sentry-setup", "prometheus-ops",
		"linear-workflow", "changelog-gen",
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

// TestIssue78To82Skills_HaveTriggers ensures all new skills define at least
// one trigger phrase (needed for auto-invocation).
func TestIssue78To82Skills_HaveTriggers(t *testing.T) {
	t.Parallel()
	all := loadAllSkills(t)
	newSkills := []string{
		"go-deps", "npm-deps",
		"helm-deploy", "netlify-deploy",
		"sentry-setup", "prometheus-ops",
		"linear-workflow", "changelog-gen",
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
