package nativegui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-agent-harness/internal/acceptance/inventory"
)

const (
	ScenarioCoreToolTwoMessage   = "native.core-tool.two-message"
	ScenarioCronConversation     = "native.cron.linked-conversation"
	ScenarioCallbackConversation = "native.callback.linked-conversation"
)

// FakeProviderScenarioManifest is the fixed, reviewable input to a future
// owned native driver. It is not evidence and cannot be rendered as a PASS.
// The driver must resolve every marker from its own run and conversation IDs.
type FakeProviderScenarioManifest struct {
	Nonce     string                 `json:"nonce"`
	Scenarios []FakeProviderScenario `json:"scenarios"`
}

type FakeProviderScenario struct {
	ID          string                 `json:"id"`
	Turns       []FakeProviderTurn     `json:"turns"`
	Artifacts   []ScenarioArtifactPlan `json:"artifacts"`
	Correlation ScenarioCorrelation    `json:"correlation"`
}

type FakeProviderTurn struct {
	Content   string                 `json:"content"`
	ToolCalls []FakeProviderToolCall `json:"tool_calls,omitempty"`
}

type FakeProviderToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ScenarioArtifactPlan struct {
	Kind inventory.ArtifactKind `json:"kind"`
	Path string                 `json:"path"`
}

// ScenarioCorrelation requires every artifact generated for one scenario to
// carry the same launcher nonce and the IDs discovered by that scenario's
// independent SSE/API probes. Markers are deliberately placeholders: static
// fixture code cannot fabricate runtime identity evidence.
type ScenarioCorrelation struct {
	Nonce                string `json:"nonce"`
	RunIDMarker          string `json:"run_id_marker"`
	ConversationIDMarker string `json:"conversation_id_marker"`
}

func DefaultFakeProviderScenarioManifest(nonce string) FakeProviderScenarioManifest {
	nameNonce := nonce
	if len(nameNonce) > 8 {
		nameNonce = nameNonce[:8]
	}
	return FakeProviderScenarioManifest{Nonce: nonce, Scenarios: []FakeProviderScenario{
		{
			ID: ScenarioCoreToolTwoMessage,
			Turns: []FakeProviderTurn{
				{ToolCalls: []FakeProviderToolCall{{ID: "native-core-tool", Name: "ls", Arguments: `{"path":"."}`}}},
				{Content: "Native core tool result recorded for " + nonce + "."},
				{Content: "Native second message continues the same conversation for " + nonce + "."},
			},
			Artifacts:   scenarioArtifacts(nonce, ScenarioCoreToolTwoMessage),
			Correlation: scenarioCorrelation(nonce, "core-tool"),
		},
		{
			ID: ScenarioCronConversation,
			Turns: []FakeProviderTurn{
				{ToolCalls: []FakeProviderToolCall{{ID: "native-cron-create", Name: "cron_create", Arguments: fmt.Sprintf(`{"name":"native-cron-%s","schedule":"*/1 * * * *","execution_type":"harness","prompt":"Continue native cron conversation %s"}`, nameNonce, nonce)}}},
				{Content: "Native cron schedule recorded for " + nonce + "."},
				{Content: "Native cron continuation recorded for " + nonce + "."},
			},
			Artifacts:   scenarioArtifacts(nonce, ScenarioCronConversation),
			Correlation: scenarioCorrelation(nonce, "cron"),
		},
		{
			ID: ScenarioCallbackConversation,
			Turns: []FakeProviderTurn{
				{ToolCalls: []FakeProviderToolCall{{ID: "native-callback-create", Name: "set_delayed_callback", Arguments: fmt.Sprintf(`{"delay":"5s","prompt":"Continue native callback conversation %s"}`, nonce)}}},
				{Content: "Native callback due state recorded for " + nonce + "."},
				{Content: "Native callback continuation recorded exactly once for " + nonce + "."},
			},
			Artifacts:   scenarioArtifacts(nonce, ScenarioCallbackConversation),
			Correlation: scenarioCorrelation(nonce, "callback"),
		},
	}}
}

func scenarioArtifacts(nonce, scenarioID string) []ScenarioArtifactPlan {
	prefix := filepath.ToSlash(filepath.Join("artifacts", nonce, scenarioID))
	return []ScenarioArtifactPlan{
		{Kind: inventory.ArtifactScreenshot, Path: prefix + "/screen.png"},
		{Kind: inventory.ArtifactAXSnapshot, Path: prefix + "/accessibility.json"},
		{Kind: inventory.ArtifactRawSSEEvent, Path: prefix + "/events.sse"},
		{Kind: inventory.ArtifactAPIStoreProbe, Path: prefix + "/store.json"},
	}
}

func scenarioCorrelation(nonce, name string) ScenarioCorrelation {
	return ScenarioCorrelation{Nonce: nonce, RunIDMarker: "{{run_id:" + name + "}}", ConversationIDMarker: "{{conversation_id:" + name + "}}"}
}

