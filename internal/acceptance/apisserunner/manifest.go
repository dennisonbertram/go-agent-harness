package apisserunner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-agent-harness/internal/acceptance/inventory"
	"go-agent-harness/internal/acceptance/scheduledlifecycle"
)

// Manifest is the reviewed, intent-specific API overlay. It intentionally
// contains cases rather than a second list of tool names; Compile remains the
// only source of catalog identity and dynamic N/A records.
type Manifest struct {
	// InventoryHash pins these reviewed cases to the exact resolver-derived
	// catalog they were authored against. Case IDs alone cannot detect a
	// changed current catalog whose identifiers happen to remain stable.
	InventoryHash string `json:"inventory_hash"`
	// DaemonSourceSHA pins this manifest to the source revision that the
	// lifecycle-owned harness daemon was built from. The CLI compares it to the
	// lifecycle provenance artifact before accepting the live inventory.
	DaemonSourceSHA string `json:"daemon_source_sha"`
	// Mappings are a reviewed execution-plan overlay. They deliberately remain
	// distinct from Cases: a mapping assigns a future executor cohort, but does
	// not claim that the tool was executed successfully.
	Mappings []ToolMapping    `json:"mappings"`
	Cases    []inventory.Case `json:"cases"`
}

// DaemonProvenance is the identity written by scheduledlifecycle to its owned
// artifact root. An alias prevents this consumer from drifting from the
// lifecycle's authoritative source/listener/executable contract.
type DaemonProvenance = scheduledlifecycle.Provenance

// LoadDaemonProvenance reads the lifecycle provenance.json envelope without
// importing the lifecycle package. Its JSON shape is intentionally compatible
// with scheduledlifecycle's durable artifact.
func LoadDaemonProvenance(path string) (DaemonProvenance, error) {
	file, err := os.Open(path)
	if err != nil {
		return DaemonProvenance{}, err
	}
	defer file.Close()
	var artifact struct {
		Provenance DaemonProvenance `json:"provenance"`
	}
	if err := json.NewDecoder(file).Decode(&artifact); err != nil {
		return DaemonProvenance{}, fmt.Errorf("decode daemon provenance: %w", err)
	}
	if err := validateProvenanceExecutable(artifact.Provenance); err != nil {
		return DaemonProvenance{}, err
	}
	return artifact.Provenance, nil
}

// validateProvenanceExecutable repeats the lifecycle's canonical executable
// identity check at consumption time. A durable artifact can outlive or be
// edited after a daemon starts, so a report must not trust its stored path or
// digest without verifying the current file bytes.
func validateProvenanceExecutable(provenance DaemonProvenance) error {
	path := strings.TrimSpace(provenance.CommandPath)
	if path == "" || !filepath.IsAbs(path) {
		return fmt.Errorf("daemon command path must be absolute")
	}
	if filepath.Clean(path) != path {
		return fmt.Errorf("daemon command path %q is not canonical", path)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("canonicalize daemon command %q: %w", path, err)
	}
	if canonical != path {
		return fmt.Errorf("daemon command path %q is not canonical (resolved %q)", path, canonical)
	}
	bytes, err := os.ReadFile(canonical)
	if err != nil {
		return fmt.Errorf("read daemon command %q: %w", canonical, err)
	}
	sum := sha256.Sum256(bytes)
	if actual := hex.EncodeToString(sum[:]); provenance.CommandSHA256 == "" || provenance.CommandSHA256 != actual {
		return fmt.Errorf("daemon command SHA-256 %q does not match canonical executable SHA-256 %q", provenance.CommandSHA256, actual)
	}
	return nil
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
	InventoryHash       string   `json:"inventory_hash"`
	DaemonSourceSHA     string   `json:"daemon_source_sha"`
	DaemonCommandSHA256 string   `json:"daemon_command_sha256"`
	Available           int      `json:"available"`
	Mapped              int      `json:"mapped"`
	Planned             int      `json:"planned"`
	Excluded            []string `json:"excluded"`
	NotApplicable       []string `json:"not_applicable"`
	Missing             []string `json:"missing"`
}

func BuildCoverageReport(compiled inventory.Compiled, manifest Manifest, daemon DaemonProvenance, harnessURL string) (CoverageReport, error) {
	if err := validateDaemonProvenance(manifest, daemon, harnessURL); err != nil {
		return CoverageReport{}, err
	}
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
	report := CoverageReport{InventoryHash: compiled.Hash, DaemonSourceSHA: daemon.SourceSHA, DaemonCommandSHA256: daemon.CommandSHA256}
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

func validateDaemonProvenance(manifest Manifest, daemon DaemonProvenance, harnessURL string) error {
	manifestSource := strings.TrimSpace(manifest.DaemonSourceSHA)
	if manifestSource == "" {
		return fmt.Errorf("manifest daemon source SHA is required")
	}
	if daemon.SourceSHA == "" || daemon.SourceSHA != manifestSource {
		return fmt.Errorf("daemon source SHA %q does not match manifest daemon source SHA %q", daemon.SourceSHA, manifestSource)
	}
	if strings.TrimSpace(daemon.CommandPath) == "" || strings.TrimSpace(daemon.CommandSHA256) == "" {
		return fmt.Errorf("daemon command path and SHA256 are required")
	}
	base, err := url.Parse(harnessURL)
	if err != nil || base.Scheme == "" || base.Host == "" || base.Path != "" && base.Path != "/" {
		return fmt.Errorf("invalid harness URL %q", harnessURL)
	}
	if daemon.Address != base.Host {
		return fmt.Errorf("daemon provenance address %q does not match harness URL address %q", daemon.Address, base.Host)
	}
	return nil
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
