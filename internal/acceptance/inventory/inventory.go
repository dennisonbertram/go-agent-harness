// Package inventory compiles the condition-resolved production catalogs used by
// the acceptance matrix. It intentionally owns no tool or command list: names,
// aliases, descriptions, tiers, and tags are copied from the registries passed
// to Compile.
package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/internal/harness"
	htools "go-agent-harness/internal/harness/tools"
)

// SchemaVersion is incremented only for incompatible evidence-format changes.
const SchemaVersion = "acceptance-evidence/v2"

type Kind string

const (
	ToolKind       Kind = "tool"
	TUICommandKind Kind = "tui_command"
	// ToolsetKind represents a configured dynamic provider whose individual
	// names are unknowable because resolution did not reach that provider.
	ToolsetKind Kind = "toolset"
)

type Availability string

const (
	Available     Availability = "available"
	NotApplicable Availability = "not-applicable"
)

type Surface string

const (
	SurfaceAPI       Surface = "api"
	SurfaceTUI       Surface = "tui"
	SurfaceNativeGUI Surface = "native_gui"
)

type Outcome string

const (
	Pass Outcome = "pass"
	Fail Outcome = "fail"
)

type InvocationVariant string

const (
	InvocationCanonical InvocationVariant = "canonical"
	InvocationAlias     InvocationVariant = "alias"
)

type Invocation struct {
	ID      string            `json:"id"`
	Input   string            `json:"input"`
	Variant InvocationVariant `json:"variant"`
}

// Input takes snapshots from the actual, already condition-resolved registries.
// Unavailable holds capability observations from the same runtime resolution
// pass. A capability absent from both is not assumed to exist or to be skipped.
type Input struct {
	Tools                         []harness.ToolMetadata
	Commands                      []tui.CommandEntry
	Unavailable                   []ResolverObservation
	ConfiguredUnavailableToolsets []ConfiguredToolset
}

// HTTPTool is the stable, non-secret subset returned by harnessd's `/v1/tools`
// boundary. It lets an operator compile the report from the running daemon
// rather than reconstructing a registry in a test process.
type HTTPTool struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tier        string   `json:"tier"`
	Tags        []string `json:"tags,omitempty"`
	Owner       string   `json:"owner"`
	Condition   string   `json:"condition"`
}

// InputFromHTTPBoundary preserves both the resolved present catalog and the
// paired configured/observed unavailable resolver records returned by the
// running daemon.
func InputFromHTTPBoundary(tools []HTTPTool, commands []tui.CommandEntry, unavailable []ResolverObservation, configured []ConfiguredToolset) Input {
	metas := make([]harness.ToolMetadata, 0, len(tools))
	for _, tool := range tools {
		metas = append(metas, harness.ToolMetadata{
			Definition: harness.ToolDefinition{Name: tool.Name, Description: tool.Description},
			Tier:       htools.ToolTier(tool.Tier),
			Tags:       append([]string(nil), tool.Tags...),
			Owner:      tool.Owner,
			Condition:  tool.Condition,
		})
	}
	return Input{
		Tools:                         metas,
		Commands:                      commands,
		Unavailable:                   append([]ResolverObservation(nil), unavailable...),
		ConfiguredUnavailableToolsets: append([]ConfiguredToolset(nil), configured...),
	}
}

// ResolverProvenance makes an unavailable observation traceable to the actual
// runtime condition/config resolver rather than a hand-maintained matrix list.
type ResolverProvenance struct {
	Source               string `json:"source"`
	Provider             string `json:"provider"`
	IndividualNamesKnown bool   `json:"individual_names_known"`
}

// ResolverObservation records a condition-backed stable not-applicable result
// from a runtime resolver. Use ToolsetKind when a configured provider did not
// disclose individual names; ToolKind is valid only when the resolver observed
// the particular name.
type ResolverObservation struct {
	Kind       Kind               `json:"kind"`
	Name       string             `json:"name"`
	Owner      string             `json:"owner"`
	Condition  string             `json:"condition"`
	Reason     string             `json:"reason"`
	Provenance ResolverProvenance `json:"provenance"`
}

// ConfiguredToolset is supplied by the same runtime condition resolver when a
// configured dynamic provider is unavailable. Compile requires its matching
// observation, making a resolver omission fail deterministically.
type ConfiguredToolset struct {
	Name       string             `json:"name"`
	Owner      string             `json:"owner"`
	Condition  string             `json:"condition"`
	Provenance ResolverProvenance `json:"provenance"`
}

type Item struct {
	ID           string              `json:"id"`
	Kind         Kind                `json:"kind"`
	Name         string              `json:"name"`
	Aliases      []string            `json:"aliases,omitempty"`
	Invocations  []Invocation        `json:"invocations,omitempty"`
	Owner        string              `json:"owner"`
	Condition    string              `json:"condition"`
	Availability Availability        `json:"availability"`
	Reason       string              `json:"reason,omitempty"`
	Tier         string              `json:"tier,omitempty"`
	Tags         []string            `json:"tags,omitempty"`
	Description  string              `json:"description,omitempty"`
	Surfaces     []Surface           `json:"surfaces"`
	Provenance   *ResolverProvenance `json:"provenance,omitempty"`
}

type Compiled struct {
	SchemaVersion string `json:"schema_version"`
	Hash          string `json:"hash"`
	Items         []Item `json:"items"`
}

func (i Item) canonicalID() string { return string(i.Kind) + ":" + i.Name }

