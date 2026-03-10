package packs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ActivatedPack is the result of successfully activating a skill pack.
// It contains the manifest metadata and the loaded instructions text ready
// for injection into an LLM conversation.
type ActivatedPack struct {
	// Manifest is the parsed manifest of the activated pack.
	Manifest *SkillManifest
	// Instructions is the full markdown text loaded from the instructions file.
	Instructions string
}

// PackRegistry holds a collection of skill pack manifests loaded from a directory.
// It is safe for concurrent use.
type PackRegistry struct {
	mu    sync.RWMutex
	packs map[string]*packEntry // keyed by pack name
}

// packEntry stores a manifest and the directory it was loaded from, so the
// instructions file can be located relative to the pack directory at activation time.
type packEntry struct {
	manifest *SkillManifest
	dir      string // absolute path to the pack directory
}

// NewPackRegistry scans the given directory for skill pack subdirectories,
// loads and validates each manifest, and returns a ready-to-use PackRegistry.
//
// Each pack must live in its own subdirectory. The directory name is used as a
// hint but the canonical pack name comes from the manifest's `name` field.
// The manifest file must be named `<dirname>.yaml` inside that subdirectory.
//
// If dir does not exist, an empty registry is returned without error.
// If any manifest file is malformed, an error is returned.
func NewPackRegistry(dir string) (*PackRegistry, error) {
	r := &PackRegistry{
		packs: make(map[string]*packEntry),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, fmt.Errorf("reading pack registry directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		packDir := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(packDir, entry.Name()+".yaml")

		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			// Subdirectory without a <name>.yaml manifest — skip silently.
			continue
		}

		m, err := LoadManifestFromFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("loading pack manifest %s: %w", manifestPath, err)
		}

		r.packs[m.Name] = &packEntry{
			manifest: m,
			dir:      packDir,
		}
	}

	return r, nil
}

// List returns all registered skill pack manifests sorted by name.
func (r *PackRegistry) List() []*SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SkillManifest, 0, len(r.packs))
	for _, e := range r.packs {
		result = append(result, e.manifest)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByCategory returns all packs that match the given category string
// (case-insensitive). Returns an empty (non-nil) slice if no packs match.
func (r *PackRegistry) ListByCategory(category string) []*SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(category)
	var result []*SkillManifest
	for _, e := range r.packs {
		if strings.ToLower(e.manifest.Category) == lower {
			result = append(result, e.manifest)
		}
	}
	if result == nil {
		result = []*SkillManifest{}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Find performs a case-insensitive keyword search across each pack's Name,
// DisplayName, Description, Category, and Tags. Returns a non-nil slice
// (possibly empty) of matching manifests sorted by name.
func (r *PackRegistry) Find(query string) []*SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lower := strings.ToLower(query)
	var result []*SkillManifest
	for _, e := range r.packs {
		if matchesPack(e.manifest, lower) {
			result = append(result, e.manifest)
		}
	}
	if result == nil {
		result = []*SkillManifest{}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Activate validates prerequisites for the named pack and loads its instructions.
// Returns an error if the pack does not exist, any prerequisite is unmet,
// or the instructions file cannot be read.
func (r *PackRegistry) Activate(name string) (*ActivatedPack, error) {
	r.mu.RLock()
	entry, ok := r.packs[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("skill pack %q not found in registry", name)
	}

	if prereqErrs := ValidatePrereqs(entry.manifest); len(prereqErrs) > 0 {
		msgs := make([]string, len(prereqErrs))
		for i, e := range prereqErrs {
			msgs[i] = e.Error()
		}
		return nil, fmt.Errorf("skill pack %q has unmet prerequisites:\n  - %s",
			name, strings.Join(msgs, "\n  - "))
	}

	instrPath := filepath.Join(entry.dir, entry.manifest.Instructions)
	instrBytes, err := os.ReadFile(instrPath)
	if err != nil {
		return nil, fmt.Errorf("loading instructions for pack %q from %s: %w", name, instrPath, err)
	}

	return &ActivatedPack{
		Manifest:     entry.manifest,
		Instructions: string(instrBytes),
	}, nil
}

// matchesPack reports whether the manifest matches the lowercased query string.
func matchesPack(m *SkillManifest, lowerQuery string) bool {
	if strings.Contains(strings.ToLower(m.Name), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(m.DisplayName), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(m.Description), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(m.Category), lowerQuery) {
		return true
	}
	for _, tag := range m.Tags {
		if strings.Contains(strings.ToLower(tag), lowerQuery) {
			return true
		}
	}
	return false
}
