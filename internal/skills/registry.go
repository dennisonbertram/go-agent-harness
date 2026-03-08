package skills

import (
	"sort"
	"sync"
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
