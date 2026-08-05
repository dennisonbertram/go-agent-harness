// Package nativegui validates the installed, rendered macOS acceptance lane.
// It deliberately does not drive the product UI: a driver must provide real
// screenshot/AX/OCR and daemon artifacts, which are then bound to #1086's
// inventory and checked again from disk before a PASS can be rendered.
package nativegui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go-agent-harness/internal/acceptance/inventory"
)

type Manifest struct {
	Contract   inventory.SuiteContract `json:"contract"`
	Cases      []inventory.Case        `json:"cases"`
	Evidence   []inventory.Evidence    `json:"evidence"`
	Collection CollectionProvenance    `json:"collection"`
}

// CollectionProvenance binds a proof pack to one launcher-created isolated
// collection. The launcher generates the nonce and supplies the exact local
// app/daemon values to its repository-owned driver; a copied report or an
// arbitrary remote driver cannot qualify as native acceptance evidence.
type CollectionProvenance struct {
	Launcher       string                    `json:"launcher"`
	Nonce          string                    `json:"nonce"`
	TempRoot       string                    `json:"temp_root"`
	ArtifactRoot   string                    `json:"artifact_root"`
	RepositoryRoot string                    `json:"repository_root"`
	DriverPath     string                    `json:"driver_path"`
	DriverDigest   string                    `json:"driver_digest"`
	AppBundlePath  string                    `json:"app_bundle_path"`
	AppBuildSHA    string                    `json:"app_build_sha"`
	DaemonPID      int                       `json:"daemon_pid"`
	DaemonPort     int                       `json:"daemon_port"`
	DaemonURL      string                    `json:"daemon_url"`
	Cleanup        inventory.CleanupEvidence `json:"cleanup"`
}

// Validate fails closed for absent native coverage, a non-native row, a
// missing artifact, or an artifact whose recorded digest no longer matches.
func Validate(compiled inventory.Compiled, root string, manifest Manifest) error {
	canonicalRoot, err := canonicalDirectory(root, "artifact root")
	if err != nil {
		return err
	}
	if err := validateCollection(canonicalRoot, manifest.Collection); err != nil {
		return err
	}
	tempRoot, err := canonicalDirectory(manifest.Collection.TempRoot, "collection temp root")
	if err != nil {
		return err
	}
	if err := inventory.ValidateSuiteCasesForSurface(compiled, manifest.Contract, manifest.Cases, inventory.SurfaceNativeGUI); err != nil {
		return err
	}
	passes := make(map[string]int, len(manifest.Cases))
	declared := make(map[string]struct{}, len(manifest.Cases))
	for _, c := range manifest.Cases {
		declared[manifestCaseKey(c)] = struct{}{}
	}
	for _, record := range manifest.Evidence {
		if record.Surface != inventory.SurfaceNativeGUI {
			return fmt.Errorf("non-native evidence is not accepted by native runner")
		}
		var matched *inventory.Case
		for i := range manifest.Cases {
			if manifest.Cases[i].ItemID == record.ItemID && manifest.Cases[i].ScenarioID == record.ScenarioID && manifest.Cases[i].InvocationID == record.InvocationID {
				matched = &manifest.Cases[i]
				break
			}
		}
		if matched == nil {
			return fmt.Errorf("evidence has no declared case")
		}
		if err := inventory.ValidateSuiteEvidence(compiled, manifest.Contract, *matched, record); err != nil {
			return err
		}
		if err := validateEvidenceCollection(record, manifest.Collection, tempRoot); err != nil {
			return err
		}
		key := manifestCaseKey(*matched)
		if _, ok := declared[key]; !ok {
			return fmt.Errorf("evidence has no declared case")
		}
		if record.Outcome != inventory.Pass {
			return fmt.Errorf("qualifying native manifest requires a final PASS for %q", key)
		}
		passes[key]++
		if passes[key] > 1 {
			return fmt.Errorf("qualifying native manifest has duplicate PASS evidence for %q", key)
		}
		if record.Outcome == inventory.Pass {
			for _, artifact := range record.Artifacts {
				if err := verifyArtifact(canonicalRoot, artifact); err != nil {
					return err
				}
			}
		}
	}
	for key := range declared {
		if passes[key] != 1 {
			return fmt.Errorf("qualifying native manifest is missing final PASS evidence for %q", key)
		}
	}
	// RenderSuiteResultMarkdown is deliberately cross-surface and therefore
	// requires API/TUI cases too. This child validates only the native lane;
	// #1090 is responsible for the assembled report once every surface exists.
	return nil
}

