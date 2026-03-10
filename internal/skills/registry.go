package skills

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Registry provides thread-safe access to loaded skills.
// Local skills take precedence over global skills with the same name.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
	}
}

// Load loads all skills from the given Loader into the registry.
// Global skills are loaded first, then local skills override globals with the same name.
func (r *Registry) Load(loader *Loader) error {
	skills, err := loader.Load()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range skills {
		s := skills[i]
		existing, exists := r.skills[s.Name]
		if !exists || (existing.Source == SourceGlobal && s.Source == SourceLocal) {
			r.skills[s.Name] = &s
		}
	}

	return nil
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// List returns all skills sorted by name.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Reload replaces all skills in the registry with a fresh load from the
// given Loader. Unlike Load, Reload discards the previous skill set
// entirely before inserting the newly-loaded skills, which is the correct
// semantics for hot-reload: a deleted SKILL.md should unregister its skill.
func (r *Registry) Reload(loader *Loader) error {
	skills, err := loader.Load()
	if err != nil {
		return err
	}

	newMap := make(map[string]*Skill, len(skills))
	for i := range skills {
		s := skills[i]
		existing, exists := newMap[s.Name]
		if !exists || (existing.Source == SourceGlobal && s.Source == SourceLocal) {
			newMap[s.Name] = &s
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = newMap
	return nil
}

// GetFilePath returns the absolute path to a skill's SKILL.md file.
// Returns empty string and false if the skill does not exist.
func (r *Registry) GetFilePath(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	if !ok {
		return "", false
	}
	return s.FilePath, true
}

// UpdateSkillVerification updates the verified status of a skill in the registry.
// It returns an error if the skill does not exist.
func (r *Registry) UpdateSkillVerification(_ context.Context, name string, verified bool, verifiedAt time.Time, verifiedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found in registry", name)
	}
	s.Verified = verified
	s.VerifiedAt = verifiedAt.UTC().Format(time.RFC3339)
	s.VerifiedBy = verifiedBy
	return nil
}

// MatchTriggers returns skills whose triggers match the given text.
func (r *Registry) MatchTriggers(text string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*Skill
	for _, s := range r.skills {
		if len(s.Triggers) > 0 && MatchTrigger(text, s.Triggers) {
			matched = append(matched, s)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})
	return matched
}
