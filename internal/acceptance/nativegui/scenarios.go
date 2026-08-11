package nativegui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-agent-harness/internal/acceptance/inventory"
	"go-agent-harness/internal/cron"
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
	if err := validateScenarioTurns(scenario, nonce); err != nil {
		return err
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
		if !safeRelativePath(artifact.Path) || !strings.HasPrefix(artifact.Path, "artifacts/"+nonce+"/"+scenario.ID+"/") {
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

func validateScenarioTurns(scenario FakeProviderScenario, nonce string) error {
	if len(scenario.Turns) != 3 || len(scenario.Turns[0].ToolCalls) != 1 {
		return fmt.Errorf("native scenario %q requires one setup tool call plus terminal and continuation turns", scenario.ID)
	}
	call := scenario.Turns[0].ToolCalls[0]
	switch scenario.ID {
	case ScenarioCoreToolTwoMessage:
		var args struct {
			Path string `json:"path"`
		}
		if call.Name != "ls" || json.Unmarshal([]byte(call.Arguments), &args) != nil || args.Path != "." {
			return fmt.Errorf("native core-tool scenario requires ls of the isolated workspace")
		}
		if !strings.Contains(scenario.Turns[1].Content, "tool result") || !strings.Contains(scenario.Turns[2].Content, "same conversation") {
			return fmt.Errorf("native core-tool scenario requires tool result and same-conversation continuation")
		}
	case ScenarioCronConversation:
		var args struct {
			Name          string `json:"name"`
			Schedule      string `json:"schedule"`
			ExecutionType string `json:"execution_type"`
			Prompt        string `json:"prompt"`
			Command       string `json:"command"`
		}
		if call.Name != "cron_create" || json.Unmarshal([]byte(call.Arguments), &args) != nil || args.Name == "" || args.Schedule == "" || args.ExecutionType != "harness" || args.Prompt == "" || args.Command != "" || !strings.Contains(args.Prompt, nonce) {
			return fmt.Errorf("native cron scenario requires a scoped harness cron continuation")
		}
		if _, err := cron.NextRunTime(args.Schedule, time.Now()); err != nil {
			return fmt.Errorf("native cron scenario requires a cron_create-valid schedule: %w", err)
		}
		if !strings.Contains(scenario.Turns[1].Content, "schedule") || !strings.Contains(scenario.Turns[2].Content, "continuation") {
			return fmt.Errorf("native cron scenario requires schedule and linked continuation turns")
		}
	case ScenarioCallbackConversation:
		var args struct{ Delay, Prompt string }
		if call.Name != "set_delayed_callback" || json.Unmarshal([]byte(call.Arguments), &args) != nil || args.Delay != "5s" || args.Prompt == "" || !strings.Contains(args.Prompt, nonce) {
			return fmt.Errorf("native callback scenario requires the reviewed 5s delayed continuation")
		}
		if _, err := time.ParseDuration(args.Delay); err != nil || !strings.Contains(scenario.Turns[1].Content, "due state") || !strings.Contains(scenario.Turns[2].Content, "continuation") || !strings.Contains(scenario.Turns[2].Content, "exactly once") {
			return fmt.Errorf("native callback scenario requires due, linked continuation, and exactly-once turns")
		}
	}
	return nil
}

// WriteFakeProviderTurns writes only the flat turn list understood by
// harnessd's HARNESS_FAKE_TURNS loader. It is intentionally separate from the
// later GUI driver and does not start a process or create evidence.
func WriteFakeProviderTurns(root, relativePath string, manifest FakeProviderScenarioManifest) error {
	if err := ValidateFakeProviderScenarioManifest(manifest); err != nil {
		return err
	}
	path, err := containedWritePath(root, relativePath)
	if err != nil {
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

// containedWritePath resolves the parent under a canonical private root and
// rejects both lexical traversal and any existing symlink. This is the write
// boundary for the scenario fixture, not merely an advisory manifest check.
func containedWritePath(root, relativePath string) (string, error) {
	if !safeRelativePath(relativePath) {
		return "", fmt.Errorf("native scenario artifact path must be a safe relative path")
	}
	canonicalRoot, err := canonicalDirectory(root, "native scenario root")
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(canonicalRoot, filepath.FromSlash(relativePath))
	parent := filepath.Dir(candidate)
	canonicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("resolve native scenario artifact parent: %w", err)
	}
	canonicalParent = filepath.Clean(canonicalParent)
	if !contained(canonicalRoot, canonicalParent) {
		return "", fmt.Errorf("native scenario artifact path escapes private root")
	}
	if info, err := os.Lstat(candidate); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("native scenario artifact path must not be a symlink")
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("lstat native scenario artifact path: %w", err)
	}
	return candidate, nil
}

func safeRelativePath(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	return clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}
