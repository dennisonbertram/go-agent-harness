package skills

import (
	"context"
	"fmt"
)

// SkillResolver resolves a skill name + args into interpolated content.
type SkillResolver interface {
	ResolveSkill(ctx context.Context, name, args, workspace string) (string, error)
}

// Resolver implements SkillResolver using a Registry.
type Resolver struct {
	registry *Registry
}

// NewResolver creates a new Resolver backed by the given Registry.
func NewResolver(registry *Registry) *Resolver {
	return &Resolver{registry: registry}
}

// ResolveSkill looks up a skill by name, interpolates its body with the given
// arguments and workspace, and returns the result. Shell command preprocessing
// (!`cmd`) is applied after variable interpolation.
func (r *Resolver) ResolveSkill(ctx context.Context, name, args, workspace string) (string, error) {
	skill, ok := r.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	vars := buildVars(skill, args, workspace)
	content := Interpolate(skill.Body, vars)
	content = preprocessCommands(ctx, content, workspace)
	return content, nil
}
