package main

import (
	"os"
	"strings"
	"testing"
)

func TestTUI001_DependenciesPinned(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	required := []string{
		"github.com/charmbracelet/bubbletea",
		"github.com/charmbracelet/lipgloss",
		"github.com/charmbracelet/glamour",
		"github.com/charmbracelet/x/exp/teatest",
	}
	for _, dep := range required {
		if !strings.Contains(content, dep) {
			t.Errorf("go.mod missing required dependency: %s", dep)
		}
	}
}
