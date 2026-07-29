package repostructure

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIssueProcessProvidesOnlyRequiredStructuredForms(t *testing.T) {
	t.Parallel()

	templateDir := filepath.Join(repoRoot(t), ".github", "ISSUE_TEMPLATE")
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatal(err)
	}

	var formNames []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			t.Errorf("legacy Markdown issue template remains: %s", entry.Name())
		}
		if entry.Name() != "config.yml" && strings.HasSuffix(entry.Name(), ".yml") {
			formNames = append(formNames, entry.Name())
		}
	}
	sort.Strings(formNames)
	wantNames := []string{
		"bug-regression.yml",
		"epic.yml",
		"feature-slice.yml",
		"minor-change.yml",
		"research-spike.yml",
	}
	if strings.Join(formNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("Issue Forms = %v, want %v", formNames, wantNames)
	}

	contracts := expectedIssueFormContracts()
	for _, name := range formNames {
		var form issueForm
		raw, err := os.ReadFile(filepath.Join(templateDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := yaml.Unmarshal(raw, &form); err != nil {
			t.Fatalf("%s is not valid YAML: %v", name, err)
		}
		if form.Name == "" || form.Description == "" || len(form.Body) < 8 {
			t.Errorf("%s lacks an exhaustive form contract", name)
		}

		contract := contracts[name]
		labels := make(map[string]struct{})
		ids := make(map[string]struct{})
		workTypeMatches := false
		for _, item := range form.Body {
			if item.Type == "markdown" {
				continue
			}
			if item.ID == "" {
				t.Errorf("%s has %s field without stable id", name, item.Type)
			}
			if _, exists := ids[item.ID]; exists {
				t.Errorf("%s repeats field id %q", name, item.ID)
			}
			ids[item.ID] = struct{}{}
			if !item.Validations.Required {
				t.Errorf("%s field %q is not required", name, item.ID)
			}
			labels[item.Attributes.Label] = struct{}{}
			if item.ID == "work_type" &&
				len(item.Attributes.Options) == 1 &&
				item.Attributes.Options[0] == contract.workType {
				workTypeMatches = true
			}
		}
		if !workTypeMatches {
			t.Errorf("%s lacks exact Work type marker %q", name, contract.workType)
		}
		for _, required := range contract.requiredLabels {
			if _, ok := labels[required]; !ok {
				t.Errorf("%s missing required field label %q", name, required)
			}
		}
	}
}

func TestIssueSelectorDisablesBlankIssuesAndRoutesPrivateSecurityReports(t *testing.T) {
	t.Parallel()

	raw := readRepoFile(t, ".github/ISSUE_TEMPLATE/config.yml")
	if !strings.Contains(raw, "blank_issues_enabled: false") {
		t.Error("blank issues must remain disabled")
	}
	if !strings.Contains(raw, "/security/advisories/new") {
		t.Error("issue selector must link private vulnerability reporting")
	}
}

func TestPullRequestTemplateCarriesIssueScopeAndEvidence(t *testing.T) {
	t.Parallel()

	raw := readRepoFile(t, ".github/pull_request_template.md")
	for _, required := range []string{
		"Summary",
		"Scope and issue reconciliation",
		"Impact analysis reconciliation",
		"Architecture and duplication check",
		"Test-first evidence",
		"Verification evidence",
		"Rollout and rollback",
		"Documentation",
		"Contract checklist",
	} {
		if !strings.Contains(raw, "## "+required) {
			t.Errorf("pull request template missing %q", required)
		}
	}
	for _, required := range []string{"Closes #", "Red command", "Observed failure"} {
		if !strings.Contains(raw, required) {
			t.Errorf("pull request template missing evidence marker %q", required)
		}
	}
}

func TestClaudeAndAgentPoliciesRequireIssueFirstImpactAnalysis(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"AGENTS.md", "CLAUDE.md"} {
		raw := readRepoFile(t, file)
		for _, policy := range []string{
			"Every change requires a GitHub issue",
			"Minor changes do not bypass",
			"impact analysis",
			"Do not begin implementation",
		} {
			if !strings.Contains(raw, policy) {
				t.Errorf("%s missing policy %q", file, policy)
			}
		}
	}
}

type issueForm struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Body        []issueFormItem `yaml:"body"`
}

type issueFormItem struct {
	Type       string `yaml:"type"`
	ID         string `yaml:"id"`
	Attributes struct {
		Label   string `yaml:"label"`
		Options []any  `yaml:"options"`
	} `yaml:"attributes"`
	Validations struct {
		Required bool `yaml:"required"`
	} `yaml:"validations"`
}

type issueFormContract struct {
	workType       string
	requiredLabels []string
}

func expectedIssueFormContracts() map[string]issueFormContract {
	return map[string]issueFormContract{
		"feature-slice.yml": {
			workType: "Engineering change / feature slice",
			requiredLabels: []string{
				"Why this matters",
				"Protected path",
				"Acceptance contract",
				"Current architecture and search evidence",
				"Cross-surface impact map",
				"In scope",
				"Out of scope",
				"Test-first plan",
				"Verification plan",
				"Rollout and rollback",
				"Documentation and handoff",
				"Definition of done",
			},
		},
		"bug-regression.yml": {
			workType: "Bug / regression",
			requiredLabels: []string{
				"Observed behavior",
				"Expected behavior",
				"Reproduction",
				"User and operational impact",
				"Suspected seam and search evidence",
				"Blast-radius impact map",
				"Regression test first",
				"Fix boundaries",
				"Verification plan",
				"Rollout and rollback",
				"Documentation and handoff",
				"Definition of done",
			},
		},
		"minor-change.yml": {
			workType: "Minor documentation-only change",
			requiredLabels: []string{
				"Exact change",
				"Why this is minor",
				"Files expected",
				"Behavior and impact attestation",
				"Verification plan",
				"Rollback",
				"Definition of done",
			},
		},
		"research-spike.yml": {
			workType: "Research spike",
			requiredLabels: []string{
				"Decision question",
				"Current evidence",
				"Options to evaluate",
				"Evaluation criteria",
				"Timebox and stop conditions",
				"Experiments and evidence",
				"Security and data boundaries",
				"Required output",
				"Follow-on decision",
				"Definition of done",
			},
		},
		"epic.yml": {
			workType: "Epic",
			requiredLabels: []string{
				"Problem and outcome",
				"Non-goals",
				"Current architecture and evidence",
				"Cross-surface impact map",
				"Shippable child issues",
				"Dependency graph",
				"Integration contracts",
				"Rollout sequence",
				"Risks and observability",
				"Definition of done",
			},
		},
	}
}

func readRepoFile(t *testing.T, relative string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
