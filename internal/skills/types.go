package skills

// SkillSource indicates where a skill was loaded from.
type SkillSource string

const (
	SourceGlobal SkillSource = "global"
	SourceLocal  SkillSource = "local"
)

// SkillContext determines how a skill is executed.
type SkillContext string

const (
	// ContextConversation injects the skill body into the current conversation.
	// This is the default behavior.
	ContextConversation SkillContext = "conversation"

	// ContextFork spawns an isolated subagent to execute the skill.
	ContextFork SkillContext = "fork"
)

// Skill represents a parsed SKILL.md file.
type Skill struct {
	Name         string       // required, must match directory name, kebab-case
	Description  string       // required
	Body         string       // markdown body after frontmatter
	FilePath     string       // absolute path to SKILL.md
	Version      int          // required, must be 1
	AutoInvoke   bool         // default: true
	AllowedTools []string     // default: nil (all tools)
	ArgumentHint string       // optional
	Source       SkillSource  // "global" or "local"
	Triggers     []string     // extracted from description "Trigger: ..."
	Context      SkillContext // "conversation" (default) or "fork"
	Agent        string       // optional agent type hint (e.g., "Explore", "Code")
	Verified     bool         `json:"verified,omitempty"`
	VerifiedAt   string       `json:"verified_at,omitempty"` // RFC3339
	VerifiedBy   string       `json:"verified_by,omitempty"` // agent/user that verified
}

// frontmatter represents the YAML frontmatter of a SKILL.md.
type frontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Version      int      `yaml:"version"`
	AutoInvoke   *bool    `yaml:"auto-invoke"`
	AllowedTools []string `yaml:"allowed-tools"`
	ArgumentHint string   `yaml:"argument-hint"`
	Context      string   `yaml:"context"`
	Agent        string   `yaml:"agent"`
	Verified     bool     `yaml:"verified"`
	VerifiedAt   string   `yaml:"verified_at"`
	VerifiedBy   string   `yaml:"verified_by"`
}

// LoaderConfig holds paths for skill discovery.
type LoaderConfig struct {
	GlobalDir    string // e.g. ~/.go-harness/skills/
	WorkspaceDir string // e.g. <workspace>/.go-harness/skills/
}
