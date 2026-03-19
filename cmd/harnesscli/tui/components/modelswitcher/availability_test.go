package modelswitcher_test

import (
	"strings"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/modelswitcher"
)

// ─── Issue #313: ModelEntry.Available field ───────────────────────────────────

// TestTUI313_ModelEntryAvailableField verifies ModelEntry has an Available field.
func TestTUI313_ModelEntryAvailableField(t *testing.T) {
	entry := modelswitcher.ModelEntry{
		ID:          "gpt-4.1",
		DisplayName: "GPT-4.1",
		Provider:    "openai",
		Available:   true,
	}
	if !entry.Available {
		t.Error("ModelEntry.Available should be settable to true")
	}

	entry2 := modelswitcher.ModelEntry{
		ID:          "deepseek-chat",
		DisplayName: "DeepSeek Chat",
		Provider:    "deepseek",
		Available:   false,
	}
	if entry2.Available {
		t.Error("ModelEntry.Available should be settable to false")
	}
}

// TestTUI313_DefaultModelsAvailableUnset verifies DefaultModels do not have Available set
// (zero value = false, representing "unknown" until provider data arrives).
func TestTUI313_DefaultModelsAvailableUnset(t *testing.T) {
	for _, dm := range modelswitcher.DefaultModels {
		if dm.Available {
			t.Errorf("DefaultModels entry %q should have Available=false (zero value)", dm.ID)
		}
	}
}

// TestTUI313_WithAvailabilityMarksModels verifies WithAvailability sets Available on each
// ModelEntry based on the provided provider-configured function.
func TestTUI313_WithAvailabilityMarksModels(t *testing.T) {
	m := modelswitcher.New("gpt-4.1")

	// Only openai is configured.
	m2 := m.WithAvailability(func(provider string) bool {
		return provider == "openai"
	})

	for _, e := range m2.Models {
		if e.Provider == "openai" {
			if !e.Available {
				t.Errorf("model %q (openai) should be Available=true", e.ID)
			}
		} else {
			if e.Available {
				t.Errorf("model %q (%s) should be Available=false", e.ID, e.Provider)
			}
		}
	}
}

// TestTUI313_WithAvailabilityNilFnLeavesAllUnavailable verifies that passing nil fn
// leaves all models with Available=false.
func TestTUI313_WithAvailabilityNilFnLeavesAllUnavailable(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").WithAvailability(nil)
	for _, e := range m.Models {
		if e.Available {
			t.Errorf("model %q should have Available=false when fn is nil", e.ID)
		}
	}
}

// TestTUI313_WithAvailabilityDoesNotMutateOriginal verifies value semantics.
func TestTUI313_WithAvailabilityDoesNotMutateOriginal(t *testing.T) {
	m1 := modelswitcher.New("gpt-4.1")
	_ = m1.WithAvailability(func(string) bool { return true })
	// m1.Models should all still have Available=false.
	for _, e := range m1.Models {
		if e.Available {
			t.Errorf("WithAvailability must not mutate original; %q has Available=true", e.ID)
		}
	}
}

// TestTUI313_WithAvailabilityAllConfigured verifies all models marked available when
// all providers are configured.
func TestTUI313_WithAvailabilityAllConfigured(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").WithAvailability(func(string) bool { return true })
	for _, e := range m.Models {
		if !e.Available {
			t.Errorf("model %q should be Available=true when all providers configured", e.ID)
		}
	}
}

// TestTUI313_WithAvailabilityNoneConfigured verifies all models unavailable when
// no providers are configured.
func TestTUI313_WithAvailabilityNoneConfigured(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").WithAvailability(func(string) bool { return false })
	for _, e := range m.Models {
		if e.Available {
			t.Errorf("model %q should be Available=false when no providers configured", e.ID)
		}
	}
}

// TestTUI313_WithModelsPreservesAvailability verifies that WithModels called after
// WithAvailability preserves availability for server-fetched entries via the fn re-application.
func TestTUI313_WithModelsPreservesAvailability(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").WithAvailability(func(provider string) bool {
		return provider == "openai"
	})
	serverModels := []modelswitcher.ServerModelEntry{
		{ID: "gpt-4.1", Provider: "openai"},
		{ID: "deepseek-chat", Provider: "deepseek"},
	}
	m2 := m.WithModels(serverModels)

	for _, e := range m2.Models {
		switch e.Provider {
		case "openai":
			if !e.Available {
				t.Errorf("model %q (openai) should be Available=true after WithModels", e.ID)
			}
		case "deepseek":
			if e.Available {
				t.Errorf("model %q (deepseek) should be Available=false after WithModels", e.ID)
			}
		}
	}
}

// ─── View rendering tests for availability ────────────────────────────────────

