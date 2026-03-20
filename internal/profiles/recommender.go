package profiles

import (
	"fmt"
	"strings"
)

// Recommendation holds the result of a profile recommendation for a given task.
type Recommendation struct {
	ProfileName string `json:"profile_name"`
	Reason      string `json:"reason"`
	Confidence  string `json:"confidence"` // "high" | "medium" | "low"
}

// heuristicRule maps a set of keywords to a profile name and a human-readable label.
type heuristicRule struct {
	keywords    []string
	profileName string
	label       string
}

// rules is the ordered list of deterministic heuristics used by RecommendProfile.
// Rules are evaluated in order; the first matching rule wins.
// All profile names must correspond to existing built-in profiles.
var rules = []heuristicRule{
	{
		keywords:    []string{"review", "audit", "check", "analyze"},
		profileName: "reviewer",
		label:       "review/audit task",
	},
	{
		keywords:    []string{"research", "search", "find", "investigate"},
		profileName: "researcher",
		label:       "research/investigation task",
	},
	{
		keywords:    []string{"bash", "shell", "script", "run command"},
		profileName: "bash-runner",
		label:       "shell/script execution task",
	},
	{
		keywords:    []string{"write file", "create file", "edit file"},
		profileName: "file-writer",
		label:       "file creation/modification task",
	},
	{
		keywords:    []string{"github", "pull request", " pr ", "issue"},
		profileName: "github",
		label:       "GitHub task",
	},
}

// RecommendProfile returns a deterministic Recommendation for the given task string.
// It uses keyword matching on the task (case-insensitive) and falls back to the
// "full" profile when no heuristic matches. No LLM inference is performed.
func RecommendProfile(task string) Recommendation {
	lower := strings.ToLower(task)

	for _, rule := range rules {
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				return Recommendation{
					ProfileName: rule.profileName,
					Reason:      fmt.Sprintf("Task matched keyword %q — using %s profile (%s).", kw, rule.profileName, rule.label),
					Confidence:  "high",
				}
			}
		}
	}

	return Recommendation{
		ProfileName: "full",
		Reason:      "No specific keyword matched — falling back to full profile (all tools available).",
		Confidence:  "low",
	}
}
