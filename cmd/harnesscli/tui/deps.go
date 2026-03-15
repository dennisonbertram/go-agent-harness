package tui

// deps.go pins TUI dependencies as direct requires in go.mod.
// These blank imports ensure go mod tidy does not prune them.
// They will be replaced by real imports as TUI-003+ land.

import (
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/lipgloss"
)
