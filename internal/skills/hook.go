package skills

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AutoInvokeHook returns a function that detects skill invocations in user messages.
// It handles two patterns:
//  1. Explicit: "/skill-name args" as the user message
//  2. Auto-invoke: trigger phrase matching (only for skills with AutoInvoke=true)
//
// The returned function takes a user message string and returns the matched skill name
// and its interpolated content. If no skill matches, both return values are empty strings.
//
// For explicit invocation, the skill name is extracted from the slash prefix and looked
// up in the registry. For auto-invocation, exactly one AutoInvoke-enabled skill must
// match to avoid ambiguity; multiple matches return empty strings.
func AutoInvokeHook(registry *Registry) func(lastUserMessage string) (skillName string, skillContent string) {
	return func(lastUserMessage string) (string, string) {
		msg := strings.TrimSpace(lastUserMessage)
		if msg == "" {
			return "", ""
		}

		// 1. Explicit invocation: /skill-name [args]
		if strings.HasPrefix(msg, "/") {
			parts := strings.SplitN(msg[1:], " ", 2)
			name := strings.TrimSpace(parts[0])
			args := ""
			if len(parts) > 1 {
				args = strings.TrimSpace(parts[1])
			}
			skill, ok := registry.Get(name)
			if ok {
				vars := buildVars(skill, args, "")
				content := Interpolate(skill.Body, vars)
				return skill.Name, content
			}
			// Not a known skill — fall through to trigger matching
		}

		// 2. Auto-invocation via trigger matching
		matched := registry.MatchTriggers(msg)
		// Only auto-invoke skills with AutoInvoke=true
		var autoInvokeMatches []*Skill
		for _, s := range matched {
			if s.AutoInvoke {
				autoInvokeMatches = append(autoInvokeMatches, s)
			}
		}
		// Exactly one match required to avoid ambiguity
		if len(autoInvokeMatches) == 1 {
			skill := autoInvokeMatches[0]
			vars := buildVars(skill, msg, "")
			content := Interpolate(skill.Body, vars)
			return skill.Name, content
		}

		return "", ""
	}
}

// buildVars creates the variable map for skill body interpolation.
func buildVars(skill *Skill, args, workspace string) map[string]string {
	vars := map[string]string{
		"$ARGUMENTS": args,
		"$WORKSPACE": workspace,
		"$SKILL_DIR": filepath.Dir(skill.FilePath),
	}
	fields := strings.Fields(args)
	for i := 1; i <= 9; i++ {
		key := fmt.Sprintf("$%d", i)
		if i-1 < len(fields) {
			vars[key] = fields[i-1]
		}
	}
	return vars
}
