package inventory_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/internal/acceptance/inventory"
	"go-agent-harness/internal/harness"
	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/server"
)

func TestCompileDerivesResolvedToolAndBuiltinCommandInventory(t *testing.T) {
	registry := harness.NewDefaultRegistry(t.TempDir())
	t.Cleanup(func() { _ = registry.Shutdown(t.Context()) })

	compiled, err := inventory.Compile(inventory.Input{
		Tools:    registry.DefinitionsWithMetadata(),
		Commands: tui.NewCommandRegistry().All(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.SchemaVersion != inventory.SchemaVersion {
		t.Fatalf("schema version = %q, want %q", compiled.SchemaVersion, inventory.SchemaVersion)
	}
	if compiled.Hash == "" {
		t.Fatal("compiled inventory hash must be present")
	}
	assertItem(t, compiled, "tool:read", inventory.Available, "harness.default.core", "built-in runtime registry")
	assertItem(t, compiled, "tool:create_prompt_extension", inventory.Available, "harness.default.deferred", "built-in runtime registry")
	resume := assertItem(t, compiled, "tui_command:resume", inventory.Available, "harnesscli.tui.builtin", "built-in")
	if len(resume.Aliases) != 1 || resume.Aliases[0] != "continue" {
		t.Fatalf("resume aliases = %#v, want [continue]", resume.Aliases)
	}

	again, err := inventory.Compile(inventory.Input{Tools: registry.DefinitionsWithMetadata(), Commands: tui.NewCommandRegistry().All()})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Hash != again.Hash {
		t.Fatalf("inventory hash is non-deterministic: %q != %q", compiled.Hash, again.Hash)
	}
	if report := inventory.RenderMarkdown(compiled); report == "" || !contains(report, compiled.Hash) || !contains(report, "tool:read") {
		t.Fatalf("report does not identify the generated inventory: %q", report)
	}
}

func TestCompiledInventoryReconcilesToolsHTTPBoundary(t *testing.T) {
	registry := harness.NewDefaultRegistry(t.TempDir())
	t.Cleanup(func() { _ = registry.Shutdown(t.Context()) })
	compiled, err := inventory.Compile(inventory.Input{Tools: registry.DefinitionsWithMetadata(), Commands: tui.NewCommandRegistry().All()})
	if err != nil {
		t.Fatal(err)
	}

	h := server.NewWithOptions(server.ServerOptions{AuthDisabled: true, Tools: registry})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tools", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/v1/tools status = %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Tools []struct {
			Name      string `json:"name"`
			Owner     string `json:"owner"`
			Condition string `json:"condition"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	fromHTTP := make(map[string]struct {
		owner     string
		condition string
	}, len(response.Tools))
	for _, tool := range response.Tools {
		fromHTTP[tool.Name] = struct {
			owner     string
			condition string
		}{tool.Owner, tool.Condition}
	}
	fromInventory := make(map[string]struct {
		owner     string
		condition string
	})
	for _, item := range compiled.Items {
		if item.Kind == inventory.ToolKind && item.Availability == inventory.Available {
			fromInventory[item.Name] = struct {
				owner     string
				condition string
			}{item.Owner, item.Condition}
		}
	}
	if len(fromHTTP) != len(fromInventory) {
		t.Fatalf("/v1/tools has %d names; compiled runtime inventory has %d", len(fromHTTP), len(fromInventory))
	}
	for name, httpMeta := range fromHTTP {
		inventoryMeta, found := fromInventory[name]
		if !found {
			t.Fatalf("/v1/tools item %q missing from compiled inventory", name)
		}
		if httpMeta != inventoryMeta {
			t.Fatalf("/v1/tools item %q provenance = %#v; compiled inventory = %#v", name, httpMeta, inventoryMeta)
		}
	}
}

func TestCompileCarriesExplicitUnavailableToolsetObservation(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{
		Unavailable: []inventory.ResolverObservation{{
			Kind:       inventory.ToolsetKind,
			Name:       "mcp:calendar",
			Owner:      "harness.mcp",
			Condition:  "MCP server calendar is connected",
			Reason:     "mcp_server_not_connected",
			Provenance: inventory.ResolverProvenance{Source: "runtime.mcp_registry", Provider: "calendar"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	item := assertItem(t, compiled, "toolset:mcp:calendar", inventory.NotApplicable, "harness.mcp", "MCP server calendar is connected")
	if item.Reason != "mcp_server_not_connected" {
		t.Fatalf("reason = %q", item.Reason)
	}
}

func TestCompileRejectsDuplicateOrReasonlessUnavailableIdentity(t *testing.T) {
	_, err := inventory.Compile(inventory.Input{
		Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "same"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}},
		Unavailable: []inventory.ResolverObservation{{
			Kind: inventory.ToolKind, Name: "same", Owner: "harness.runtime.core", Condition: "disabled",
			Provenance: inventory.ResolverProvenance{Source: "runtime.test", Provider: "test", IndividualNamesKnown: true},
		}},
	})
	if err == nil {
		t.Fatal("Compile accepted duplicate/reasonless unavailable capability")
	}
}

func TestCompileRejectsGenericRegistryProvenanceAfterWhitespaceNormalization(t *testing.T) {
	_, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{
		Definition: harness.ToolDefinition{Name: "direct"},
		Tier:       tools.TierCore,
		Owner:      "  harness.registry  ",
		Condition:  "  direct Register call  ",
	}}})
	if err == nil || !contains(err.Error(), "missing authoritative owner or condition provenance") {
		t.Fatalf("Compile error = %v, want generic provenance rejection", err)
	}
}

func TestCompileRejectsUnprovenIndividualDynamicToolName(t *testing.T) {
	_, err := inventory.Compile(inventory.Input{Unavailable: []inventory.ResolverObservation{{
		Kind: inventory.ToolKind, Name: "calendar.create", Owner: "harness.mcp", Condition: "calendar configured", Reason: "server unavailable",
		Provenance: inventory.ResolverProvenance{Source: "runtime.mcp_registry", Provider: "calendar"},
	}}})
	if err == nil || !contains(err.Error(), "unproven individual name") {
		t.Fatalf("Compile error = %v, want unproven-name rejection", err)
	}
}

func TestCompileRejectsConfiguredUnavailableToolsetThatResolverSilentlyOmits(t *testing.T) {
	configured := inventory.ConfiguredToolset{Name: "mcp:calendar", Owner: "harness.mcp", Condition: "calendar configured", Provenance: inventory.ResolverProvenance{Source: "runtime.mcp_registry", Provider: "calendar"}}
	_, err := inventory.Compile(inventory.Input{ConfiguredUnavailableToolsets: []inventory.ConfiguredToolset{configured}})
	if err == nil || !contains(err.Error(), "silently omitted") {
		t.Fatalf("Compile error = %v, want omitted configured provider rejection", err)
	}
	_, err = inventory.Compile(inventory.Input{
		ConfiguredUnavailableToolsets: []inventory.ConfiguredToolset{configured},
		Unavailable:                   []inventory.ResolverObservation{{Kind: inventory.ToolsetKind, Name: configured.Name, Owner: configured.Owner, Condition: configured.Condition, Reason: "mcp_server_not_connected", Provenance: configured.Provenance}},
	})
	if err != nil {
		t.Fatalf("matching resolver observation rejected: %v", err)
	}
}

func TestCompileRejectsConfiguredUnavailableToolsetWithMismatchedResolverProvenance(t *testing.T) {
	configured := inventory.ConfiguredToolset{
		Name: "mcp:calendar", Owner: "harness.mcp", Condition: "calendar configured",
		Provenance: inventory.ResolverProvenance{Source: "runtime.mcp_registry", Provider: "calendar"},
	}
	matching := inventory.ResolverObservation{
		Kind: inventory.ToolsetKind, Name: configured.Name, Owner: configured.Owner,
		Condition: configured.Condition, Reason: "mcp_server_not_connected", Provenance: configured.Provenance,
	}

	tests := map[string]func(*inventory.ResolverObservation){
		"owner":                  func(got *inventory.ResolverObservation) { got.Owner = "harness.other" },
		"condition":              func(got *inventory.ResolverObservation) { got.Condition = "different condition" },
		"resolver source":        func(got *inventory.ResolverObservation) { got.Provenance.Source = "runtime.other" },
		"provider":               func(got *inventory.ResolverObservation) { got.Provenance.Provider = "mail" },
		"individual names known": func(got *inventory.ResolverObservation) { got.Provenance.IndividualNamesKnown = true },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			observation := matching
			mutate(&observation)
			_, err := inventory.Compile(inventory.Input{
				ConfiguredUnavailableToolsets: []inventory.ConfiguredToolset{configured},
				Unavailable:                   []inventory.ResolverObservation{observation},
			})
			if err == nil {
				t.Fatal("Compile accepted a same-name observation with mismatched authoritative provenance")
			}
		})
	}

	_, err := inventory.Compile(inventory.Input{
		ConfiguredUnavailableToolsets: []inventory.ConfiguredToolset{configured, configured},
		Unavailable:                   []inventory.ResolverObservation{matching},
	})
	if err == nil {
		t.Fatal("Compile accepted a duplicate configured unavailable toolset")
	}
}

func TestCompileRetainsResolverProvenanceInItemAndInventoryHash(t *testing.T) {
	compile := func(provider string) inventory.Compiled {
		t.Helper()
		compiled, err := inventory.Compile(inventory.Input{Unavailable: []inventory.ResolverObservation{{
			Kind: inventory.ToolsetKind, Name: "mcp:calendar", Owner: "harness.mcp",
			Condition: "calendar configured", Reason: "server unavailable",
			Provenance: inventory.ResolverProvenance{Source: "runtime.mcp_registry", Provider: provider},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		return compiled
	}
	calendar := compile("calendar")
	mail := compile("mail")
	if calendar.Items[0].Provenance.Source != "runtime.mcp_registry" || calendar.Items[0].Provenance.Provider != "calendar" {
		t.Fatalf("compiled item dropped resolver provenance: %#v", calendar.Items[0])
	}
	if calendar.Hash == mail.Hash {
		t.Fatal("inventory hash did not change when authoritative resolver provider changed")
	}
}

func TestCompileRejectsUnavailableTUICommandObservation(t *testing.T) {
	_, err := inventory.Compile(inventory.Input{Unavailable: []inventory.ResolverObservation{{
		Kind: inventory.TUICommandKind, Name: "help", Owner: "harnesscli.tui.builtin",
		Condition: "built-in", Reason: "invented resolver omission",
		Provenance: inventory.ResolverProvenance{Source: "runtime.test", Provider: "tui", IndividualNamesKnown: true},
	}}})
	if err == nil {
		t.Fatal("Compile accepted a TUI command as a runtime resolver observation")
	}
}

func TestCompileRejectsDuplicateCommandAliasWithoutCreatingAliasItem(t *testing.T) {
	_, err := inventory.Compile(inventory.Input{Commands: []tui.CommandEntry{
		{Name: "resume", Aliases: []string{"continue"}, Owner: "test", Condition: "test"},
		{Name: "other", Aliases: []string{"continue"}, Owner: "test", Condition: "test"},
	}})
	if err == nil || !contains(err.Error(), "alias") {
		t.Fatalf("Compile error = %v, want duplicate alias rejection", err)
	}

	compiled, err := inventory.Compile(inventory.Input{Commands: []tui.CommandEntry{{Name: "resume", Aliases: []string{"continue"}, Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Items) != 1 || compiled.Items[0].ID != "tui_command:resume" {
		t.Fatalf("aliases must be linked to their canonical command, got %#v", compiled.Items)
	}
}

func TestCompilePreservesAuthoritativeTUICommandProvenance(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Commands: []tui.CommandEntry{{
		Name: "custom", Owner: "harnesscli.tui.plugin", Condition: "enabled plugin bundle",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	item := assertItem(t, compiled, "tui_command:custom", inventory.Available, "harnesscli.tui.plugin", "enabled plugin bundle")
	if item.Owner != "harnesscli.tui.plugin" || item.Condition != "enabled plugin bundle" {
		t.Fatalf("compiled command provenance = owner %q condition %q", item.Owner, item.Condition)
	}
}

func TestCompileRejectsTUICommandWithoutAuthoritativeProvenance(t *testing.T) {
	_, err := inventory.Compile(inventory.Input{Commands: []tui.CommandEntry{{Name: "custom"}}})
	if err == nil || !contains(err.Error(), "provenance") {
		t.Fatalf("Compile error = %v, want missing command provenance rejection", err)
	}
}

func TestValidateCasesRejectsInventoryOmission(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{
		{Definition: harness.ToolDefinition{Name: "one"}, Tier: tools.TierCore, Owner: "test", Condition: "test"},
		{Definition: harness.ToolDefinition{Name: "two"}, Tier: tools.TierDeferred, Owner: "test", Condition: "test"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = inventory.ValidateCases(compiled, []inventory.Case{{
		ItemID: "tool:one", EvidenceClass: inventory.EvidenceClassConversation,
		Surfaces:               []inventory.Surface{inventory.SurfaceAPI, inventory.SurfaceTUI},
		OrderedActions:         []inventory.Action{{Kind: "prompt", Value: "use one"}},
		ExpectedPostconditions: testPostconditions("fixture probe", "fixture-observed", "fixture was observed"),
		Cleanup:                "remove fixture",
	}})
	if err == nil || !contains(err.Error(), "tool:two") {
		t.Fatalf("ValidateCases error = %v, want omitted tool identity", err)
	}
}

func TestValidateCasesForSurfaceUsesFullInventoryWithoutRequiringOtherSurfaces(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{
		Tools: []harness.ToolMetadata{
			{Definition: harness.ToolDefinition{Name: "one"}, Tier: tools.TierCore, Owner: "test", Condition: "test"},
			{Definition: harness.ToolDefinition{Name: "two"}, Tier: tools.TierDeferred, Owner: "test", Condition: "test"},
		},
		Commands: []tui.CommandEntry{{Name: "help", Owner: "test", Condition: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	originalHash := compiled.Hash
	apiCase := func(itemID string) inventory.Case {
		return inventory.Case{
			ItemID: itemID, Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation,
			OrderedActions:         []inventory.Action{{Kind: "prompt", Value: "exercise " + itemID}},
			ExpectedPostconditions: testPostconditions("fixture probe", itemID, "fixture was observed"),
			Cleanup:                "remove fixture",
		}
	}
	cases := []inventory.Case{apiCase("tool:one"), apiCase("tool:two")}
	if err := inventory.ValidateCasesForSurface(compiled, cases, inventory.SurfaceAPI); err != nil {
		t.Fatalf("API-only validation rejected complete API cases: %v", err)
	}
	if compiled.Hash != originalHash || len(compiled.Items) != 3 {
		t.Fatalf("surface validation filtered or re-hashed the compiled inventory: %#v", compiled)
	}

	if err := inventory.ValidateCasesForSurface(compiled, cases[:1], inventory.SurfaceAPI); err == nil || !contains(err.Error(), "tool:two@api") {
		t.Fatalf("missing API case error = %v", err)
	}
	extra := apiCase("tool:one")
	extra.Surfaces = []inventory.Surface{inventory.SurfaceTUI}
	if err := inventory.ValidateCasesForSurface(compiled, append(cases, extra), inventory.SurfaceAPI); err == nil || !contains(err.Error(), "selected surface") {
		t.Fatalf("extra non-API mapping error = %v", err)
	}
	if err := inventory.ValidateCasesForSurface(compiled, cases, inventory.Surface("telepathy")); err == nil {
		t.Fatal("unknown selected surface was accepted")
	}
}

func TestValidateEvidenceRejectsCompletedEventWithoutCorrectPostcondition(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "write"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	caseSpec := inventory.Case{
		ItemID: "tool:write", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation,
		OrderedActions:         []inventory.Action{{Kind: "prompt", Value: "write a fixture"}},
		ExpectedPostconditions: testPostconditions("read fixture", "fixture-content", "fixture file contains exact content"), Cleanup: "remove fixture",
	}
	base := inventory.Evidence{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash,
		ItemID: "tool:write", Surface: inventory.SurfaceAPI, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Pass,
		OrderedActions: []inventory.Action{{Kind: "prompt", Value: "write a fixture"}},
		RunID:          "run-1", ConversationID: "conversation-1", EventIDs: []string{"event-tool-completed"},
		ExpectedPostconditions: caseSpec.ExpectedPostconditions,
		Artifacts:              testArtifacts("events.jsonl", "probe.txt"), Cleanup: inventory.CleanupEvidence{Verified: true, Detail: "fixture removed"},
		Timing: inventory.Timing{StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC().Add(time.Second)},
	}
	if err := inventory.ValidateEvidence(compiled, caseSpec, base); err == nil {
		t.Fatal("completed tool event without postcondition was accepted as pass")
	}
	base.ObservedPostconditions = []inventory.ProbeObservation{{Kind: inventory.PostconditionExternalState, Probe: "wrong probe", AssertionID: "wrong-assertion", Value: "wrong content", Verified: true}}
	if err := inventory.ValidateEvidence(compiled, caseSpec, base); err == nil {
		t.Fatal("incorrect postcondition was accepted as pass")
	}
	base.ObservedPostconditions = []inventory.ProbeObservation{{Kind: caseSpec.ExpectedPostconditions[0].Kind, Probe: caseSpec.ExpectedPostconditions[0].Probe, AssertionID: caseSpec.ExpectedPostconditions[0].AssertionID, Value: "sha256:123", Verified: true}}
	if err := inventory.ValidateEvidence(compiled, caseSpec, base); err != nil {
		t.Fatalf("fully evidenced pass rejected: %v", err)
	}
	base.OrderedActions = []inventory.Action{{Kind: "prompt", Value: "a different action"}}
	if err := inventory.ValidateEvidence(compiled, caseSpec, base); err == nil {
		t.Fatal("evidence with different ordered actions was accepted")
	}
}

func TestValidateEvidenceRejectsIdentityOrSurfaceOutsideCompiledInventory(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{
		Tools:    []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "read"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}},
		Commands: []tui.CommandEntry{{Name: "help", Description: "show help", Owner: "test", Condition: "test"}},
		Unavailable: []inventory.ResolverObservation{{
			Kind: inventory.ToolsetKind, Name: "mcp:calendar", Owner: "harness.mcp", Condition: "calendar configured", Reason: "unavailable",
			Provenance: inventory.ResolverProvenance{Source: "runtime.mcp_registry", Provider: "calendar"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	makePair := func(itemID string, surface inventory.Surface) (inventory.Case, inventory.Evidence) {
		postcondition := inventory.Postcondition{Kind: inventory.PostconditionExternalState, Probe: "probe", AssertionID: "assertion", Description: "observed effect"}
		actions := []inventory.Action{{Kind: "prompt", Value: "exercise item"}}
		caseSpec := inventory.Case{ItemID: itemID, Surfaces: []inventory.Surface{surface}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: actions, ExpectedPostconditions: []inventory.Postcondition{postcondition}, Cleanup: "clean"}
		evidence := inventory.Evidence{
			SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash, ItemID: itemID, Surface: surface, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Pass,
			OrderedActions: actions, RunID: "run-1", ConversationID: "conversation-1", EventIDs: []string{"event-1"},
			ExpectedPostconditions: []inventory.Postcondition{postcondition},
			ObservedPostconditions: []inventory.ProbeObservation{{Kind: postcondition.Kind, Probe: postcondition.Probe, AssertionID: postcondition.AssertionID, Value: "verified", Verified: true}},
			Artifacts:              testArtifacts("events.jsonl"), Cleanup: inventory.CleanupEvidence{Verified: true, Detail: "clean"},
			Timing: inventory.Timing{StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC().Add(time.Second)},
		}
		return caseSpec, evidence
	}

	for _, test := range []struct {
		name    string
		itemID  string
		surface inventory.Surface
	}{
		{name: "unknown item", itemID: "tool:ghost", surface: inventory.SurfaceAPI},
		{name: "not-applicable item", itemID: "toolset:mcp:calendar", surface: inventory.SurfaceAPI},
		{name: "item surface mismatch", itemID: "tui_command:help", surface: inventory.SurfaceAPI},
		{name: "unknown surface", itemID: "tool:read", surface: inventory.Surface("telepathy")},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseSpec, evidence := makePair(test.itemID, test.surface)
			if err := inventory.ValidateEvidence(compiled, caseSpec, evidence); err == nil {
				t.Fatal("ValidateEvidence accepted caller-controlled identity/applicability outside the compiled inventory")
			}
		})
	}
}

func TestRenderResultMarkdownRepresentsPassFailAndNotApplicable(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{
		Tools:       []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "read"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}},
		Commands:    []tui.CommandEntry{{Name: "help", Owner: "test", Condition: "test"}},
		Unavailable: []inventory.ResolverObservation{{Kind: inventory.ToolsetKind, Name: "dynamic-provider", Owner: "harness.dynamic", Condition: "enabled", Reason: "disabled", Provenance: inventory.ResolverProvenance{Source: "runtime.dynamic", Provider: "dynamic-provider"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	caseSpec := inventory.Case{
		ItemID: "tool:read", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation,
		OrderedActions:         []inventory.Action{{Kind: "prompt", Value: "read"}},
		ExpectedPostconditions: testPostconditions("probe", "read", "read observed"), Cleanup: "clean",
	}
	report, err := inventory.RenderResultMarkdown(compiled, []inventory.Case{caseSpec}, []inventory.Evidence{{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash, ItemID: "tool:read", Surface: inventory.SurfaceAPI, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Fail,
		OrderedActions: caseSpec.OrderedActions, Artifacts: testArtifacts("events.jsonl"),
		Timing: inventory.Timing{StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC().Add(time.Second)}, FailureClass: "product",
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fail", "not-applicable", "pending"} {
		if !contains(report, want) {
			t.Fatalf("report missing %q: %s", want, report)
		}
	}
}

func TestRenderResultMarkdownDoesNotTrustStructurallyInvalidPassEvidence(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{
		Definition: harness.ToolDefinition{Name: "read"}, Tier: tools.TierCore, Owner: "test", Condition: "test",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	caseSpec := inventory.Case{
		ItemID: "tool:read", Surfaces: []inventory.Surface{inventory.SurfaceAPI}, EvidenceClass: inventory.EvidenceClassConversation,
		OrderedActions:         []inventory.Action{{Kind: "prompt", Value: "read"}},
		ExpectedPostconditions: testPostconditions("probe", "read", "read observed"), Cleanup: "clean",
	}
	if _, err := inventory.RenderResultMarkdown(compiled, []inventory.Case{caseSpec}, []inventory.Evidence{{ItemID: "tool:read", Surface: inventory.SurfaceAPI, Outcome: inventory.Pass}}); err == nil {
		t.Fatal("renderer accepted an unvalidated structurally empty pass")
	}
}

func TestRenderResultMarkdownEmitsOneWellFormedRowPerItemSurface(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{
		Definition: harness.ToolDefinition{Name: "read", Description: "read a fixture"},
		Tier:       tools.TierCore,
		Owner:      "test",
		Condition:  "test",
	}}})
	if err != nil {
		t.Fatal(err)
	}

	var rows []string
	for _, line := range strings.Split(inventory.RenderMarkdown(compiled), "\n") {
		if strings.Contains(line, "tool:read") {
			rows = append(rows, line)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("tool:read row count = %d, want registry-derived API and TUI rows; native requires a suite overlay:\n%s", len(rows), strings.Join(rows, "\n"))
	}
	for _, row := range rows {
		if columns := strings.Count(row, "|") - 1; columns != 8 {
			t.Errorf("tool:read report columns = %d, want 8: %s", columns, row)
		}
	}
}

func TestTUICommandAliasesRequireDistinctInventoryDerivedInvocations(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Commands: []tui.CommandEntry{{
		Name: "resume", Aliases: []string{"continue"}, Owner: "test", Condition: "test",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	item := compiled.Items[0]
	if len(item.Invocations) != 2 || item.Invocations[0].ID != "tui_command:resume/canonical" || item.Invocations[1].ID != "tui_command:resume/alias:continue" {
		t.Fatalf("command invocations = %#v", item.Invocations)
	}
	makeCase := func(invocationID string) inventory.Case {
		return inventory.Case{
			ItemID: "tui_command:resume", InvocationID: invocationID,
			Surfaces: []inventory.Surface{inventory.SurfaceTUI}, EvidenceClass: inventory.EvidenceClassConversation,
			OrderedActions:         []inventory.Action{{Kind: "pty_input", Value: invocationID}},
			ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionConversationState, Probe: "conversation", AssertionID: invocationID, Description: "continuation starts exactly once"}},
			Cleanup:                "finish continuation",
		}
	}
	canonical := makeCase("tui_command:resume/canonical")
	alias := makeCase("tui_command:resume/alias:continue")
	if err := inventory.ValidateCasesForSurface(compiled, []inventory.Case{canonical}, inventory.SurfaceTUI); err == nil || !contains(err.Error(), alias.InvocationID) {
		t.Fatalf("missing alias invocation error = %v", err)
	}
	if err := inventory.ValidateCasesForSurface(compiled, []inventory.Case{canonical, alias}, inventory.SurfaceTUI); err != nil {
		t.Fatalf("complete canonical+alias cases rejected: %v", err)
	}
	evidence := inventory.Evidence{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash,
		ItemID: canonical.ItemID, InvocationID: alias.InvocationID, Surface: inventory.SurfaceTUI, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Pass,
		OrderedActions: canonical.OrderedActions, RunID: "run", ConversationID: "conversation", EventIDs: []string{"event"},
		ExpectedPostconditions: canonical.ExpectedPostconditions,
		ObservedPostconditions: []inventory.ProbeObservation{{Kind: inventory.PostconditionConversationState, Probe: "conversation", AssertionID: canonical.InvocationID, Value: "one continuation", Verified: true}},
		Artifacts:              testArtifacts("resume.ansi"), Cleanup: inventory.CleanupEvidence{Verified: true, Detail: "finished"},
		Timing: inventory.Timing{StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC().Add(time.Second)},
	}
	if err := inventory.ValidateEvidence(compiled, canonical, evidence); err == nil || !contains(err.Error(), "identity") {
		t.Fatalf("alias evidence satisfied canonical invocation: %v", err)
	}
	report := inventory.RenderMarkdown(compiled)
	if strings.Count(report, "tui_command:resume/canonical") != 1 || strings.Count(report, "tui_command:resume/alias:continue") != 1 {
		t.Fatalf("report does not expose distinct command invocations:\n%s", report)
	}
}

func TestEvidenceClassesMultiProbeAndTypedArtifactsFailClosed(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Commands: []tui.CommandEntry{{Name: "help", Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	postconditions := []inventory.Postcondition{
		{Kind: inventory.PostconditionRenderedScreen, Probe: "pty_frame", AssertionID: "help-visible", Description: "help is rendered"},
		{Kind: inventory.PostconditionDurableState, Probe: "conversation_store", AssertionID: "no-message-write", Description: "local help does not mutate conversation"},
	}
	caseSpec := inventory.Case{
		ItemID: "tui_command:help", InvocationID: "tui_command:help/canonical",
		Surfaces: []inventory.Surface{inventory.SurfaceTUI}, EvidenceClass: inventory.EvidenceClassLocal,
		OrderedActions: []inventory.Action{{Kind: "pty_input", Value: "/help"}}, ExpectedPostconditions: postconditions, Cleanup: "close help",
	}
	evidence := inventory.Evidence{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash,
		ItemID: caseSpec.ItemID, InvocationID: caseSpec.InvocationID, Surface: inventory.SurfaceTUI, EvidenceClass: inventory.EvidenceClassLocal, Outcome: inventory.Pass,
		OrderedActions: caseSpec.OrderedActions, ExpectedPostconditions: postconditions,
		ObservedPostconditions: []inventory.ProbeObservation{{Kind: inventory.PostconditionRenderedScreen, Probe: "pty_frame", AssertionID: "help-visible", Value: "visible", Verified: true}},
		Artifacts:              []inventory.ArtifactRef{{Kind: inventory.ArtifactTerminalCapture, Path: "help.ansi", Digest: "sha256:" + strings.Repeat("0", 64), Redacted: boolPointer(true)}},
		Cleanup:                inventory.CleanupEvidence{Verified: true, Detail: "closed"}, Timing: inventory.Timing{StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC().Add(time.Second)},
	}
	if err := inventory.ValidateEvidence(compiled, caseSpec, evidence); err == nil || !contains(err.Error(), "postcondition") {
		t.Fatalf("missing independent probe error = %v", err)
	}
	evidence.ObservedPostconditions = append(evidence.ObservedPostconditions, inventory.ProbeObservation{Kind: inventory.PostconditionDurableState, Probe: "conversation_store", AssertionID: "no-message-write", Value: "unchanged", Verified: true})
	if err := inventory.ValidateEvidence(compiled, caseSpec, evidence); err != nil {
		t.Fatalf("honest local evidence without runtime IDs rejected: %v", err)
	}
	evidence.RunID = "fabricated"
	if err := inventory.ValidateEvidence(compiled, caseSpec, evidence); err == nil || !contains(err.Error(), "local") {
		t.Fatalf("local evidence with fabricated runtime ID error = %v", err)
	}
	evidence.RunID = ""
	evidence.Artifacts[0].Digest = "not-a-digest"
	if err := inventory.ValidateEvidence(compiled, caseSpec, evidence); err == nil || !contains(err.Error(), "digest") {
		t.Fatalf("unverifiable artifact error = %v", err)
	}
	evidence.Artifacts[0].Digest = "sha256:" + strings.Repeat("0", 64)
	evidence.Artifacts[0].Redacted = nil
	if err := inventory.ValidateEvidence(compiled, caseSpec, evidence); err == nil || !contains(err.Error(), "redaction") {
		t.Fatalf("artifact without explicit redaction declaration error = %v", err)
	}
	evidence.Artifacts[0].Redacted = boolPointer(true)

	conversation := caseSpec
	conversation.EvidenceClass = inventory.EvidenceClassConversation
	evidence.Artifacts[0].Digest = "sha256:" + strings.Repeat("0", 64)
	evidence.EvidenceClass = inventory.EvidenceClassConversation
	if err := inventory.ValidateEvidence(compiled, conversation, evidence); err == nil || !contains(err.Error(), "run, conversation, and event") {
		t.Fatalf("conversation evidence without runtime identities error = %v", err)
	}

	toolCompiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{Definition: harness.ToolDefinition{Name: "read"}, Tier: tools.TierCore, Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	toolCase := caseSpec
	toolCase.ItemID, toolCase.InvocationID = "tool:read", ""
	toolEvidence := evidence
	toolEvidence.InventoryHash, toolEvidence.ItemID, toolEvidence.InvocationID = toolCompiled.Hash, "tool:read", ""
	toolEvidence.EvidenceClass, toolEvidence.RunID = inventory.EvidenceClassLocal, ""
	if err := inventory.ValidateEvidence(toolCompiled, toolCase, toolEvidence); err == nil || !contains(err.Error(), "non-command") {
		t.Fatalf("local evidence accepted for non-command inventory: %v", err)
	}
}

func TestSuiteContractRequiresDeclaredSyntheticScenarios(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Commands: []tui.CommandEntry{{Name: "help", Owner: "test", Condition: "test"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inventory.CompileSuiteContract(compiled, []inventory.ScenarioContract{{
		ID: "unstable id", Kind: inventory.ScenarioUnknownCommand, Surface: inventory.SurfaceTUI, EvidenceClass: inventory.EvidenceClassLocal, Description: "invalid ID",
	}}, nil); err == nil || !contains(err.Error(), "stable ID") {
		t.Fatalf("suite accepted unstable scenario ID: %v", err)
	}
	contract, err := inventory.CompileSuiteContract(compiled, []inventory.ScenarioContract{
		{ID: "tui.unknown-command", Kind: inventory.ScenarioUnknownCommand, Surface: inventory.SurfaceTUI, EvidenceClass: inventory.EvidenceClassLocal, Description: "unknown command is rejected visibly"},
		{ID: "tui.help.invalid-form", Kind: inventory.ScenarioInvalidForm, Surface: inventory.SurfaceTUI, EvidenceClass: inventory.EvidenceClassLocal, Description: "invalid help form is rejected visibly"},
	}, []inventory.SurfaceApplicability{{ItemID: "tui_command:help", Surface: inventory.SurfaceNativeGUI, Availability: inventory.NotApplicable, SourceRefs: []string{"macapp/GoCodeApp/Views/ContentView.swift"}, UXRationale: "slash commands are terminal-only"}})
	if err != nil {
		t.Fatal(err)
	}
	if contract.InventoryHash != compiled.Hash || contract.Hash == "" {
		t.Fatalf("suite contract is not bound to full inventory: %#v", contract)
	}
	makeCase := func(itemID, scenarioID, invocationID string) inventory.Case {
		return inventory.Case{
			ItemID: itemID, ScenarioID: scenarioID, InvocationID: invocationID,
			Surfaces: []inventory.Surface{inventory.SurfaceTUI}, EvidenceClass: inventory.EvidenceClassLocal,
			OrderedActions:         []inventory.Action{{Kind: "pty_input", Value: itemID + scenarioID}},
			ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionRenderedScreen, Probe: "pty_frame", AssertionID: itemID + scenarioID, Description: "visible result"}},
			Cleanup:                "dismiss output",
		}
	}
	commandCase := makeCase("tui_command:help", "", "tui_command:help/canonical")
	unknownCase := makeCase("", "tui.unknown-command", "")
	invalidCase := makeCase("", "tui.help.invalid-form", "")
	if err := inventory.ValidateSuiteCasesForSurface(compiled, contract, []inventory.Case{commandCase, unknownCase}, inventory.SurfaceTUI); err == nil || !contains(err.Error(), "tui.help.invalid-form") {
		t.Fatalf("missing required negative scenario error = %v", err)
	}
	undeclared := makeCase("", "tui.invented", "")
	if err := inventory.ValidateSuiteCasesForSurface(compiled, contract, []inventory.Case{commandCase, unknownCase, invalidCase, undeclared}, inventory.SurfaceTUI); err == nil || !contains(err.Error(), "undeclared") {
		t.Fatalf("undeclared synthetic scenario error = %v", err)
	}
	if err := inventory.ValidateSuiteCasesForSurface(compiled, contract, []inventory.Case{commandCase, unknownCase, invalidCase}, inventory.SurfaceTUI); err != nil {
		t.Fatalf("complete suite contract rejected: %v", err)
	}
	evidence := inventory.Evidence{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash, SuiteHash: contract.Hash,
		ScenarioID: unknownCase.ScenarioID, Surface: inventory.SurfaceTUI, EvidenceClass: inventory.EvidenceClassLocal, Outcome: inventory.Pass,
		OrderedActions: unknownCase.OrderedActions, ExpectedPostconditions: unknownCase.ExpectedPostconditions,
		ObservedPostconditions: []inventory.ProbeObservation{{Kind: inventory.PostconditionRenderedScreen, Probe: "pty_frame", AssertionID: unknownCase.ScenarioID, Value: "unknown command shown", Verified: true}},
		Artifacts:              testArtifacts("unknown-command.ansi"), Cleanup: inventory.CleanupEvidence{Verified: true, Detail: "dismissed"},
		Timing: inventory.Timing{StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC().Add(time.Second)},
	}
	if err := inventory.ValidateSuiteEvidence(compiled, contract, unknownCase, evidence); err != nil {
		t.Fatalf("valid synthetic scenario evidence rejected: %v", err)
	}
	report, err := inventory.RenderSuiteResultMarkdown(compiled, contract, []inventory.Case{commandCase, unknownCase, invalidCase}, []inventory.Evidence{evidence})
	if err != nil || !contains(report, "tui.unknown-command") || !contains(report, "unknown_command") || !contains(report, "pass") {
		t.Fatalf("suite report omitted validated synthetic evidence: err=%v report=%s", err, report)
	}
	evidence.SuiteHash = "wrong"
	if err := inventory.ValidateSuiteEvidence(compiled, contract, unknownCase, evidence); err == nil || !contains(err.Error(), "suite hash") {
		t.Fatalf("synthetic evidence escaped suite hash provenance: %v", err)
	}
}

func TestNativeApplicabilityAndPassProofFailClosed(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Tools: []harness.ToolMetadata{{
		Definition: harness.ToolDefinition{Name: "read", Description: "read a file"}, Tier: tools.TierCore, Owner: "test", Condition: "test",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inventory.CompileSuiteContract(compiled, nil, nil); err == nil || !contains(err.Error(), "missing native applicability") {
		t.Fatalf("missing native applicability mapping error = %v", err)
	}
	if _, err := inventory.CompileSuiteContract(compiled, nil, []inventory.SurfaceApplicability{{
		ItemID: "tool:invented", Surface: inventory.SurfaceNativeGUI, Availability: inventory.Available,
		SourceRefs: []string{"macapp/GoCodeApp/Views/ContentView.swift"}, UXRationale: "visible control",
	}}); err == nil || !contains(err.Error(), "unknown inventory item") {
		t.Fatalf("unknown native applicability mapping error = %v", err)
	}
	notApplicable, err := inventory.CompileSuiteContract(compiled, nil, []inventory.SurfaceApplicability{{
		ItemID: "tool:read", Surface: inventory.SurfaceNativeGUI, Availability: inventory.NotApplicable,
		SourceRefs: []string{"macapp/GoCodeApp/Views/ContentView.swift"}, UXRationale: "no native GUI analogue",
	}})
	if err != nil {
		t.Fatal(err)
	}
	nativeCase := inventory.Case{
		ItemID: "tool:read", Surfaces: []inventory.Surface{inventory.SurfaceNativeGUI}, EvidenceClass: inventory.EvidenceClassConversation,
		OrderedActions:         []inventory.Action{{Kind: "native_click", Value: "read"}},
		ExpectedPostconditions: []inventory.Postcondition{{Kind: inventory.PostconditionRenderedScreen, Probe: "native_window", AssertionID: "read-visible", Description: "result is visible"}},
		Cleanup:                "close conversation",
	}
	if err := inventory.ValidateSuiteCasesForSurface(compiled, notApplicable, []inventory.Case{nativeCase}, inventory.SurfaceNativeGUI); err == nil || !contains(err.Error(), "not-applicable") {
		t.Fatalf("native case accepted for explicit N/A mapping: %v", err)
	}
	applicable, err := inventory.CompileSuiteContract(compiled, nil, []inventory.SurfaceApplicability{{
		ItemID: "tool:read", Surface: inventory.SurfaceNativeGUI, Availability: inventory.Available,
		SourceRefs: []string{"macapp/GoCodeApp/Views/ToolPanel.swift"}, UXRationale: "native tool panel exposes read",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := inventory.ValidateSuiteCasesForSurface(compiled, applicable, nil, inventory.SurfaceNativeGUI); err == nil || !contains(err.Error(), "missing acceptance case") {
		t.Fatalf("native-applicable item did not require a case: %v", err)
	}
	if err := inventory.ValidateSuiteCasesForSurface(compiled, applicable, []inventory.Case{nativeCase}, inventory.SurfaceNativeGUI); err != nil {
		t.Fatalf("complete native mapping/case rejected: %v", err)
	}
	now := time.Now().UTC()
	evidence := inventory.Evidence{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash, SuiteHash: applicable.Hash,
		ItemID: nativeCase.ItemID, Surface: inventory.SurfaceNativeGUI, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Pass,
		OrderedActions: nativeCase.OrderedActions, RunID: "run", ConversationID: "conversation", EventIDs: []string{"event"},
		ExpectedPostconditions: nativeCase.ExpectedPostconditions,
		ObservedPostconditions: []inventory.ProbeObservation{{Kind: inventory.PostconditionRenderedScreen, Probe: "native_window", AssertionID: "read-visible", Value: "visible", Verified: true}},
		Artifacts:              testArtifacts("native-probe.json"),
		Cleanup:                inventory.CleanupEvidence{Verified: true, Detail: "closed"}, Timing: inventory.Timing{StartedAt: now, FinishedAt: now.Add(time.Second)},
	}
	if err := inventory.ValidateSuiteEvidence(compiled, applicable, nativeCase, evidence); err == nil || !contains(err.Error(), "native pass") {
		t.Fatalf("native pass without proof bundle/environment error = %v", err)
	}
	evidence.Artifacts = []inventory.ArtifactRef{
		{Kind: inventory.ArtifactScreenshot, Path: "window.png", Digest: "sha256:" + strings.Repeat("1", 64), Redacted: boolPointer(true)},
		{Kind: inventory.ArtifactAXSnapshot, Path: "window.ax.json", Digest: "sha256:" + strings.Repeat("2", 64), Redacted: boolPointer(true)},
		{Kind: inventory.ArtifactRawSSEEvent, Path: "events.ndjson", Digest: "sha256:" + strings.Repeat("3", 64), Redacted: boolPointer(true)},
		{Kind: inventory.ArtifactAPIStoreProbe, Path: "state.json", Digest: "sha256:" + strings.Repeat("4", 64), Redacted: boolPointer(true)},
	}
	evidence.Environment = inventory.EnvironmentMetadata{
		BuildSHA: "0123456789abcdef0123456789abcdef01234567", BundlePath: "/Applications/GoCode.app",
		DaemonPID: 1234, DaemonPort: 18080, WorkspacePath: "/private/tmp/gocode-native-1086", WorkspaceIsolated: true,
	}
	if err := inventory.ValidateSuiteEvidence(compiled, applicable, nativeCase, evidence); err != nil {
		t.Fatalf("complete native proof rejected: %v", err)
	}
	evidence.Environment.WorkspaceIsolated = false
	if err := inventory.ValidateSuiteEvidence(compiled, applicable, nativeCase, evidence); err == nil || !contains(err.Error(), "workspace isolation") {
		t.Fatalf("shared-workspace native pass accepted: %v", err)
	}
}

func assertItem(t *testing.T, compiled inventory.Compiled, id string, availability inventory.Availability, owner, condition string) inventory.Item {
	t.Helper()
	for _, item := range compiled.Items {
		if item.ID == id {
			if item.Availability != availability || item.Owner != owner || item.Condition != condition {
				t.Fatalf("%s = %#v", id, item)
			}
			return item
		}
	}
	t.Fatalf("inventory item %q not found", id)
	return inventory.Item{}
}

func contains(s, want string) bool { return len(want) == 0 || (len(s) >= len(want) && find(s, want)) }

func testPostconditions(probe, assertionID, description string) []inventory.Postcondition {
	return []inventory.Postcondition{{
		Kind: inventory.PostconditionExternalState, Probe: probe, AssertionID: assertionID, Description: description,
	}}
}

func testArtifacts(paths ...string) []inventory.ArtifactRef {
	artifacts := make([]inventory.ArtifactRef, 0, len(paths))
	for _, path := range paths {
		artifacts = append(artifacts, inventory.ArtifactRef{
			Kind: inventory.ArtifactProbe, Path: path, Digest: "sha256:" + strings.Repeat("0", 64), Redacted: boolPointer(true),
		})
	}
	return artifacts
}

func boolPointer(value bool) *bool { return &value }

func find(s, want string) bool {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
