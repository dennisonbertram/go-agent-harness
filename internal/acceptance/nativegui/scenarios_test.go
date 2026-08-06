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
	root := t.TempDir()
	path := filepath.Join(root, "fake-turns.json")
	if err := WriteFakeProviderTurns(root, "fake-turns.json", manifest); err != nil {
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
	if len(turns) != 9 {
		t.Fatalf("turn count = %d, want 9", len(turns))
	}
}

func TestFakeProviderScenarioManifestPreflightValidatesScenarioContracts(t *testing.T) {
	manifest := DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32))
	if err := ValidateFakeProviderScenarioManifest(manifest); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*FakeProviderScenarioManifest){
		func(m *FakeProviderScenarioManifest) { m.Scenarios[0].Turns[0].ToolCalls[0].Name = "read" },
		func(m *FakeProviderScenarioManifest) { m.Scenarios[0].Turns[2].Content = "second message" },
		func(m *FakeProviderScenarioManifest) {
			m.Scenarios[1].Turns[0].ToolCalls[0].Arguments = `{"execution_type":"shell","command":"true"}`
		},
		func(m *FakeProviderScenarioManifest) { m.Scenarios[1].Turns[2].Content = "cron result" },
		func(m *FakeProviderScenarioManifest) {
			m.Scenarios[2].Turns[0].ToolCalls[0].Arguments = `{"delay":"1s","prompt":"later"}`
		},
		func(m *FakeProviderScenarioManifest) { m.Scenarios[2].Turns[2].Content = "callback continuation" },
	} {
		copy := DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32))
		mutate(&copy)
		if err := ValidateFakeProviderScenarioManifest(copy); err == nil {
			t.Fatal("expected contract rejection")
		}
	}
}

func TestWriteFakeProviderTurnsRejectsTraversalAndSymlinkEscape(t *testing.T) {
	manifest := DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32))
	root := t.TempDir()
	if err := WriteFakeProviderTurns(root, "../escape.json", manifest); err == nil {
		t.Fatal("expected traversal rejection")
	}
	outside := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFakeProviderTurns(root, "linked/escape.json", manifest); err == nil {
		t.Fatal("expected symlink escape rejection")
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
		func(m *FakeProviderScenarioManifest) { m.Scenarios[0].Artifacts[0].Path = "artifacts/../escape.png" },
	} {
		manifest := DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32))
		mutate(&manifest)
		if err := ValidateFakeProviderScenarioManifest(manifest); err == nil {
			t.Fatal("expected preflight rejection")
		}
	}
}
