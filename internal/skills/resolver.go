package skills

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SkillResolver resolves a skill name + args into interpolated content.
type SkillResolver interface {
	ResolveSkill(name, args, workspace string) (string, error)
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
// arguments and workspace, and returns the result.
func (r *Resolver) ResolveSkill(name, args, workspace string) (string, error) {
	skill, ok := r.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	vars := map[string]string{
		"$ARGUMENTS": args,
		"$WORKSPACE": workspace,
		"$SKILL_DIR": filepath.Dir(skill.FilePath),
	}

	// Split args into positional parameters
	fields := strings.Fields(args)
	for i := 1; i <= 9; i++ {
		key := fmt.Sprintf("$%d", i)
		if i-1 < len(fields) {
			vars[key] = fields[i-1]
		}
	}

	return Interpolate(skill.Body, vars), nil
}
