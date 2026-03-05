package tools

import (
	"context"
	"time"
)

func ValidateWorkspaceRelativePattern(pattern string) error {
	return validateWorkspaceRelativePattern(pattern)
}

func BuildLineMatcher(query string, useRegex bool, caseSensitive bool) (func(string) bool, error) {
	return buildLineMatcher(query, useRegex, caseSensitive)
}

func RunCommand(ctx context.Context, timeout time.Duration, command string, args ...string) (string, int, bool, error) {
	return runCommand(ctx, timeout, command, args...)
}

func IsDangerousCommand(command string) bool {
	return isDangerousCommand(command)
}
