package symphd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Workflow holds parsed WORKFLOW.md content.
// The file format is:
//
//	---
//	max_concurrent_agents: 5
//	max_turns: 20
//	workspace_type: worktree
//	track_label: symphd
//	---
//	# Workflow: Fix GitHub Issue
//
//	You are an agent working on issue #{{ .issue_number }}: {{ .issue_title }}.
//
// Front matter fields override the global Config when non-zero.
// The body is a Go text/template prompt using {{ .variable_name }} syntax.
type Workflow struct {
	// Front matter config overrides (zero values mean "use global config default").
	MaxConcurrentAgents int    `yaml:"max_concurrent_agents"`
	MaxTurns            int    `yaml:"max_turns"`
	TurnTimeoutMs       int    `yaml:"turn_timeout_ms"`
	RetryMaxAttempts    int    `yaml:"retry_max_attempts"`
	WorkspaceType       string `yaml:"workspace_type"`
	TrackLabel          string `yaml:"track_label"`

	// Extra captures unknown front matter fields for forward compatibility.
	Extra map[string]any `yaml:",inline"`

	// Template is the Markdown prompt template body (after front matter).
	Template string
}

const frontMatterDelimiter = "---"

// LoadWorkflow reads and parses a WORKFLOW.md file.
// Front matter is YAML between --- delimiters at the start of the file.
// The body after the second --- is stored as the prompt template.
// Returns an error if the file is missing, empty, or has malformed front matter.
func LoadWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("symphd: read workflow %q: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("symphd: workflow %q is empty", path)
	}

	content := string(data)
	wf := &Workflow{}

	// Check for YAML front matter: file must start with "---\n"
	if strings.HasPrefix(content, frontMatterDelimiter+"\n") {
		// Find the closing delimiter
		rest := content[len(frontMatterDelimiter)+1:] // skip opening "---\n"
		closeIdx := strings.Index(rest, "\n"+frontMatterDelimiter)
		if closeIdx == -1 {
			// No closing delimiter — treat entire content as template (no front matter)
			wf.Template = content
			return wf, nil
		}

		yamlContent := rest[:closeIdx]
		// Body is everything after the closing "\n---"
		afterClose := rest[closeIdx+1+len(frontMatterDelimiter):]
		// Strip leading newline from body if present
		if strings.HasPrefix(afterClose, "\n") {
			afterClose = afterClose[1:]
		}
		wf.Template = afterClose

		// Parse the YAML front matter
		if err := yaml.Unmarshal([]byte(yamlContent), wf); err != nil {
			return nil, fmt.Errorf("symphd: parse workflow front matter %q: %w", path, err)
		}
	} else {
		// No front matter: treat entire content as template
		wf.Template = content
	}

	return wf, nil
}

// RenderPrompt renders the workflow template with the given variables.
// Variables in the template use Go text/template syntax: {{ .variable_name }}.
// Returns an error if any variable referenced in the template is not provided (strict mode).
// Passing nil for vars is allowed when the template has no variables.
func (w *Workflow) RenderPrompt(vars map[string]string) (string, error) {
	tmpl, err := template.New("workflow").Option("missingkey=error").Parse(w.Template)
	if err != nil {
		return "", fmt.Errorf("symphd: parse workflow template: %w", err)
	}

	// Convert map[string]string to map[string]any for text/template execution.
	data := make(map[string]any, len(vars))
	for k, v := range vars {
		data[k] = v
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("symphd: render workflow template: %w", err)
	}
	return buf.String(), nil
}
