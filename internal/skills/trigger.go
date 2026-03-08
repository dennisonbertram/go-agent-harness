package skills

import (
	"strings"
)

// ExtractTriggers parses trigger phrases from a skill description.
// It looks for "Trigger:" or "Triggers:" followed by comma-separated phrases.
// Example: "Does something. Trigger: phrase one, phrase two" -> ["phrase one", "phrase two"]
func ExtractTriggers(description string) []string {
	lower := strings.ToLower(description)
	var idx int
	var found bool

	// Look for "triggers:" first, then "trigger:"
	if i := strings.Index(lower, "triggers:"); i >= 0 {
		idx = i + len("triggers:")
		found = true
	} else if i := strings.Index(lower, "trigger:"); i >= 0 {
		idx = i + len("trigger:")
		found = true
	}

	if !found {
		return nil
	}

	// Extract everything after the trigger keyword using original case
	rest := description[idx:]
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil
	}

	parts := strings.Split(rest, ",")
	var triggers []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			triggers = append(triggers, t)
		}
	}
	return triggers
}

// MatchTrigger checks if any trigger phrase appears as a substring in the text (case-insensitive).
func MatchTrigger(text string, triggers []string) bool {
	lower := strings.ToLower(text)
	for _, t := range triggers {
		if strings.Contains(lower, strings.ToLower(t)) {
			return true
		}
	}
	return false
}