// Compile derives every available item from the supplied registry snapshots.
// It rejects malformed not-applicable records and duplicate identities so
// condition resolution cannot be silently replaced with an omission.
func Compile(input Input) (Compiled, error) {
	items := make([]Item, 0, len(input.Tools)+len(input.Commands)+len(input.Unavailable))
	seen := make(map[string]struct{})
	unavailableToolsets := make(map[string]ResolverObservation)
	commandNames := make(map[string]string, len(input.Commands))
	add := func(item Item) error {
		item.Name = strings.TrimSpace(item.Name)
		item.Owner = strings.TrimSpace(item.Owner)
		item.Condition = strings.TrimSpace(item.Condition)
		item.Reason = strings.TrimSpace(item.Reason)
		item.ID = item.canonicalID()
		if item.Kind != ToolKind && item.Kind != TUICommandKind && item.Kind != ToolsetKind {
			return fmt.Errorf("unsupported inventory kind %q", item.Kind)
		}
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Owner) == "" || strings.TrimSpace(item.Condition) == "" {
			return fmt.Errorf("inventory item %q must include name, owner, and condition", item.ID)
		}
		if item.Availability != Available && item.Availability != NotApplicable {
			return fmt.Errorf("inventory item %q has invalid availability %q", item.ID, item.Availability)
		}
		if item.Availability == NotApplicable && strings.TrimSpace(item.Reason) == "" {
			return fmt.Errorf("not-applicable inventory item %q requires a stable reason", item.ID)
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("duplicate inventory item %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		item.Aliases = sortedCopy(item.Aliases)
		item.Tags = sortedCopy(item.Tags)
		item.Surfaces = sortedSurfaces(item.Surfaces)
		items = append(items, item)
		return nil
	}
	for _, tool := range input.Tools {
		owner := strings.TrimSpace(tool.Owner)
		condition := strings.TrimSpace(tool.Condition)
		if owner == "" || condition == "" || owner == "harness.registry" {
			return Compiled{}, fmt.Errorf("resolved runtime tool %q is missing authoritative owner or condition provenance", tool.Definition.Name)
		}
		if err := add(Item{
			Kind: ToolKind, Name: tool.Definition.Name, Owner: owner,
			Condition: condition, Availability: Available, Tier: string(tool.Tier),
			Tags: tool.Tags, Description: tool.Definition.Description,
			Surfaces: []Surface{SurfaceAPI, SurfaceTUI},
		}); err != nil {
			return Compiled{}, err
		}
	}
	for _, command := range input.Commands {
		owner := strings.TrimSpace(command.Owner)
		condition := strings.TrimSpace(command.Condition)
		if owner == "" || condition == "" {
			return Compiled{}, fmt.Errorf("TUI command %q is missing authoritative owner or condition provenance", command.Name)
		}
		if owner, exists := commandNames[command.Name]; exists {
			return Compiled{}, fmt.Errorf("command name %q is already owned by %q", command.Name, owner)
		}
		commandNames[command.Name] = command.Name
		for _, alias := range command.Aliases {
			if owner, exists := commandNames[alias]; exists {
				return Compiled{}, fmt.Errorf("command alias %q is already owned by %q", alias, owner)
			}
			commandNames[alias] = command.Name
		}
		aliases := sortedCopy(command.Aliases)
		invocations := []Invocation{{
			ID: "tui_command:" + command.Name + "/canonical", Input: "/" + command.Name, Variant: InvocationCanonical,
		}}
		for _, alias := range aliases {
			invocations = append(invocations, Invocation{
				ID: "tui_command:" + command.Name + "/alias:" + alias, Input: "/" + alias, Variant: InvocationAlias,
			})
		}
		if err := add(Item{
			Kind: TUICommandKind, Name: command.Name, Aliases: aliases, Invocations: invocations,
			Owner: owner, Condition: condition, Availability: Available,
			Description: command.Description, Surfaces: []Surface{SurfaceTUI},
		}); err != nil {
			return Compiled{}, err
		}
	}
	for _, unavailable := range input.Unavailable {
		if unavailable.Kind != ToolKind && unavailable.Kind != ToolsetKind {
			return Compiled{}, fmt.Errorf("unavailable observation %q must describe a tool or toolset", unavailable.Name)
		}
		unavailable.Name = strings.TrimSpace(unavailable.Name)
		unavailable.Owner = strings.TrimSpace(unavailable.Owner)
		unavailable.Condition = strings.TrimSpace(unavailable.Condition)
		unavailable.Reason = strings.TrimSpace(unavailable.Reason)
		unavailable.Provenance = normalizeResolverProvenance(unavailable.Provenance)
		if unavailable.Kind == ToolKind && !unavailable.Provenance.IndividualNamesKnown {
			return Compiled{}, fmt.Errorf("unavailable tool %q has an unproven individual name; emit a toolset observation instead", unavailable.Name)
		}
		if unavailable.Kind == ToolsetKind && unavailable.Provenance.IndividualNamesKnown {
			return Compiled{}, fmt.Errorf("toolset observation %q must enumerate its known individual names", unavailable.Name)
		}
		if unavailable.Provenance.Source == "" || unavailable.Provenance.Provider == "" {
			return Compiled{}, fmt.Errorf("unavailable observation %q requires resolver provenance", unavailable.Name)
		}
		provenance := unavailable.Provenance
		if err := add(Item{
			Kind: unavailable.Kind, Name: unavailable.Name, Owner: unavailable.Owner,
			Condition: unavailable.Condition, Availability: NotApplicable, Reason: unavailable.Reason,
			Surfaces: []Surface{SurfaceAPI, SurfaceTUI, SurfaceNativeGUI}, Provenance: &provenance,
		}); err != nil {
			return Compiled{}, err
		}
		if unavailable.Kind == ToolsetKind {
			unavailableToolsets[string(ToolsetKind)+":"+unavailable.Name] = unavailable
		}
	}
	configuredSeen := make(map[string]struct{}, len(input.ConfiguredUnavailableToolsets))
	for _, configured := range input.ConfiguredUnavailableToolsets {
		configured.Name = strings.TrimSpace(configured.Name)
		configured.Owner = strings.TrimSpace(configured.Owner)
		configured.Condition = strings.TrimSpace(configured.Condition)
		configured.Provenance = normalizeResolverProvenance(configured.Provenance)
		if configured.Name == "" || configured.Owner == "" || configured.Condition == "" || configured.Provenance.Source == "" || configured.Provenance.Provider == "" {
			return Compiled{}, fmt.Errorf("configured unavailable toolset requires name, owner, condition, and resolver provenance")
		}
		if configured.Provenance.IndividualNamesKnown {
			return Compiled{}, fmt.Errorf("configured unavailable toolset %q cannot claim known individual names", configured.Name)
		}
		id := string(ToolsetKind) + ":" + configured.Name
		if _, duplicate := configuredSeen[id]; duplicate {
			return Compiled{}, fmt.Errorf("duplicate configured unavailable toolset %q", configured.Name)
		}
		configuredSeen[id] = struct{}{}
		observed, found := unavailableToolsets[id]
		if !found {
			return Compiled{}, fmt.Errorf("configured unavailable toolset %q was silently omitted by resolver", configured.Name)
		}
		if observed.Owner != configured.Owner || observed.Condition != configured.Condition || observed.Provenance != configured.Provenance {
			return Compiled{}, fmt.Errorf("configured unavailable toolset %q does not match resolver provenance", configured.Name)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	canonical, err := json.Marshal(struct {
		SchemaVersion string `json:"schema_version"`
		Items         []Item `json:"items"`
	}{SchemaVersion: SchemaVersion, Items: items})
	if err != nil {
		return Compiled{}, fmt.Errorf("marshal inventory: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return Compiled{SchemaVersion: SchemaVersion, Hash: hex.EncodeToString(sum[:]), Items: items}, nil
}

func normalizeResolverProvenance(value ResolverProvenance) ResolverProvenance {
	value.Source = strings.TrimSpace(value.Source)
	value.Provider = strings.TrimSpace(value.Provider)
	return value
}

func sortedSurfaces(values []Surface) []Surface {
	if len(values) == 0 {
		return nil
	}
	copy := append([]Surface(nil), values...)
	sort.Slice(copy, func(i, j int) bool { return copy[i] < copy[j] })
	return copy
}

func sortedCopy(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	copy := append([]string(nil), values...)
	sort.Strings(copy)
	return copy
}

// Action is an observable input performed by a later surface runner. The
// schema records it here without attempting to prescribe runner mechanics.
type Action struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type EvidenceClass string

const (
	EvidenceClassConversation EvidenceClass = "conversation"
	EvidenceClassLocal        EvidenceClass = "local"
)

// Case contains the manual, intent-specific portion of the matrix. It never
// names an item that is not present in Compiled, preventing a second catalog.
type Case struct {
	ItemID                 string          `json:"item_id,omitempty"`
	ScenarioID             string          `json:"scenario_id,omitempty"`
	InvocationID           string          `json:"invocation_id,omitempty"`
	Surfaces               []Surface       `json:"surfaces"`
	EvidenceClass          EvidenceClass   `json:"evidence_class"`
	OrderedActions         []Action        `json:"ordered_actions"`
	ExpectedPostconditions []Postcondition `json:"expected_postconditions"`
	Cleanup                string          `json:"cleanup"`
}

type PostconditionKind string

const (
	PostconditionRenderedScreen    PostconditionKind = "rendered_screen"
	PostconditionDurableState      PostconditionKind = "durable_state"
	PostconditionConversationState PostconditionKind = "conversation_state"
	PostconditionExternalState     PostconditionKind = "external_state"
)

// Postcondition identifies a probe and machine-checkable assertion contract;
// the human description is explanatory and cannot itself constitute proof.
type Postcondition struct {
	Kind        PostconditionKind `json:"kind"`
	Probe       string            `json:"probe"`
	AssertionID string            `json:"assertion_id"`
	Description string            `json:"description"`
}

type ScenarioKind string

const (
	ScenarioUnknownCommand ScenarioKind = "unknown_command"
	ScenarioInvalidForm    ScenarioKind = "invalid_form"
)

// ScenarioContract declares runner-owned negative or synthetic behavior that
// cannot honestly be derived as a registered inventory item. Every declaration
// is required and is bound to the full inventory hash through SuiteContract.
type ScenarioContract struct {
	ID            string        `json:"id"`
	Kind          ScenarioKind  `json:"kind"`
	Surface       Surface       `json:"surface"`
	EvidenceClass EvidenceClass `json:"evidence_class"`
	Description   string        `json:"description"`
}

// SurfaceApplicability is an operator-reviewed overlay for a surface whose UX
// is not mechanically equivalent to the runtime registry. The full mapping is
// hash-bound to the suite so a runner cannot silently invent native coverage
// or skip a terminal-only capability.
type SurfaceApplicability struct {
	ItemID       string       `json:"item_id"`
	Surface      Surface      `json:"surface"`
	Availability Availability `json:"availability"`
	SourceRefs   []string     `json:"source_refs"`
	UXRationale  string       `json:"ux_rationale"`
}

type SuiteContract struct {
	SchemaVersion        string                 `json:"schema_version"`
	InventoryHash        string                 `json:"inventory_hash"`
	Hash                 string                 `json:"hash"`
	Scenarios            []ScenarioContract     `json:"scenarios"`
	SurfaceApplicability []SurfaceApplicability `json:"surface_applicability"`
}

func CompileSuiteContract(compiled Compiled, scenarios []ScenarioContract, applicability []SurfaceApplicability) (SuiteContract, error) {
	copy := append([]ScenarioContract(nil), scenarios...)
	seen := make(map[string]struct{}, len(copy))
	for i := range copy {
		copy[i].ID = strings.TrimSpace(copy[i].ID)
		copy[i].Description = strings.TrimSpace(copy[i].Description)
		if !stableScenarioID(copy[i].ID) || copy[i].Description == "" {
			return SuiteContract{}, fmt.Errorf("suite scenario requires stable ID and description")
		}
		if copy[i].Kind != ScenarioUnknownCommand && copy[i].Kind != ScenarioInvalidForm {
			return SuiteContract{}, fmt.Errorf("suite scenario %q has unsupported kind %q", copy[i].ID, copy[i].Kind)
		}
		if !supportedSurface(copy[i].Surface) {
			return SuiteContract{}, fmt.Errorf("suite scenario %q has unsupported surface %q", copy[i].ID, copy[i].Surface)
		}
		if !supportedEvidenceClass(copy[i].EvidenceClass) {
			return SuiteContract{}, fmt.Errorf("suite scenario %q has unsupported evidence class %q", copy[i].ID, copy[i].EvidenceClass)
		}
		if copy[i].EvidenceClass == EvidenceClassLocal && copy[i].Surface != SurfaceTUI {
			return SuiteContract{}, fmt.Errorf("suite scenario %q cannot use local evidence outside the TUI surface", copy[i].ID)
		}
		if _, duplicate := seen[copy[i].ID]; duplicate {
			return SuiteContract{}, fmt.Errorf("duplicate suite scenario %q", copy[i].ID)
		}
		seen[copy[i].ID] = struct{}{}
	}
	sort.Slice(copy, func(i, j int) bool { return copy[i].ID < copy[j].ID })
	applicabilityCopy, err := canonicalApplicability(compiled, applicability)
	if err != nil {
		return SuiteContract{}, err
	}
	canonical, err := json.Marshal(struct {
		SchemaVersion        string                 `json:"schema_version"`
		InventoryHash        string                 `json:"inventory_hash"`
		Scenarios            []ScenarioContract     `json:"scenarios"`
		SurfaceApplicability []SurfaceApplicability `json:"surface_applicability"`
	}{SchemaVersion: SchemaVersion, InventoryHash: compiled.Hash, Scenarios: copy, SurfaceApplicability: applicabilityCopy})
	if err != nil {
		return SuiteContract{}, fmt.Errorf("marshal suite contract: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return SuiteContract{SchemaVersion: SchemaVersion, InventoryHash: compiled.Hash, Hash: hex.EncodeToString(sum[:]), Scenarios: copy, SurfaceApplicability: applicabilityCopy}, nil
}

func canonicalApplicability(compiled Compiled, mappings []SurfaceApplicability) ([]SurfaceApplicability, error) {
	available := make(map[string]struct{})
	for _, item := range compiled.Items {
		if item.Availability == Available {
			available[item.ID] = struct{}{}
		}
	}
	copy := append([]SurfaceApplicability(nil), mappings...)
	seen := make(map[string]struct{}, len(copy))
	for i := range copy {
		copy[i].ItemID = strings.TrimSpace(copy[i].ItemID)
		copy[i].UXRationale = strings.TrimSpace(copy[i].UXRationale)
		copy[i].SourceRefs = sortedCopy(copy[i].SourceRefs)
		if _, found := available[copy[i].ItemID]; !found {
			return nil, fmt.Errorf("native applicability references unknown inventory item %q", copy[i].ItemID)
		}
		if copy[i].Surface != SurfaceNativeGUI {
			return nil, fmt.Errorf("applicability for %q must target native_gui", copy[i].ItemID)
		}
		if copy[i].Availability != Available && copy[i].Availability != NotApplicable {
			return nil, fmt.Errorf("native applicability for %q has invalid availability", copy[i].ItemID)
		}
		if len(copy[i].SourceRefs) == 0 || copy[i].UXRationale == "" {
			return nil, fmt.Errorf("native applicability for %q requires source refs and UX rationale", copy[i].ItemID)
		}
		for _, ref := range copy[i].SourceRefs {
			if strings.TrimSpace(ref) == "" {
				return nil, fmt.Errorf("native applicability for %q has an empty source ref", copy[i].ItemID)
			}
		}
		if _, duplicate := seen[copy[i].ItemID]; duplicate {
			return nil, fmt.Errorf("duplicate native applicability mapping for %q", copy[i].ItemID)
		}
		seen[copy[i].ItemID] = struct{}{}
	}
	for itemID := range available {
		if _, found := seen[itemID]; !found {
			return nil, fmt.Errorf("missing native applicability mapping for %q", itemID)
		}
	}
	sort.Slice(copy, func(i, j int) bool { return copy[i].ItemID < copy[j].ItemID })
	return copy, nil
}

func stableScenarioID(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

// ValidateCases proves that every resolved capability has one intent case and
// that no stale or duplicate case can be hidden in the matrix.
func ValidateCases(compiled Compiled, cases []Case) error {
	return validateCases(compiled, cases, nil)
}

// ValidateCasesForSurface proves completeness for one independently executed
// surface while retaining the full compiled inventory and its hash. Cases for
// another surface are rejected rather than silently filtered.
func ValidateCasesForSurface(compiled Compiled, cases []Case, surface Surface) error {
	if !supportedSurface(surface) {
		return fmt.Errorf("unsupported selected surface %q", surface)
	}
	return validateCases(compiled, cases, &surface)
}

func validateCases(compiled Compiled, cases []Case, selected *Surface) error {
	items := make(map[string]Item, len(compiled.Items))
	for _, item := range compiled.Items {
		items[item.ID] = item
	}
	seen := make(map[string]struct{}, len(cases))
	for _, c := range cases {
		if strings.TrimSpace(c.ScenarioID) != "" {
			return fmt.Errorf("synthetic scenario %q requires a suite contract", c.ScenarioID)
		}
		item, found := items[c.ItemID]
		if !found {
			return fmt.Errorf("case references unknown inventory item %q", c.ItemID)
		}
		if item.Availability != Available {
			return fmt.Errorf("case %q targets not-applicable inventory item", c.ItemID)
		}
		if err := validateCase(c); err != nil {
			return err
		}
		if selected != nil && (len(c.Surfaces) != 1 || c.Surfaces[0] != *selected) {
			return fmt.Errorf("case %q must map only the selected surface %q", c.ItemID, *selected)
		}
		for _, surface := range c.Surfaces {
			if !containsSurface(item.Surfaces, surface) {
				return fmt.Errorf("case %q targets inapplicable surface %q", c.ItemID, surface)
			}
			if err := validateInvocation(item, surface, c.InvocationID); err != nil {
				return err
			}
			if c.EvidenceClass == EvidenceClassLocal && item.Kind != TUICommandKind {
				return fmt.Errorf("case %q cannot use local evidence for non-command inventory", c.ItemID)
			}
			key := caseKey(c.ItemID, surface, c.InvocationID)
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate case for inventory invocation %q", key)
			}
			seen[key] = struct{}{}
		}
	}
	for _, item := range compiled.Items {
		if item.Availability == Available {
			if selected != nil {
				if !containsSurface(item.Surfaces, *selected) {
					continue
				}
				for _, invocationID := range requiredInvocationIDs(item, *selected) {
					key := caseKey(item.ID, *selected, invocationID)
					if _, covered := seen[key]; !covered {
						return fmt.Errorf("missing acceptance case for %q", key)
					}
				}
				continue
			}
			for _, surface := range item.Surfaces {
				for _, invocationID := range requiredInvocationIDs(item, surface) {
					key := caseKey(item.ID, surface, invocationID)
					if _, covered := seen[key]; !covered {
						return fmt.Errorf("missing acceptance case for %q", key)
					}
				}
			}
		}
	}
	return nil
}

func validateCase(c Case) error {
	itemID := strings.TrimSpace(c.ItemID)
	scenarioID := strings.TrimSpace(c.ScenarioID)
	if (itemID == "") == (scenarioID == "") {
		return fmt.Errorf("case must reference exactly one inventory item or suite scenario")
	}
	label := itemID
	if label == "" {
		label = scenarioID
	}
	if len(c.Surfaces) == 0 || len(c.OrderedActions) == 0 || len(c.ExpectedPostconditions) == 0 || strings.TrimSpace(c.Cleanup) == "" || !supportedEvidenceClass(c.EvidenceClass) {
		return fmt.Errorf("case %q requires surfaces, evidence class, ordered actions, expected postconditions, and cleanup", label)
	}
	seenAssertions := make(map[string]struct{}, len(c.ExpectedPostconditions))
	for _, postcondition := range c.ExpectedPostconditions {
		if !supportedPostconditionKind(postcondition.Kind) || strings.TrimSpace(postcondition.Probe) == "" || strings.TrimSpace(postcondition.AssertionID) == "" || strings.TrimSpace(postcondition.Description) == "" {
			return fmt.Errorf("case %q has an invalid typed expected postcondition", label)
		}
		if _, duplicate := seenAssertions[postcondition.AssertionID]; duplicate {
			return fmt.Errorf("case %q has duplicate expected assertion %q", label, postcondition.AssertionID)
		}
		seenAssertions[postcondition.AssertionID] = struct{}{}
	}
	return nil
}

func ValidateSuiteCasesForSurface(compiled Compiled, contract SuiteContract, cases []Case, surface Surface) error {
	if !supportedSurface(surface) {
		return fmt.Errorf("unsupported selected surface %q", surface)
	}
	contract, err := canonicalSuiteContract(compiled, contract)
	if err != nil {
		return err
	}
	var inventoryCases, scenarioCases []Case
	for _, c := range cases {
		if strings.TrimSpace(c.ScenarioID) == "" {
			inventoryCases = append(inventoryCases, c)
		} else {
			scenarioCases = append(scenarioCases, c)
		}
	}
	effective := compiledForSuiteSurface(compiled, contract, surface)
	if err := ValidateCasesForSurface(effective, inventoryCases, surface); err != nil {
		return err
	}
	declared := make(map[string]ScenarioContract, len(contract.Scenarios))
	for _, scenario := range contract.Scenarios {
		declared[scenario.ID] = scenario
	}
	seen := make(map[string]struct{}, len(scenarioCases))
	for _, c := range scenarioCases {
		if err := validateCase(c); err != nil {
			return err
		}
		scenario, found := declared[c.ScenarioID]
		if !found {
			return fmt.Errorf("case references undeclared synthetic scenario %q", c.ScenarioID)
		}
		if c.ItemID != "" || c.InvocationID != "" {
			return fmt.Errorf("synthetic scenario %q cannot claim inventory item or invocation identity", c.ScenarioID)
		}
		if scenario.Surface != surface || len(c.Surfaces) != 1 || c.Surfaces[0] != surface {
			return fmt.Errorf("synthetic scenario %q must map only selected surface %q", c.ScenarioID, surface)
		}
		if c.EvidenceClass != scenario.EvidenceClass {
			return fmt.Errorf("synthetic scenario %q evidence class does not match suite contract", c.ScenarioID)
		}
		if _, duplicate := seen[c.ScenarioID]; duplicate {
			return fmt.Errorf("duplicate case for synthetic scenario %q", c.ScenarioID)
		}
		seen[c.ScenarioID] = struct{}{}
	}
	for _, scenario := range contract.Scenarios {
		if scenario.Surface == surface {
			if _, covered := seen[scenario.ID]; !covered {
				return fmt.Errorf("missing acceptance case for required synthetic scenario %q", scenario.ID)
			}
		}
	}
	return nil
}

func canonicalSuiteContract(compiled Compiled, contract SuiteContract) (SuiteContract, error) {
	if contract.SchemaVersion != SchemaVersion || contract.InventoryHash != compiled.Hash || strings.TrimSpace(contract.Hash) == "" {
		return SuiteContract{}, fmt.Errorf("suite contract version or inventory hash does not match compiled inventory")
	}
	rebuilt, err := CompileSuiteContract(compiled, contract.Scenarios, contract.SurfaceApplicability)
	if err != nil {
		return SuiteContract{}, err
	}
	if rebuilt.Hash != contract.Hash {
		return SuiteContract{}, fmt.Errorf("suite contract hash does not match its scenario catalog")
	}
	return rebuilt, nil
}

func compiledForSuiteSurface(compiled Compiled, contract SuiteContract, surface Surface) Compiled {
	if surface != SurfaceNativeGUI {
		return compiled
	}
	availability := make(map[string]SurfaceApplicability, len(contract.SurfaceApplicability))
	for _, mapping := range contract.SurfaceApplicability {
		availability[mapping.ItemID] = mapping
	}
	effective := compiled
	effective.Items = make([]Item, 0, len(compiled.Items))
	for _, item := range compiled.Items {
		copy := item
		if item.Availability == Available {
			mapping := availability[item.ID]
			copy.Surfaces = []Surface{SurfaceNativeGUI}
			copy.Availability = mapping.Availability
			if mapping.Availability == NotApplicable {
				copy.Reason = mapping.UXRationale
			}
		} else if !containsSurface(copy.Surfaces, SurfaceNativeGUI) {
			copy.Surfaces = append(copy.Surfaces, SurfaceNativeGUI)
			copy.Surfaces = sortedSurfaces(copy.Surfaces)
		}
		effective.Items = append(effective.Items, copy)
	}
	return effective
}

func validateInvocation(item Item, surface Surface, invocationID string) error {
	for _, required := range requiredInvocationIDs(item, surface) {
		if required == invocationID {
			return nil
		}
	}
	return fmt.Errorf("case %q has undeclared invocation %q for surface %q", item.ID, invocationID, surface)
}

func requiredInvocationIDs(item Item, surface Surface) []string {
	if item.Kind != TUICommandKind || surface != SurfaceTUI {
		return []string{""}
	}
	result := make([]string, 0, len(item.Invocations))
	for _, invocation := range item.Invocations {
		result = append(result, invocation.ID)
	}
	return result
}

func caseKey(itemID string, surface Surface, invocationID string) string {
	key := itemID + "@" + string(surface)
	if invocationID != "" {
		key += "#" + invocationID
	}
	return key
}

func supportedSurface(surface Surface) bool {
	return surface == SurfaceAPI || surface == SurfaceTUI || surface == SurfaceNativeGUI
}

func supportedEvidenceClass(class EvidenceClass) bool {
	return class == EvidenceClassConversation || class == EvidenceClassLocal
}

func supportedPostconditionKind(kind PostconditionKind) bool {
	return kind == PostconditionRenderedScreen || kind == PostconditionDurableState || kind == PostconditionConversationState || kind == PostconditionExternalState
}

type CleanupEvidence struct {
	Verified bool   `json:"verified"`
	Detail   string `json:"detail"`
}

type Timing struct {
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

// ProbeObservation is the external measurement made by a runner. It is kept
// separate from Postcondition so copying an expectation into a transcript or a
// model reply can never be accepted as proof.
type ProbeObservation struct {
	Kind        PostconditionKind `json:"kind"`
	Probe       string            `json:"probe"`
	AssertionID string            `json:"assertion_id"`
	Value       string            `json:"value"`
	Verified    bool              `json:"verified"`
}

type ArtifactKind string

const (
	ArtifactEventLog        ArtifactKind = "event_log"
	ArtifactTranscript      ArtifactKind = "transcript"
	ArtifactScreenshot      ArtifactKind = "screenshot"
	ArtifactAXSnapshot      ArtifactKind = "ax_snapshot"
	ArtifactRawSSEEvent     ArtifactKind = "raw_sse_event"
	ArtifactAPIStoreProbe   ArtifactKind = "api_store_probe"
	ArtifactProbe           ArtifactKind = "probe"
	ArtifactStateSnapshot   ArtifactKind = "state_snapshot"
	ArtifactTerminalCapture ArtifactKind = "terminal_capture"
)

type ArtifactRef struct {
	Kind     ArtifactKind `json:"kind"`
	Path     string       `json:"path"`
	Digest   string       `json:"digest"`
	Redacted *bool        `json:"redacted"`
}

// EnvironmentMetadata binds native GUI proof to the exact installed build and
// isolated daemon/workspace used by the operator. Zero values are deliberately
// invalid for a passing native record.
type EnvironmentMetadata struct {
	BuildSHA          string `json:"build_sha"`
	BundlePath        string `json:"bundle_path"`
	DaemonPID         int    `json:"daemon_pid"`
	DaemonPort        int    `json:"daemon_port"`
	WorkspacePath     string `json:"workspace_path"`
	WorkspaceIsolated bool   `json:"workspace_isolated"`
}

// Evidence is the versioned record emitted by a future runner. A pass requires
// a successful external probe matching its case's expected postcondition.
type Evidence struct {
	SchemaVersion          string              `json:"schema_version"`
	InventoryHash          string              `json:"inventory_hash"`
	SuiteHash              string              `json:"suite_hash,omitempty"`
	ItemID                 string              `json:"item_id,omitempty"`
	ScenarioID             string              `json:"scenario_id,omitempty"`
	InvocationID           string              `json:"invocation_id,omitempty"`
	Surface                Surface             `json:"surface"`
	EvidenceClass          EvidenceClass       `json:"evidence_class"`
	Outcome                Outcome             `json:"outcome"`
	OrderedActions         []Action            `json:"ordered_actions"`
	RunID                  string              `json:"run_id,omitempty"`
	ConversationID         string              `json:"conversation_id,omitempty"`
	EventIDs               []string            `json:"event_ids,omitempty"`
	ExpectedPostconditions []Postcondition     `json:"expected_postconditions"`
	ObservedPostconditions []ProbeObservation  `json:"observed_postconditions"`
	Artifacts              []ArtifactRef       `json:"artifacts"`
	Environment            EnvironmentMetadata `json:"environment"`
	Cleanup                CleanupEvidence     `json:"cleanup"`
	Timing                 Timing              `json:"timing"`
	FailureClass           string              `json:"failure_class,omitempty"`
}

// ValidateEvidence rejects records that are not comparable to the inventory or
// whose claimed success amounts only to a tool event or assistant narration.
func ValidateEvidence(compiled Compiled, c Case, evidence Evidence) error {
	if err := validateCase(c); err != nil {
		return err
	}
	if c.ScenarioID != "" || evidence.ScenarioID != "" || evidence.SuiteHash != "" {
		return fmt.Errorf("synthetic or suite-bound evidence requires ValidateSuiteEvidence")
	}
	var item Item
	found := false
	for _, candidate := range compiled.Items {
		if candidate.ID == c.ItemID {
			item = candidate
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("evidence case references unknown inventory item %q", c.ItemID)
	}
	if item.Availability != Available {
		return fmt.Errorf("evidence case %q targets not-applicable inventory item", c.ItemID)
	}
	if c.EvidenceClass == EvidenceClassLocal && item.Kind != TUICommandKind {
		return fmt.Errorf("case %q cannot use local evidence for non-command inventory", c.ItemID)
	}
	for _, surface := range c.Surfaces {
		if !containsSurface(item.Surfaces, surface) {
			return fmt.Errorf("evidence case %q targets inapplicable surface %q", c.ItemID, surface)
		}
		if err := validateInvocation(item, surface, c.InvocationID); err != nil {
			return err
		}
	}
	return validateEvidenceFields(compiled.Hash, "", c, evidence)
}

func ValidateSuiteEvidence(compiled Compiled, contract SuiteContract, c Case, evidence Evidence) error {
	contract, err := canonicalSuiteContract(compiled, contract)
	if err != nil {
		return err
	}
	if err := validateCase(c); err != nil {
		return err
	}
	if c.ScenarioID == "" {
		if evidence.SuiteHash != contract.Hash {
			return fmt.Errorf("suite evidence hash does not match suite contract")
		}
		copy := evidence
		copy.SuiteHash = ""
		return ValidateEvidence(compiledForSuiteSurface(compiled, contract, evidence.Surface), c, copy)
	}
	var scenario ScenarioContract
	found := false
	for _, candidate := range contract.Scenarios {
		if candidate.ID == c.ScenarioID {
			scenario = candidate
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("case references undeclared synthetic scenario %q", c.ScenarioID)
	}
	if c.ItemID != "" || c.InvocationID != "" || len(c.Surfaces) != 1 || c.Surfaces[0] != scenario.Surface || c.EvidenceClass != scenario.EvidenceClass {
		return fmt.Errorf("synthetic scenario %q case does not match suite contract", c.ScenarioID)
	}
	return validateEvidenceFields(compiled.Hash, contract.Hash, c, evidence)
}

func validateEvidenceFields(inventoryHash, suiteHash string, c Case, evidence Evidence) error {
	label := c.ItemID
	if label == "" {
		label = c.ScenarioID
	}
	if evidence.SchemaVersion != SchemaVersion || evidence.InventoryHash != inventoryHash {
		return fmt.Errorf("evidence inventory version or hash does not match compiled inventory")
	}
	if evidence.SuiteHash != suiteHash {
		return fmt.Errorf("evidence suite hash does not match suite contract")
	}
	if evidence.ItemID != c.ItemID || evidence.ScenarioID != c.ScenarioID || evidence.InvocationID != c.InvocationID {
		return fmt.Errorf("evidence target identity does not match case %q", label)
	}
	if !containsSurface(c.Surfaces, evidence.Surface) {
		return fmt.Errorf("evidence surface %q is not applicable to %q", evidence.Surface, label)
	}
	if evidence.EvidenceClass != c.EvidenceClass {
		return fmt.Errorf("evidence class does not match case %q", label)
	}
	if !equalActions(c.OrderedActions, evidence.OrderedActions) {
		return fmt.Errorf("evidence ordered actions do not match case %q", label)
	}
	if len(evidence.OrderedActions) == 0 || len(evidence.Artifacts) == 0 || evidence.Timing.StartedAt.IsZero() || evidence.Timing.FinishedAt.IsZero() || evidence.Timing.FinishedAt.Before(evidence.Timing.StartedAt) {
		return fmt.Errorf("evidence for %q is missing ordered actions, typed artifacts, or valid timing", label)
	}
	if err := validateArtifacts(evidence.Artifacts); err != nil {
		return fmt.Errorf("evidence for %q: %w", label, err)
	}
	if evidence.Outcome != Pass && evidence.Outcome != Fail {
		return fmt.Errorf("evidence for %q has invalid outcome %q", label, evidence.Outcome)
	}
	if evidence.EvidenceClass == EvidenceClassLocal && (strings.TrimSpace(evidence.RunID) != "" || strings.TrimSpace(evidence.ConversationID) != "" || len(evidence.EventIDs) != 0) {
		return fmt.Errorf("local evidence for %q must not fabricate runtime identities", label)
	}
	if evidence.Outcome == Pass {
		if evidence.EvidenceClass == EvidenceClassConversation && (strings.TrimSpace(evidence.RunID) == "" || strings.TrimSpace(evidence.ConversationID) == "" || len(evidence.EventIDs) == 0) {
			return fmt.Errorf("pass evidence for %q requires run, conversation, and event identities", label)
		}
		if !equalPostconditions(evidence.ExpectedPostconditions, c.ExpectedPostconditions) {
			return fmt.Errorf("pass evidence for %q does not carry all case postcondition contracts", label)
		}
		if err := validateProbeObservations(c.ExpectedPostconditions, evidence.ObservedPostconditions); err != nil {
			return fmt.Errorf("pass evidence for %q does not prove expected postconditions: %w", label, err)
		}
		if !evidence.Cleanup.Verified || strings.TrimSpace(evidence.Cleanup.Detail) == "" {
			return fmt.Errorf("pass evidence for %q requires verified cleanup", label)
		}
		if evidence.Surface == SurfaceNativeGUI {
			if err := validateNativePassProof(evidence); err != nil {
				return fmt.Errorf("native pass evidence for %q: %w", label, err)
			}
		}
	}
	if evidence.Outcome == Fail && strings.TrimSpace(evidence.FailureClass) == "" {
		return fmt.Errorf("failed evidence for %q requires a failure class", label)
	}
	return nil
}

func validateArtifacts(artifacts []ArtifactRef) error {
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if !supportedArtifactKind(artifact.Kind) || strings.TrimSpace(artifact.Path) == "" {
			return fmt.Errorf("typed artifact requires supported kind and path")
		}
		if artifact.Redacted == nil {
			return fmt.Errorf("artifact %q requires an explicit redaction declaration", artifact.Path)
		}
		if !validSHA256Digest(artifact.Digest) {
			return fmt.Errorf("artifact %q requires a sha256 digest", artifact.Path)
		}
		key := string(artifact.Kind) + "@" + artifact.Path
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("duplicate artifact reference %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func supportedArtifactKind(kind ArtifactKind) bool {
	return kind == ArtifactEventLog || kind == ArtifactTranscript || kind == ArtifactScreenshot || kind == ArtifactAXSnapshot || kind == ArtifactRawSSEEvent || kind == ArtifactAPIStoreProbe || kind == ArtifactProbe || kind == ArtifactStateSnapshot || kind == ArtifactTerminalCapture
}

func validateNativePassProof(evidence Evidence) error {
	required := map[ArtifactKind]bool{
		ArtifactScreenshot:    false,
		ArtifactAXSnapshot:    false,
		ArtifactRawSSEEvent:   false,
		ArtifactAPIStoreProbe: false,
	}
	for _, artifact := range evidence.Artifacts {
		if _, exists := required[artifact.Kind]; exists {
			required[artifact.Kind] = true
		}
	}
	for kind, present := range required {
		if !present {
			return fmt.Errorf("native pass requires artifact kind %q", kind)
		}
	}
	env := evidence.Environment
	if !validGitSHA(env.BuildSHA) || !strings.HasPrefix(env.BundlePath, "/") || !strings.HasSuffix(env.BundlePath, ".app") {
		return fmt.Errorf("native pass requires build SHA and absolute app bundle path")
	}
	if env.DaemonPID <= 0 || env.DaemonPort <= 0 || env.DaemonPort > 65535 {
		return fmt.Errorf("native pass requires daemon PID and port")
	}
	if strings.TrimSpace(env.WorkspacePath) == "" || !strings.HasPrefix(env.WorkspacePath, "/") || !env.WorkspaceIsolated {
		return fmt.Errorf("native pass requires explicit workspace isolation")
	}
	return nil
}

func validGitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validSHA256Digest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func equalPostconditions(left, right []Postcondition) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func validateProbeObservations(expected []Postcondition, observed []ProbeObservation) error {
	if len(expected) != len(observed) {
		return fmt.Errorf("got %d observations for %d assertions", len(observed), len(expected))
	}
	expectedByID := make(map[string]Postcondition, len(expected))
	for _, postcondition := range expected {
		expectedByID[postcondition.AssertionID] = postcondition
	}
	seen := make(map[string]struct{}, len(observed))
	for _, observation := range observed {
		postcondition, found := expectedByID[observation.AssertionID]
		if !found || observation.Kind != postcondition.Kind || observation.Probe != postcondition.Probe || strings.TrimSpace(observation.Value) == "" || !observation.Verified {
			return fmt.Errorf("observation %q does not independently verify its typed assertion", observation.AssertionID)
		}
		if _, duplicate := seen[observation.AssertionID]; duplicate {
			return fmt.Errorf("duplicate observation for assertion %q", observation.AssertionID)
		}
		seen[observation.AssertionID] = struct{}{}
	}
	return nil
}

func equalActions(left, right []Action) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func containsSurface(surfaces []Surface, surface Surface) bool {
	for _, candidate := range surfaces {
		if candidate == surface {
			return true
		}
	}
	return false
}

// RenderMarkdown creates the human-readable inventory artifact from the same
// compiled value whose hash a machine-readable evidence record carries.
func RenderMarkdown(compiled Compiled) string {
	report, _ := RenderResultMarkdown(compiled, nil, nil)
	return report
}

// RenderResultMarkdown makes the three terminal report statuses visible:
// pass/fail from evidence and not-applicable from condition resolution. Items
// which have not yet been executed remain explicitly pending.
func RenderResultMarkdown(compiled Compiled, cases []Case, evidence []Evidence) (string, error) {
	caseByItemSurface := make(map[string]Case)
	for _, caseSpec := range cases {
		if err := validateCase(caseSpec); err != nil {
			return "", err
		}
		if caseSpec.ScenarioID != "" {
			return "", fmt.Errorf("synthetic scenario %q requires a suite-aware renderer", caseSpec.ScenarioID)
		}
		for _, surface := range caseSpec.Surfaces {
			key := caseKey(caseSpec.ItemID, surface, caseSpec.InvocationID)
			if _, duplicate := caseByItemSurface[key]; duplicate {
				return "", fmt.Errorf("duplicate render case for inventory item/surface %q", key)
			}
			caseByItemSurface[key] = caseSpec
		}
	}
	results := make(map[string]Outcome, len(evidence))
	for _, record := range evidence {
		key := caseKey(record.ItemID, record.Surface, record.InvocationID)
		caseSpec, found := caseByItemSurface[key]
		if !found {
			return "", fmt.Errorf("evidence for %q has no acceptance case", key)
		}
		if err := ValidateEvidence(compiled, caseSpec, record); err != nil {
			return "", fmt.Errorf("validate evidence for %q: %w", key, err)
		}
		if prior, exists := results[key]; !exists || record.Outcome == Fail || prior != Fail {
			results[key] = record.Outcome
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Acceptance Inventory\n\nSchema: `%s`  \nHash: `%s`\n\n", compiled.SchemaVersion, compiled.Hash)
	b.WriteString("| ID | Surface | Invocation | Owner | Condition | Availability | Result | Notes |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, item := range compiled.Items {
		notes := item.Reason
		if notes == "" {
			notes = item.Description
		}
		if item.Provenance != nil {
			resolver := fmt.Sprintf("resolver=%s provider=%s individual_names_known=%t", item.Provenance.Source, item.Provenance.Provider, item.Provenance.IndividualNamesKnown)
			if notes == "" {
				notes = resolver
			} else {
				notes += "; " + resolver
			}
		}
		for _, surface := range item.Surfaces {
			for _, invocationID := range requiredInvocationIDs(item, surface) {
				result := "pending"
				if item.Availability == NotApplicable {
					result = string(NotApplicable)
				}
				if observed, found := results[caseKey(item.ID, surface, invocationID)]; found {
					result = string(observed)
				}
				invocation := "-"
				if invocationID != "" {
					invocation = invocationID
				}
				fmt.Fprintf(&b, "| %q | %s | %q | %q | %s | %s | %s | %s |\n", item.ID, surface, invocation, item.Owner, item.Condition, item.Availability, result, strings.ReplaceAll(notes, "|", "\\|"))
			}
		}
	}
	return b.String(), nil
}

// RenderSuiteResultMarkdown validates and renders both inventory-derived rows
// and runner-owned synthetic scenarios. The suite hash remains bound to the
// full compiled inventory; scenario rows never masquerade as registry items.
func RenderSuiteResultMarkdown(compiled Compiled, contract SuiteContract, cases []Case, evidence []Evidence) (string, error) {
	contract, err := canonicalSuiteContract(compiled, contract)
	if err != nil {
		return "", err
	}
	for _, surface := range []Surface{SurfaceAPI, SurfaceTUI, SurfaceNativeGUI} {
		var surfaceCases []Case
		for _, c := range cases {
			if containsSurface(c.Surfaces, surface) {
				surfaceCases = append(surfaceCases, c)
			}
		}
		if err := ValidateSuiteCasesForSurface(compiled, contract, surfaceCases, surface); err != nil {
			return "", err
		}
	}
	var inventoryCases []Case
	for _, c := range cases {
		if c.ScenarioID == "" && !containsSurface(c.Surfaces, SurfaceNativeGUI) {
			inventoryCases = append(inventoryCases, c)
		}
	}
	var inventoryEvidence []Evidence
	nativeResults := make(map[string]Outcome)
	scenarioResults := make(map[string]Outcome)
	for _, record := range evidence {
		var caseSpec Case
		found := false
		for _, candidate := range cases {
			if candidate.ItemID == record.ItemID && candidate.ScenarioID == record.ScenarioID && candidate.InvocationID == record.InvocationID && containsSurface(candidate.Surfaces, record.Surface) {
				caseSpec = candidate
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("suite evidence target has no acceptance case")
		}
		if err := ValidateSuiteEvidence(compiled, contract, caseSpec, record); err != nil {
			return "", err
		}
		if record.ScenarioID == "" {
			if record.Surface == SurfaceNativeGUI {
				nativeResults[record.ItemID] = record.Outcome
			} else {
				record.SuiteHash = ""
				inventoryEvidence = append(inventoryEvidence, record)
			}
		} else {
			scenarioResults[record.ScenarioID] = record.Outcome
		}
	}
	report, err := RenderResultMarkdown(compiled, inventoryCases, inventoryEvidence)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(report)
	b.WriteString("\n## Native GUI Applicability\n\n| ID | Availability | Result | Source refs | UX rationale |\n| --- | --- | --- | --- | --- |\n")
	for _, mapping := range contract.SurfaceApplicability {
		result := "pending"
		if mapping.Availability == NotApplicable {
			result = string(NotApplicable)
		} else if observed, found := nativeResults[mapping.ItemID]; found {
			result = string(observed)
		}
		fmt.Fprintf(&b, "| %q | %s | %s | %s | %s |\n", mapping.ItemID, mapping.Availability, result, strings.Join(mapping.SourceRefs, ", "), strings.ReplaceAll(mapping.UXRationale, "|", "\\|"))
	}
	b.WriteString("\n## Synthetic Scenarios\n\n| ID | Surface | Kind | Evidence class | Result | Description |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, scenario := range contract.Scenarios {
		result := "pending"
		if observed, found := scenarioResults[scenario.ID]; found {
			result = string(observed)
		}
		fmt.Fprintf(&b, "| %q | %s | %s | %s | %s | %s |\n", scenario.ID, scenario.Surface, scenario.Kind, scenario.EvidenceClass, result, strings.ReplaceAll(scenario.Description, "|", "\\|"))
	}
	return b.String(), nil
}
