package apisserunner

import (
	"fmt"
	"sort"
	"strings"

	"go-agent-harness/internal/acceptance/inventory"
)

// Manifest is the reviewed, intent-specific API overlay. It intentionally
// contains cases rather than a second list of tool names; Compile remains the
// only source of catalog identity and dynamic N/A records.
type Manifest struct {
	// InventoryHash pins these reviewed cases to the exact resolver-derived
	// catalog they were authored against. Case IDs alone cannot detect a
	// changed current catalog whose identifiers happen to remain stable.
	InventoryHash string `json:"inventory_hash"`
	// Mappings are a reviewed execution-plan overlay. They deliberately remain
	// distinct from Cases: a mapping assigns a future executor cohort, but does
	// not claim that the tool was executed successfully.
	Mappings []ToolMapping    `json:"mappings"`
	Cases    []inventory.Case `json:"cases"`
}

type CoverageDisposition string

const (
	CoveragePlanned     CoverageDisposition = "planned"
	CoverageOutOfScope  CoverageDisposition = "out_of_scope"
	CoverageUnavailable CoverageDisposition = "unavailable"
)

// ToolMapping binds a reviewed cohort (or explicit exclusion) to the exact
// inventory item and its resolver provenance. A mapping cannot survive an
// identity, owner, condition, or inventory-hash drift.
type ToolMapping struct {
	ItemID      string              `json:"item_id"`
	Owner       string              `json:"owner"`
	Condition   string              `json:"condition"`
	Disposition CoverageDisposition `json:"disposition"`
	Cohort      string              `json:"cohort,omitempty"`
	Reason      string              `json:"reason,omitempty"`
}

// CoverageReport is a machine-readable gap report. Missing is never silently
// treated as N/A: only the daemon's resolver-derived records appear in
// NotApplicable.
type CoverageReport struct {
	InventoryHash string   `json:"inventory_hash"`
	Available     int      `json:"available"`
	Mapped        int      `json:"mapped"`
	Planned       int      `json:"planned"`
	Excluded      []string `json:"excluded"`
	NotApplicable []string `json:"not_applicable"`
	Missing       []string `json:"missing"`
}

func BuildCoverageReport(compiled inventory.Compiled, manifest Manifest) (CoverageReport, error) {
	if strings.TrimSpace(manifest.InventoryHash) == "" {
		return CoverageReport{}, fmt.Errorf("manifest inventory hash is required")
	}
	if manifest.InventoryHash != compiled.Hash {
		return CoverageReport{}, fmt.Errorf("manifest inventory hash %q does not match live inventory hash %q", manifest.InventoryHash, compiled.Hash)
	}
	if err := validateMappings(compiled, manifest.Mappings); err != nil {
		return CoverageReport{}, err
	}
	// Validate each claimed row against a projection of the real compiled
	// inventory. The projection deliberately excludes unrelated items so a
	// manifest can report gaps, while unknown, N/A, wrong-surface, duplicate,
	// or invalid-invocation entries still fail through the authoritative schema.
	selected := make(map[string]inventory.Item, len(manifest.Cases))
	for _, item := range compiled.Items {
		selected[item.ID] = item
	}
	projected := compiled
	projected.Items = nil
	for _, c := range manifest.Cases {
		if item, found := selected[c.ItemID]; found {
			projected.Items = append(projected.Items, item)
		}
	}
	if err := inventory.ValidateCasesForSurface(projected, manifest.Cases, inventory.SurfaceAPI); err != nil {
		return CoverageReport{}, fmt.Errorf("validate manifest against live inventory: %w", err)
	}
	planned := make(map[string]struct{}, len(manifest.Cases))
	for _, c := range manifest.Cases {
		if len(c.Surfaces) != 1 || c.Surfaces[0] != inventory.SurfaceAPI {
			return CoverageReport{}, fmt.Errorf("manifest case %q must target API only", c.ItemID)
		}
		key := c.ItemID + "#" + c.InvocationID
		if _, exists := planned[key]; exists {
			return CoverageReport{}, fmt.Errorf("duplicate API manifest case %q", key)
		}
		planned[key] = struct{}{}
	}
	mappings := make(map[string]ToolMapping, len(manifest.Mappings))
	for _, mapping := range manifest.Mappings {
		mappings[mapping.ItemID] = mapping
	}
	report := CoverageReport{InventoryHash: compiled.Hash}
	for _, item := range compiled.Items {
		if item.Availability == inventory.NotApplicable {
			report.NotApplicable = append(report.NotApplicable, item.ID+": "+item.Reason)
			continue
		}
		if !contains(item.Surfaces, inventory.SurfaceAPI) {
			continue
		}
		report.Available++
		if mapping := mappings[item.ID]; mapping.Disposition == CoveragePlanned {
			report.Mapped++
		} else {
			report.Excluded = append(report.Excluded, item.ID+": "+mapping.Reason)
		}
		key := item.ID + "#"
		if _, found := planned[key]; found {
			report.Planned++
		} else {
			report.Missing = append(report.Missing, item.ID)
		}
	}
	sort.Strings(report.NotApplicable)
	sort.Strings(report.Excluded)
	sort.Strings(report.Missing)
	return report, nil
}