// TestTUI313_ViewAvailableModelNotDimmed verifies that an available model renders
// WITHOUT an unavailability marker.
func TestTUI313_ViewAvailableModelNotDimmed(t *testing.T) {
	m := modelswitcher.New("gpt-4.1").Open().WithAvailability(func(string) bool { return true })
	v := m.View(80)

	// Strip ANSI escapes for clean string matching.
	plain := stripANSI(v)

	// Should NOT contain any unavailability indicator.
	if strings.Contains(plain, "(unavailable)") {
		t.Errorf("view should not show '(unavailable)' when all models are available:\n%s", plain)
	}
}

// TestTUI313_ViewUnavailableModelShowsIndicator verifies that an unavailable model
// renders with a visual indicator (e.g., "(unavailable)") when at level 1.
func TestTUI313_ViewUnavailableModelShowsIndicator(t *testing.T) {
	// Only openai is configured; deepseek is not.
	m := modelswitcher.New("gpt-4.1").Open().WithAvailability(func(provider string) bool {
		return provider == "openai"
	})
	// Drill into DeepSeek provider to see unavailability indicator on models.
	provs := m.Providers()
	for i := range provs {
		if provs[i].Label == "DeepSeek" {
			for m.ProviderCursorIndex() != i {
				m = m.ProviderDown()
			}
			break
		}
	}
	m2 := m.DrillIntoProvider()
	v := m2.View(80)
	plain := stripANSI(v)

	// DeepSeek models should show unavailability indicator at level 1.
	if !strings.Contains(plain, "(unavailable)") {
		t.Errorf("view should show '(unavailable)' for unconfigured provider models at level 1:\n%s", plain)
	}
}

// TestTUI313_ViewUnavailableModelStillPresent verifies that unavailable models are
// still present in the list at level 1 (not hidden).
func TestTUI313_ViewUnavailableModelStillPresent(t *testing.T) {
	// Only openai is configured.
	m := modelswitcher.New("gpt-4.1").Open().WithAvailability(func(provider string) bool {
		return provider == "openai"
	})
	// For each provider, drill in and verify all models are still visible.
	provs := m.Providers()
	for i, p := range provs {
		m2 := m
		for m2.ProviderCursorIndex() != i {
			m2 = m2.ProviderDown()
		}
		m3 := m2.DrillIntoProvider()
		v := m3.View(80)
		for _, dm := range modelswitcher.DefaultModels {
			if dm.ProviderLabel == p.Label {
				if !strings.Contains(v, dm.DisplayName) {
					t.Errorf("view should still show %q even when unavailable:\n%s", dm.DisplayName, v)
				}
			}
		}
	}
}

// TestTUI313_ViewNoAvailabilitySetShowsNoDimming verifies that when WithAvailability
// is never called (zero value), no unavailability indicators appear (backwards compat).
func TestTUI313_ViewNoAvailabilitySetShowsNoDimming(t *testing.T) {
	// Default New() without WithAvailability call.
	m := modelswitcher.New("gpt-4.1").Open()
	v := m.View(80)
	plain := stripANSI(v)

	if strings.Contains(plain, "(unavailable)") {
		t.Errorf("view should not show '(unavailable)' when no availability info set:\n%s", plain)
	}
}

// TestTUI313_ViewSelectedUnavailableModelHighlightedCorrectly verifies that a selected
// but unavailable model still gets the highlight style (cursor on it).
func TestTUI313_ViewSelectedUnavailableModelHighlightedCorrectly(t *testing.T) {
	// Navigate to a deepseek model.
	m := modelswitcher.New("gpt-4.1").Open().WithAvailability(func(provider string) bool {
		return provider == "openai"
	})
	// Navigate down until we hit a non-openai model.
	for i := 0; i < 5; i++ {
		m = m.SelectDown()
	}
	v := m.View(80)
	// Should not panic and should contain the ">" cursor indicator.
	if !strings.Contains(v, ">") {
		t.Errorf("selected row should show '>' cursor:\n%s", v)
	}
}

// TestTUI313_WithAvailabilityPreservesExistingFields verifies that WithAvailability
// does not clobber other ModelEntry fields.
func TestTUI313_WithAvailabilityPreservesExistingFields(t *testing.T) {
	m := modelswitcher.New("deepseek-reasoner").WithAvailability(func(string) bool { return true })
	for _, e := range m.Models {
		if e.ID == "deepseek-reasoner" {
			if !e.ReasoningMode {
				t.Error("WithAvailability should not clear ReasoningMode on deepseek-reasoner")
			}
			if e.Provider != "deepseek" {
				t.Errorf("WithAvailability should not change Provider; got %q", e.Provider)
			}
			return
		}
	}
	t.Fatal("deepseek-reasoner not found in models")
}

// stripANSI removes ANSI escape sequences for plain-text assertions.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEsc = false
			}
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}
