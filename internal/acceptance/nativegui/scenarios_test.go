package nativegui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-agent-harness/internal/acceptance/inventory"
)

func TestOwnedFakeProviderScenarioManifestPreflight(t *testing.T) {
	manifest := DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32))
	if err := ValidateFakeProviderScenarioManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if got, want := len(manifest.Scenarios), 3; got != want {
		t.Fatalf("scenario count = %d, want %d", got, want)
	}
	for _, scenario := range manifest.Scenarios {
		seen := map[inventory.ArtifactKind]string{}
		for _, artifact := range scenario.Artifacts {
			if previous, duplicate := seen[artifact.Kind]; duplicate || artifact.Path == previous {
				t.Fatalf("%s does not retain one distinct path per required artifact: %#v", scenario.ID, scenario.Artifacts)
			}
			seen[artifact.Kind] = artifact.Path
		}
		for _, kind := range []inventory.ArtifactKind{
			inventory.ArtifactScreenshot,
			inventory.ArtifactAXSnapshot,
			inventory.ArtifactRawSSEEvent,
			inventory.ArtifactAPIStoreProbe,
		} {
			if seen[kind] == "" {
				t.Fatalf("%s lacks %s", scenario.ID, kind)
			}
		}
		if scenario.Correlation.Nonce != manifest.Nonce || scenario.Correlation.RunIDMarker == "" || scenario.Correlation.ConversationIDMarker == "" {
			t.Fatalf("%s lacks nonce/run/conversation correlation: %#v", scenario.ID, scenario.Correlation)
		}
	}
}

func TestWriteFakeProviderTurnsFlattensOnlyPreflightedFixtures(t *testing.T) {
	manifest := DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32))
	path := filepath.Join(t.TempDir(), "fake-turns.json")
	if err := WriteFakeProviderTurns(path, manifest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var turns []FakeProviderTurn
	if err := json.Unmarshal(data, &turns); err != nil {
		t.Fatal(err)
	}
	if got, want := len(turns), 9; got != want {
		t.Fatalf("turn count = %d, want %d", got, want)
	}
	if turns[0].ToolCalls[0].Name != "ls" || turns[3].ToolCalls[0].Name != "cron_create" || turns[6].ToolCalls[0].Name != "set_delayed_callback" {
		t.Fatalf("fixture tool ordering = %#v", turns)
	}
}

func TestFakeProviderScenarioManifestPreflightRejectsIncompleteOrAmbiguousEvidence(t *testing.T) {
	for _, mutate := range []func(*FakeProviderScenarioManifest){
		func(m *FakeProviderScenarioManifest) { m.Scenarios[0].Artifacts = m.Scenarios[0].Artifacts[:3] },
		func(m *FakeProviderScenarioManifest) {
			m.Scenarios[0].Artifacts[1].Path = m.Scenarios[0].Artifacts[0].Path
		},
		func(m *FakeProviderScenarioManifest) {
			m.Scenarios[1].Correlation.ConversationIDMarker = m.Scenarios[0].Correlation.ConversationIDMarker
		},
	} {
		manifest := DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32))
		mutate(&manifest)
		if err := ValidateFakeProviderScenarioManifest(manifest); err == nil {
			t.Fatal("expected preflight rejection")
		}
	}
}