func validateMappings(compiled inventory.Compiled, mappings []ToolMapping) error {
	items := make(map[string]inventory.Item, len(compiled.Items))
	required := make(map[string]struct{}, len(compiled.Items))
	for _, item := range compiled.Items {
		items[item.ID] = item
		if contains(item.Surfaces, inventory.SurfaceAPI) {
			required[item.ID] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		mapping.ItemID = strings.TrimSpace(mapping.ItemID)
		mapping.Owner = strings.TrimSpace(mapping.Owner)
		mapping.Condition = strings.TrimSpace(mapping.Condition)
		mapping.Cohort = strings.TrimSpace(mapping.Cohort)
		mapping.Reason = strings.TrimSpace(mapping.Reason)
		if mapping.ItemID == "" {
			return fmt.Errorf("coverage mapping item_id is required")
		}
		if _, duplicate := seen[mapping.ItemID]; duplicate {
			return fmt.Errorf("duplicate coverage mapping %q", mapping.ItemID)
		}
		seen[mapping.ItemID] = struct{}{}
		item, found := items[mapping.ItemID]
		if !found || !contains(item.Surfaces, inventory.SurfaceAPI) {
			return fmt.Errorf("coverage mapping %q is absent from live API inventory", mapping.ItemID)
		}
		if mapping.Owner != item.Owner {
			return fmt.Errorf("coverage mapping %q owner %q does not match live owner %q", mapping.ItemID, mapping.Owner, item.Owner)
		}
		if mapping.Condition != item.Condition {
			return fmt.Errorf("coverage mapping %q condition %q does not match live condition %q", mapping.ItemID, mapping.Condition, item.Condition)
		}
		if item.Availability == inventory.NotApplicable {
			if mapping.Disposition != CoverageUnavailable || mapping.Reason == "" {
				return fmt.Errorf("unavailable coverage mapping %q requires unavailable disposition and reason", mapping.ItemID)
			}
			continue
		}
		switch mapping.Disposition {
		case CoveragePlanned:
			if mapping.Cohort == "" {
				return fmt.Errorf("planned coverage mapping %q requires cohort", mapping.ItemID)
			}
		case CoverageOutOfScope:
			if mapping.Reason == "" {
				return fmt.Errorf("out-of-scope coverage mapping %q requires reason", mapping.ItemID)
			}
		default:
			return fmt.Errorf("available coverage mapping %q has invalid disposition %q", mapping.ItemID, mapping.Disposition)
		}
	}
	for itemID := range required {
		if _, found := seen[itemID]; !found {
			return fmt.Errorf("missing coverage mapping for %q", itemID)
		}
	}
	return nil
}

func contains(values []inventory.Surface, wanted inventory.Surface) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
