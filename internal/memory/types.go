// Package memory provides a typed, capped, file-backed memory system for the
// agent harness. It maintains a MEMORY.md index of entries with a closed type
// taxonomy and supports on-demand retrieval of individual topic files.
package memory

import "time"

// MemoryType is the closed taxonomy of memory entry types.
type MemoryType string

const (
	// MemoryTypeUser stores user preferences and habits.
	MemoryTypeUser MemoryType = "user"
	// MemoryTypeFeedback stores corrections and validated approaches.
	MemoryTypeFeedback MemoryType = "feedback"
	// MemoryTypeProject stores project-specific patterns.
	MemoryTypeProject MemoryType = "project"
	// MemoryTypeReference stores tool docs and API references.
	MemoryTypeReference MemoryType = "reference"
)

// AllMemoryTypes returns the complete closed taxonomy of valid memory types.
func AllMemoryTypes() []MemoryType {
	return []MemoryType{
		MemoryTypeUser,
		MemoryTypeFeedback,
		MemoryTypeProject,
		MemoryTypeReference,
	}
}

// IsValidMemoryType reports whether t is a member of the closed taxonomy.
func IsValidMemoryType(t MemoryType) bool {
	for _, valid := range AllMemoryTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// MemoryEntry is metadata about a single memory topic file.
// FilePath and Content are not persisted to YAML frontmatter; they are
// populated at load time from the filesystem.
type MemoryEntry struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Type        MemoryType `yaml:"type"`
	FilePath    string     `yaml:"-"`
	Content     string     `yaml:"-"`
	CreatedAt   time.Time  `yaml:"created_at"`
	UpdatedAt   time.Time  `yaml:"updated_at"`
}

// MemoryConfig holds configuration for the typed memory system.
type MemoryConfig struct {
	// Enabled controls whether the memory system is active.
	Enabled bool
	// IndexMaxLines is the maximum number of lines in MEMORY.md before trimming.
	IndexMaxLines int
	// IndexMaxBytes is the maximum size of MEMORY.md in bytes before trimming.
	IndexMaxBytes int
	// MaxTopicFiles is the maximum number of individual topic files.
	MaxTopicFiles int
	// RelevanceSelectorTopK controls how many topic files are loaded per query.
	RelevanceSelectorTopK int
	// MemoryDir is the directory for memory files relative to the workspace root.
	MemoryDir string
	// SaveValidations controls whether validated approaches are saved (not just corrections).
	SaveValidations bool
	// DriftProtectionEnabled requires verification before recommending from memory.
	DriftProtectionEnabled bool
}

// DefaultMemoryConfig returns the default configuration for the memory system.
func DefaultMemoryConfig() MemoryConfig {
	return MemoryConfig{
		Enabled:                true,
		IndexMaxLines:          200,
		IndexMaxBytes:          25600,
		MaxTopicFiles:          50,
		RelevanceSelectorTopK:  5,
		MemoryDir:              ".harness/memory",
		SaveValidations:        true,
		DriftProtectionEnabled: true,
	}
}
