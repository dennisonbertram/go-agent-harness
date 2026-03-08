package skills

// SkillSource indicates where a skill was loaded from.
type SkillSource string

const (
	SourceGlobal SkillSource = "global"
	SourceLocal  SkillSource = "local"
)

// Skill represents a parsed SKILL.md file.
type Skill struct {
	Name         string      // required, must match directory name, kebab-case
	Description  string      // required
	Body         string      // markdown body after frontmatter
	FilePath     string      // absolute path to SKILL.md
	Version      int         // required, must be 1
	AutoInvoke   bool        // default: true
	AllowedTools []string    // default: nil (all tools)
	ArgumentHint string      // optional
	Source       SkillSource // "global" or "local"
	Triggers     []string    // extracted from description "Trigger: ..."
}

// frontmatter represents the YAML frontmatter of a SKILL.md.
type frontmatter struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Version      int      `yaml:"version"`
	AutoInvoke   *bool    `yaml:"auto-invoke"`
	AllowedTools []string `yaml:"allowed-tools"`
	ArgumentHint string   `yaml:"argument-hint"`
}

// LoaderConfig holds paths for skill discovery.
type LoaderConfig struct {
	GlobalDir    string // e.g. ~/.go-harness/skills/
	WorkspaceDir string // e.g. <workspace>/.go-harness/skills/
}
