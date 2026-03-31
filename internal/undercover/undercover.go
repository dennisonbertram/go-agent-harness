// Package undercover provides configuration and utilities for stripping agent
// attribution when contributing to external or public repositories.
//
// Undercover mode is disabled by default and must be explicitly opted into.
// When active, it removes agent-identifying markers from commit messages and
// other text so that contributions to public repos do not expose the tooling.
package undercover

import (
	"regexp"
	"strings"
)

// UndercoverConfig holds all settings for undercover mode.
// TOML tag names match the [undercover] section of harness config files.
type UndercoverConfig struct {
	// Enabled is the master toggle. False by default — opt-in only.
	Enabled bool `toml:"enabled"`
	// AutoDetectPublicRepos enables automatic activation for public repos.
	AutoDetectPublicRepos bool `toml:"auto_detect_public_repos"`
	// StripCoAuthoredBy removes Co-Authored-By trailers from commit messages.
	StripCoAuthoredBy bool `toml:"strip_co_authored_by"`
	// StripModelIdentifiers removes model ID patterns from text.
	StripModelIdentifiers bool `toml:"strip_model_identifiers"`
	// HumanStyleCommits rewrites conventional-commit prefixes to human style.
	HumanStyleCommits bool `toml:"human_style_commits"`
	// CustomPatterns is a list of additional regex patterns to strip.
	CustomPatterns []string `toml:"custom_attribution_patterns"`
}

// DefaultUndercoverConfig returns sensible defaults.
// Enabled is false so that undercover mode is always opt-in and never silently
// active. The feature sub-flags are all true so that when the user opts in by
// setting enabled=true they get full protection without extra configuration.
func DefaultUndercoverConfig() UndercoverConfig {
	return UndercoverConfig{
		Enabled:               false,
		AutoDetectPublicRepos: true,
		StripCoAuthoredBy:     true,
		StripModelIdentifiers: true,
		HumanStyleCommits:     true,
		CustomPatterns:        []string{},
	}
}

// publicHostPatterns is the set of known public git hosting domains.
// A remote URL that contains any of these as a host segment is treated as
// pointing to a public repository.
var publicHostPatterns = []string{
	"github.com",
	"gitlab.com",
}

// IsPublicRepo reports whether remoteURL points to a well-known public git
// hosting platform (currently github.com and gitlab.com). Private or unknown
// hosts return false. An empty string returns false.
//
// Heuristic: scans for "github.com" or "gitlab.com" as substrings in the URL.
// This handles both HTTPS (https://github.com/…) and SSH (git@github.com:…)
// forms without requiring full URL parsing.
func IsPublicRepo(remoteURL string) bool {
	if remoteURL == "" {
		return false
	}
	lower := strings.ToLower(remoteURL)
	for _, host := range publicHostPatterns {
		if strings.Contains(lower, host) {
			return true
		}
	}
	return false
}

// ShouldActivate reports whether undercover mode should be active given the
// config and the current git remote URL.
//
// Rules:
//  1. If Enabled=true, always activate (explicit opt-in wins).
//  2. If Enabled=false and AutoDetectPublicRepos=true, activate only for
//     public repos as determined by IsPublicRepo.
//  3. Otherwise, do not activate.
func ShouldActivate(cfg UndercoverConfig, remoteURL string) bool {
	if cfg.Enabled {
		return true
	}
	if cfg.AutoDetectPublicRepos && IsPublicRepo(remoteURL) {
		return true
	}
	return false
}

// coAuthoredByRE matches an entire Co-Authored-By line, case-insensitively.
// The line may appear at the start of the string, after a newline, or at the
// end of the string. We compile with (?i) for case-insensitivity.
var coAuthoredByRE = regexp.MustCompile(`(?im)^co-authored-by:.*$`)

// StripCoAuthoredByLines removes all Co-Authored-By trailer lines from a
// commit message, case-insensitively. Other trailers (e.g. Signed-off-by:)
// are preserved. Trailing blank lines introduced by the removal are cleaned up.
func StripCoAuthoredByLines(commitMsg string) string {
	result := coAuthoredByRE.ReplaceAllString(commitMsg, "")
	// Clean up any runs of blank lines left behind (more than two consecutive
	// newlines become at most two newlines to preserve blank-line separators).
	multiBlank := regexp.MustCompile(`\n{3,}`)
	result = multiBlank.ReplaceAllString(result, "\n\n")
	return strings.TrimRight(result, "\n")
}

// modelIDRE matches agent model identifier patterns.
// Patterns matched:
//   - claude-<family>-<version>  (e.g. claude-opus-4.6, claude-sonnet-4-5)
//   - gpt-<version>              (e.g. gpt-4o, gpt-4.1-mini, gpt-5)
//
// The pattern requires the model prefix to be followed by a hyphen and then
// at least one non-space character, which prevents matching plain words.
var modelIDRE = regexp.MustCompile(`(?i)\b(claude-(opus|sonnet|haiku)-[\w.]+|gpt-[\w.]+)\b`)

// StripModelIDs removes model identifier patterns such as "claude-opus-4.6"
// and "gpt-4o" from text. Plain words like "Claude" as a name are preserved
// because the pattern requires the full "claude-family-version" form.
func StripModelIDs(text string) string {
	return modelIDRE.ReplaceAllString(text, "")
}

// StripAttribution applies stripping functions to text according to the cfg
// flags. Processing order: custom patterns, Co-Authored-By lines, model IDs.
// When all flags are false the input is returned unchanged.
func StripAttribution(text string, cfg UndercoverConfig) string {
	result := text

	// Apply custom regex patterns first.
	for _, pattern := range cfg.CustomPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			// Skip malformed patterns rather than crashing.
			continue
		}
		result = re.ReplaceAllString(result, "")
	}

	if cfg.StripCoAuthoredBy {
		result = StripCoAuthoredByLines(result)
	}

	if cfg.StripModelIdentifiers {
		result = StripModelIDs(result)
	}

	return result
}

// conventionalPrefixRE matches conventional-commit prefixes of the form:
//
//	<type>(<scope>): <message>
//	<type>: <message>
//
// where <type> is one of the standard Conventional Commits types.
// It captures the message part in group 1 so we can preserve it.
var conventionalPrefixRE = regexp.MustCompile(
	`^(?:feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(?:\([^)]*\))?!?:\s*(.+)`,
)

// HumanizeCommitMessage converts a conventional-commit prefixed message to a
// plain human-readable form. The prefix (e.g. "feat(auth): ") is stripped and
// the remaining message body is returned with its first letter capitalised.
// Messages that do not match the conventional-commit pattern are returned
// unchanged.
func HumanizeCommitMessage(msg string) string {
	// Only operate on the first line (subject); preserve body if present.
	lines := strings.SplitN(msg, "\n", 2)
	subject := lines[0]

	matches := conventionalPrefixRE.FindStringSubmatch(subject)
	if matches == nil {
		// No conventional prefix — return unchanged.
		return msg
	}

	// matches[1] is the message body after the prefix.
	humanSubject := capitalise(strings.TrimSpace(matches[1]))

	if len(lines) == 1 {
		return humanSubject
	}
	return humanSubject + "\n" + lines[1]
}

// capitalise returns s with its first rune upper-cased. If s is empty it is
// returned unchanged.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
