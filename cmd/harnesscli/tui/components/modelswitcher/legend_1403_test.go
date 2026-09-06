package modelswitcher_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

// Issue #1403: the picker must explain its markers and put usable models first.

func TestModelSwitcher_FooterLegend(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").Open().WithAvailability(func(p string) bool { return p == "openai" })
	view := m.View(120)
	if !strings.Contains(view, "● ready") || !strings.Contains(view, "○ needs API key") {
		t.Fatalf("provider list footer must carry a legend for the markers, got:\n%s", view)
	}
}

func TestModelSwitcher_SearchReadyFirst(t *testing.T) {
	ready := func(p string) bool { return p == "openai" }
	m := modelswitcher.New("gpt-4.1-mini").Open().WithAvailability(ready).WithKeyStatus(ready)
	m = m.EnterSearch().SetSearch("e")
	lines := strings.Split(m.View(120), "\n")
	lastReady, firstUnavailable := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "● ready") { // legend line, not a result row
			continue
		}
		if strings.Contains(l, "(unavailable)") && firstUnavailable == -1 {
			firstUnavailable = i
		}
		if strings.Contains(l, "●") && !strings.Contains(l, "(unavailable)") {
			lastReady = i
		}
	}
	if firstUnavailable == -1 || lastReady == -1 {
		t.Skipf("test needs both ready and unavailable results (ready=%d unavailable=%d)", lastReady, firstUnavailable)
	}
	if firstUnavailable < lastReady {
		t.Fatalf("ready models must be listed before unavailable ones (first unavailable at line %d, last ready at %d):\n%s", firstUnavailable, lastReady, m.View(120))
	}
}

func TestModelSwitcher_ProviderOrderCaseInsensitive(t *testing.T) {
	m := modelswitcher.New("gpt-4.1-mini").WithModels([]modelswitcher.ServerModelEntry{
		{ID: "a/x", Provider: "xai"},
		{ID: "b/y", Provider: "cerebras"},
		{ID: "c/z", Provider: "openai"},
	})
	var labels []string
	for _, p := range m.Providers() {
		labels = append(labels, p.Label)
	}
	got := strings.Join(labels, ",")
	if got != "cerebras,OpenAI,xAI" {
		t.Fatalf("providers must sort case-insensitively, got %s", got)
	}
}
