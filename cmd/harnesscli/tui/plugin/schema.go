// Package plugin defines the plugin schema and loader for custom slash commands.
// Plugins are read from ~/.config/harnesscli/plugins/*.json.
package plugin

import (
	"fmt"
	"regexp"
)

// Handler type constants.
const (
	HandlerBash   = "bash"
	HandlerPrompt = "prompt"
)

// validName matches names that start with a lowercase letter and contain only
// lowercase letters, digits, and hyphens.
var validName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// PluginDef is the schema for a single plugin definition loaded from JSON.
type PluginDef struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Handler        string `json:"handler"`
	Command        string `json:"command"`
	PromptTemplate string `json:"prompt_template"`
}

// Validate checks that the PluginDef is well-formed and returns an error if not.
func (p PluginDef) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !validName.MatchString(p.Name) {
		return fmt.Errorf("name %q is invalid: must match ^[a-z][a-z0-9-]*$", p.Name)
	}
	switch p.Handler {
	case HandlerBash:
		if p.Command == "" {
			return fmt.Errorf("command is required when handler=%q", HandlerBash)
		}
	case HandlerPrompt:
		if p.PromptTemplate == "" {
			return fmt.Errorf("prompt_template is required when handler=%q", HandlerPrompt)
		}
	case "":
		return fmt.Errorf("handler is required")
	default:
		return fmt.Errorf("handler %q is invalid: must be %q or %q", p.Handler, HandlerBash, HandlerPrompt)
	}
	return nil
}
