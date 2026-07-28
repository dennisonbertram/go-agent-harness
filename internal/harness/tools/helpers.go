package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

func ValidateWorkspaceRelativePattern(pattern string) error {
	return validateWorkspaceRelativePattern(pattern)
}

// BuildLineMatcher compiles a search query into a per-line predicate, shared by
// the grep-style tools in tools/core and tools/deferred. The implementation
// lives here rather than behind an unexported wrapper because its previous home
// was one of the duplicated tool files removed by the single-catalog
// consolidation.
func BuildLineMatcher(query string, useRegex bool, caseSensitive bool) (func(string) bool, error) {
	if useRegex {
		pattern := query
		if !caseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile regex: %w", err)
		}
		return re.MatchString, nil
	}
	if caseSensitive {
		return func(line string) bool { return strings.Contains(line, query) }, nil
	}
	needle := strings.ToLower(query)
	return func(line string) bool { return strings.Contains(strings.ToLower(line), needle) }, nil
}

func RunCommand(ctx context.Context, timeout time.Duration, command string, args ...string) (string, int, bool, error) {
	return runCommand(ctx, timeout, command, args...)
}

func IsDangerousCommand(command string) bool {
	return isDangerousCommand(command)
}
