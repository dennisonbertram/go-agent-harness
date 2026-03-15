package tui

// deps_test.go pins test-only TUI dependencies as direct requires in go.mod.
// This blank import ensures go mod tidy does not prune teatest.
// It will be replaced by real test imports as TUI-003+ land.

import (
	_ "github.com/charmbracelet/x/exp/teatest"
)
