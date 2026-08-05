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
	InventoryHash string           `json:"inventory_hash"`
	Cases         []inventory.Case `json:"cases"`
}

// CoverageReport is a machine-readable gap report. Missing is never silently
// treated as N/A: only the daemon's resolver-derived records appear in
// NotApplicable.
type CoverageReport struct {
	InventoryHash string   `json:"inventory_hash"`
	Available     int      `json:"available"`
	Planned       int      `json:"planned"`
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
		key := item.ID + "#"
		if _, found := planned[key]; found {
			report.Planned++
		} else {
			report.Missing = append(report.Missing, item.ID)
		}
	}
	sort.Strings(report.NotApplicable)
	sort.Strings(report.Missing)
	return report, nil
}

func contains(values []inventory.Surface, wanted inventory.Surface) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