func manifestCaseKey(c inventory.Case) string {
	if c.ScenarioID != "" {
		return "scenario:" + c.ScenarioID
	}
	return c.ItemID + "#" + c.InvocationID
}

func canonicalDirectory(path, label string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s must be absolute", label)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", label, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s must be a directory", label)
	}
	return filepath.Clean(resolved), nil
}

func validateCollection(root string, collection CollectionProvenance) error {
	if collection.Launcher != "scripts/run-native-gui-acceptance.sh" || len(collection.Nonce) < 32 {
		return fmt.Errorf("manifest lacks launcher-owned collection provenance")
	}
	tempRoot, err := canonicalDirectory(collection.TempRoot, "collection temp root")
	if err != nil {
		return err
	}
	artifactRoot, err := canonicalDirectory(collection.ArtifactRoot, "collection artifact root")
	if err != nil {
		return err
	}
	if artifactRoot != root || !contained(tempRoot, root) {
		return fmt.Errorf("collection roots are not launcher-isolated")
	}
	repoRoot, err := canonicalDirectory(collection.RepositoryRoot, "collection repository root")
	if err != nil {
		return err
	}
	driver, err := canonicalRegularFile(collection.DriverPath, "collection driver")
	if err != nil {
		return err
	}
	if !contained(repoRoot, driver) || !validDigestForFile(driver, collection.DriverDigest) {
		return fmt.Errorf("collection driver is not a digest-bound repository artifact")
	}
	appBundle, err := canonicalDirectory(collection.AppBundlePath, "collection app bundle")
	if err != nil || !validGitSHA(collection.AppBuildSHA) || !strings.HasSuffix(appBundle, ".app") {
		return fmt.Errorf("collection lacks exact app build provenance")
	}
	if !collection.Cleanup.Verified || strings.TrimSpace(collection.Cleanup.Detail) == "" {
		return fmt.Errorf("collection cleanup is not verified")
	}
	parsed, err := url.Parse(collection.DaemonURL)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() == "" || parsed.Port() == "" {
		return fmt.Errorf("collection daemon URL is invalid")
	}
	port := 0
	if _, err := fmt.Sscanf(parsed.Port(), "%d", &port); err != nil || port != collection.DaemonPort || port < 1 || port > 65535 {
		return fmt.Errorf("collection daemon URL port does not match child daemon")
	}
	if host := net.ParseIP(parsed.Hostname()); host == nil || !host.IsLoopback() || collection.DaemonPID <= 0 {
		return fmt.Errorf("collection daemon must be launcher-owned loopback child")
	}
	return nil
}

func canonicalRegularFile(path, label string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s must be absolute", label)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", label, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s must be a regular file", label)
	}
	return filepath.Clean(resolved), nil
}

func validateEvidenceCollection(record inventory.Evidence, collection CollectionProvenance, tempRoot string) error {
	env := record.Environment
	bundle, err := canonicalDirectory(env.BundlePath, "evidence app bundle")
	collectionBundle, collectionErr := canonicalDirectory(collection.AppBundlePath, "collection app bundle")
	if err != nil || collectionErr != nil || env.BuildSHA != collection.AppBuildSHA || bundle != collectionBundle || env.DaemonPID != collection.DaemonPID || env.DaemonPort != collection.DaemonPort {
		return fmt.Errorf("evidence environment is not bound to launcher collection")
	}
	workspace, err := canonicalDirectory(env.WorkspacePath, "evidence workspace")
	if err != nil || !env.WorkspaceIsolated || !contained(tempRoot, workspace) {
		return fmt.Errorf("evidence workspace is not collection-isolated")
	}
	if record.Cleanup != collection.Cleanup {
		return fmt.Errorf("evidence cleanup is not bound to launcher collection")
	}
	return nil
}

func contained(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func verifyArtifact(root string, artifact inventory.ArtifactRef) error {
	path := artifact.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := canonicalRegularFile(path, "artifact")
	if err != nil || !contained(root, path) {
		return fmt.Errorf("artifact path escapes root: %q", artifact.Path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read artifact %q: %w", artifact.Path, err)
	}
	sum := sha256.Sum256(data)
	if artifact.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return fmt.Errorf("artifact digest mismatch: %q", artifact.Path)
	}
	return nil
}

func validDigestForFile(path, expected string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return expected == "sha256:"+hex.EncodeToString(sum[:])
}

func validGitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
