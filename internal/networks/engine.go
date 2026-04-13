package networks

import (
	"fmt"
	"sort"
	"strings"

	"go-agent-harness/internal/workflows"
)

type Definition struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Roles       []RoleDefinition `json:"roles"`
}

type RoleDefinition struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
}

type Options struct {
	Definitions []Definition
	Workflows   *workflows.Engine
}

type Engine struct {
	defs      map[string]Definition
	workflows *workflows.Engine
}

func NewEngine(opts Options) *Engine {
	defs := make(map[string]Definition, len(opts.Definitions))
	for _, def := range opts.Definitions {
		defs[def.Name] = def
	}
	return &Engine{
		defs:      defs,
		workflows: opts.Workflows,
	}
}

func (e *Engine) ListDefinitions() []Definition {
	out := make([]Definition, 0, len(e.defs))
	for _, def := range e.defs {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (e *Engine) GetDefinition(name string) (Definition, bool) {
	def, ok := e.defs[name]
	return def, ok
}

func (e *Engine) Start(name string, input map[string]any) (workflows.Run, error) {
	if e.workflows == nil {
		return workflows.Run{}, fmt.Errorf("workflow engine is not configured")
	}
	def, ok := e.defs[name]
	if !ok {
		return workflows.Run{}, fmt.Errorf("network %q not found", name)
	}
	compiled, err := e.compile(def)
	if err != nil {
		return workflows.Run{}, err
	}
	return e.workflows.StartDefinition(compiled, input)
}

func (e *Engine) GetRun(runID string) (workflows.Run, []workflows.StepState, error) {
	return e.workflows.GetRun(runID)
}

func (e *Engine) compile(def Definition) (workflows.Definition, error) {
	if strings.TrimSpace(def.Name) == "" {
		return workflows.Definition{}, fmt.Errorf("network name is required")
	}
	steps := make([]workflows.StepDefinition, 0, len(def.Roles))
	var prevID string
	for _, role := range def.Roles {
		if strings.TrimSpace(role.ID) == "" {
			return workflows.Definition{}, fmt.Errorf("network role id is required")
		}
		prompt := strings.TrimSpace(role.Prompt)
		if prevID != "" {
			prompt += "\n\nPrevious role result:\n{{steps." + prevID + ".output.output}}"
		}
		prompt += "\n\nReturn a JSON object with fields summary, status, findings, output, and profile."
		steps = append(steps, workflows.StepDefinition{
			ID:   role.ID,
			Type: workflows.StepTypeRun,
			Run: &workflows.RunStep{
				Prompt: prompt,
				Model:  strings.TrimSpace(role.Model),
			},
		})
		prevID = role.ID
	}
	return workflows.Definition{
		Name:        def.Name,
		Description: def.Description,
		Steps:       steps,
	}, nil
}