// ValidateFakeProviderScenarioManifest is a zero-effect preflight. It checks
// fixture completeness before the #1206 owner is allowed to create a root,
// reserve a port, or spawn the app and daemon.
func ValidateFakeProviderScenarioManifest(manifest FakeProviderScenarioManifest) error {
	if len(strings.TrimSpace(manifest.Nonce)) < 32 {
		return fmt.Errorf("native scenario manifest requires a launcher nonce of at least 32 characters")
	}
	required := map[string]struct{}{
		ScenarioCoreToolTwoMessage: {}, ScenarioCronConversation: {}, ScenarioCallbackConversation: {},
	}
	seenIDs := make(map[string]struct{}, len(manifest.Scenarios))
	seenConversationMarkers := make(map[string]struct{}, len(manifest.Scenarios))
	for _, scenario := range manifest.Scenarios {
		if _, expected := required[scenario.ID]; !expected {
			return fmt.Errorf("native scenario manifest has unknown scenario %q", scenario.ID)
		}
		if _, duplicate := seenIDs[scenario.ID]; duplicate {
			return fmt.Errorf("native scenario manifest has duplicate scenario %q", scenario.ID)
		}
		seenIDs[scenario.ID] = struct{}{}
		if err := validateScenario(scenario, manifest.Nonce); err != nil {
			return err
		}
		if _, duplicate := seenConversationMarkers[scenario.Correlation.ConversationIDMarker]; duplicate {
			return fmt.Errorf("native scenario manifest reuses conversation correlation marker %q", scenario.Correlation.ConversationIDMarker)
		}
		seenConversationMarkers[scenario.Correlation.ConversationIDMarker] = struct{}{}
	}
	if len(seenIDs) != len(required) {
		missing := make([]string, 0, len(required)-len(seenIDs))
		for id := range required {
			if _, found := seenIDs[id]; !found {
				missing = append(missing, id)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("native scenario manifest is missing required scenario %q", missing[0])
	}
	return nil
}

func validateScenario(scenario FakeProviderScenario, nonce string) error {
	if len(scenario.Turns) < 3 {
		return fmt.Errorf("native scenario %q requires deterministic setup, terminal, and continuation fake turns", scenario.ID)
	}
	correlation := scenario.Correlation
	if correlation.Nonce != nonce || strings.TrimSpace(correlation.RunIDMarker) == "" || strings.TrimSpace(correlation.ConversationIDMarker) == "" {
		return fmt.Errorf("native scenario %q has incomplete nonce/run/conversation correlation", scenario.ID)
	}
	requiredKinds := map[inventory.ArtifactKind]struct{}{
		inventory.ArtifactScreenshot: {}, inventory.ArtifactAXSnapshot: {}, inventory.ArtifactRawSSEEvent: {}, inventory.ArtifactAPIStoreProbe: {},
	}
	seenKinds := make(map[inventory.ArtifactKind]struct{}, len(scenario.Artifacts))
	seenPaths := make(map[string]struct{}, len(scenario.Artifacts))
	for _, artifact := range scenario.Artifacts {
		if _, required := requiredKinds[artifact.Kind]; !required {
			return fmt.Errorf("native scenario %q has unsupported artifact kind %q", scenario.ID, artifact.Kind)
		}
		if !strings.HasPrefix(artifact.Path, "artifacts/"+nonce+"/"+scenario.ID+"/") {
			return fmt.Errorf("native scenario %q artifact %q is not nonce/scenario scoped", scenario.ID, artifact.Path)
		}
		if _, duplicate := seenKinds[artifact.Kind]; duplicate {
			return fmt.Errorf("native scenario %q duplicates artifact kind %q", scenario.ID, artifact.Kind)
		}
		if _, duplicate := seenPaths[artifact.Path]; duplicate {
			return fmt.Errorf("native scenario %q duplicates artifact path %q", scenario.ID, artifact.Path)
		}
		seenKinds[artifact.Kind] = struct{}{}
		seenPaths[artifact.Path] = struct{}{}
	}
	if len(seenKinds) != len(requiredKinds) {
		return fmt.Errorf("native scenario %q lacks one or more required typed artifacts", scenario.ID)
	}
	return nil
}

// WriteFakeProviderTurns writes only the flat turn list understood by
// harnessd's HARNESS_FAKE_TURNS loader. It is intentionally separate from the
// later GUI driver and does not start a process or create evidence.
func WriteFakeProviderTurns(path string, manifest FakeProviderScenarioManifest) error {
	if err := ValidateFakeProviderScenarioManifest(manifest); err != nil {
		return err
	}
	var turns []FakeProviderTurn
	for _, scenario := range manifest.Scenarios {
		turns = append(turns, scenario.Turns...)
	}
	data, err := json.Marshal(turns)
	if err != nil {
		return fmt.Errorf("marshal native fake-provider turns: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write native fake-provider turns: %w", err)
	}
	return nil
}
