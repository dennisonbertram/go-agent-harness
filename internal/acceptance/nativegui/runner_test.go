package nativegui

import (
	"crypto/sha256"
	"encoding/hex"
	"go-agent-harness/internal/acceptance/inventory"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateRejectsTamperedRenderedArtifact(t *testing.T) {
	compiled, manifest := validManifest(t)
	root := manifest.Evidence[0].Environment.WorkspacePath
	if err := Validate(compiled, root, manifest); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, root, "screen.png", "tampered")
	if err := Validate(compiled, root, manifest); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateRequiresOnePassingRecordForEveryDeclaredCase(t *testing.T) {
	compiled, manifest := validManifest(t)
	for _, tc := range []struct {
		name   string
		mutate func(*Manifest)
	}{
		{
			name:   "empty evidence",
			mutate: func(m *Manifest) { m.Evidence = nil },
		},
		{
			name:   "partial evidence",
			mutate: func(m *Manifest) { m.Evidence = m.Evidence[:1] },
		},
		{
			name: "all failures",
			mutate: func(m *Manifest) {
				for i := range m.Evidence {
					m.Evidence[i].Outcome = inventory.Fail
					m.Evidence[i].FailureClass = "driver_failure"
				}
			},
		},
		{
			name:   "duplicate passing evidence",
			mutate: func(m *Manifest) { m.Evidence = append(m.Evidence, m.Evidence[0]) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := manifest
			copy.Evidence = append([]inventory.Evidence(nil), manifest.Evidence...)
			tc.mutate(&copy)
			if err := Validate(compiled, manifest.Evidence[0].Environment.WorkspacePath, copy); err == nil {
				t.Fatal("expected manifest completeness failure")
			}
		})
	}
}

func TestValidateRejectsArtifactSymlinkEscape(t *testing.T) {
	compiled, manifest := validManifest(t)
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("outside pixels"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(manifest.Evidence[0].Environment.WorkspacePath, "escape.png")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for i := range manifest.Evidence {
		for j := range manifest.Evidence[i].Artifacts {
			manifest.Evidence[i].Artifacts[j].Path = "escape.png"
			manifest.Evidence[i].Artifacts[j].Digest = digest("outside pixels")
		}
	}
	if err := Validate(compiled, manifest.Evidence[0].Environment.WorkspacePath, manifest); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

func TestValidateRejectsUnattestedDriverOrNonLoopbackDaemonURL(t *testing.T) {
	compiled, manifest := validManifest(t)
	outside := filepath.Join(t.TempDir(), "arbitrary-driver")
	if err := os.WriteFile(outside, []byte("#!/bin/zsh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Manifest)
	}{
		{
			name: "driver outside repository",
			mutate: func(m *Manifest) {
				m.Collection.DriverPath = outside
				m.Collection.DriverDigest = digest("#!/bin/zsh\n")
			},
		},
		{
			name:   "non loopback URL",
			mutate: func(m *Manifest) { m.Collection.DaemonURL = "http://192.0.2.10:1" },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := manifest
			tc.mutate(&copy)
			if err := Validate(compiled, manifest.Evidence[0].Environment.WorkspacePath, copy); err == nil {
				t.Fatal("expected unattested collection rejection")
			}
		})
	}
}

func TestValidateRejectsFinalComponentAppBundleSymlinks(t *testing.T) {
	compiled, manifest := validManifest(t)
	realBundle := manifest.Collection.AppBundlePath
	link := filepath.Join(t.TempDir(), "GoCode.app")
	if err := os.Symlink(realBundle, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Manifest)
	}{
		{
			name:   "collection bundle",
			mutate: func(m *Manifest) { m.Collection.AppBundlePath = link },
		},
		{
			name: "evidence bundle",
			mutate: func(m *Manifest) {
				for i := range m.Evidence {
					m.Evidence[i].Environment.BundlePath = link
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := manifest
			copy.Evidence = append([]inventory.Evidence(nil), manifest.Evidence...)
			tc.mutate(&copy)
			if err := Validate(compiled, manifest.Evidence[0].Environment.WorkspacePath, copy); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("expected final-component symlink rejection, got %v", err)
			}
		})
	}
}

func validManifest(t *testing.T) (inventory.Compiled, Manifest) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, root, "screen.png", "actual rendered pixels")
	repoRoot := t.TempDir()
	appBundle := filepath.Join(t.TempDir(), "GoCode.app")
	if err := os.Mkdir(appBundle, 0700); err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(repoRoot, "scripts", "native-gui-driver.sh")
	if err := os.MkdirAll(filepath.Dir(driver), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(driver, []byte("#!/bin/zsh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	items := []inventory.Item{
		{ID: "tool:read", Kind: inventory.ToolKind, Name: "read", Owner: "test", Condition: "test", Availability: inventory.Available, Surfaces: []inventory.Surface{inventory.SurfaceNativeGUI}},
		{ID: "tool:write", Kind: inventory.ToolKind, Name: "write", Owner: "test", Condition: "test", Availability: inventory.Available, Surfaces: []inventory.Surface{inventory.SurfaceNativeGUI}},
	}
	compiled := inventory.Compiled{SchemaVersion: inventory.SchemaVersion, Hash: strings.Repeat("b", 64), Items: items}
	mappings := make([]inventory.SurfaceApplicability, 0, len(items))
	for _, item := range items {
		mappings = append(mappings, inventory.SurfaceApplicability{ItemID: item.ID, Surface: inventory.SurfaceNativeGUI, Availability: inventory.Available, SourceRefs: []string{"macapp/Sources/GoCodeUI/ChatView.swift"}, UXRationale: "tool transcript is rendered in native chat"})
	}
	contract, err := inventory.CompileSuiteContract(compiled, nil, mappings)
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	post := []inventory.Postcondition{{Kind: inventory.PostconditionRenderedScreen, Probe: "ocr", AssertionID: "visible", Description: "ordered message visible"}}
	actions := []inventory.Action{{Kind: "submit", Value: "type ordered prompt"}}
	cases := make([]inventory.Case, 0, len(items))
	evidence := make([]inventory.Evidence, 0, len(items))
	for _, item := range items {
		cases = append(cases, inventory.Case{ItemID: item.ID, Surfaces: []inventory.Surface{inventory.SurfaceNativeGUI}, EvidenceClass: inventory.EvidenceClassConversation, OrderedActions: actions, ExpectedPostconditions: post, Cleanup: "isolated app stopped"})
		evidence = append(evidence, inventory.Evidence{SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash, SuiteHash: contract.Hash, ItemID: item.ID, Surface: inventory.SurfaceNativeGUI, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Pass, OrderedActions: actions, RunID: "run-" + item.Name, ConversationID: "conversation-" + item.Name, EventIDs: []string{"run:1"}, ExpectedPostconditions: post, ObservedPostconditions: []inventory.ProbeObservation{{Kind: inventory.PostconditionRenderedScreen, Probe: "ocr", AssertionID: "visible", Value: "message", Verified: true}}, Artifacts: []inventory.ArtifactRef{{Kind: inventory.ArtifactScreenshot, Path: "screen.png", Digest: digest("actual rendered pixels"), Redacted: &yes}, {Kind: inventory.ArtifactAXSnapshot, Path: "screen.png", Digest: digest("actual rendered pixels"), Redacted: &yes}, {Kind: inventory.ArtifactRawSSEEvent, Path: "screen.png", Digest: digest("actual rendered pixels"), Redacted: &yes}, {Kind: inventory.ArtifactAPIStoreProbe, Path: "screen.png", Digest: digest("actual rendered pixels"), Redacted: &yes}}, Environment: inventory.EnvironmentMetadata{BuildSHA: strings.Repeat("a", 40), BundlePath: appBundle, DaemonPID: 1, DaemonPort: 1, WorkspacePath: root, WorkspaceIsolated: true}, Cleanup: inventory.CleanupEvidence{Verified: true, Detail: "stopped"}, Timing: inventory.Timing{StartedAt: time.Now().Add(-time.Second), FinishedAt: time.Now()}})
	}
	collection := CollectionProvenance{Launcher: "scripts/run-native-gui-acceptance.sh", Nonce: strings.Repeat("a", 32), TempRoot: root, ArtifactRoot: root, RepositoryRoot: repoRoot, DriverPath: driver, DriverDigest: digest("#!/bin/zsh\n"), AppBundlePath: appBundle, AppBuildSHA: strings.Repeat("a", 40), DaemonPID: 1, DaemonPort: 1, DaemonURL: "http://127.0.0.1:1", Cleanup: inventory.CleanupEvidence{Verified: true, Detail: "stopped"}}
	return compiled, Manifest{Contract: contract, Cases: cases, Evidence: evidence, Collection: collection}
}
func mustWrite(t *testing.T, r, n, v string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r, n), []byte(v), 0600); err != nil {
		t.Fatal(err)
	}
}
func digest(v string) string {
	s := sha256.Sum256([]byte(v))
	return "sha256:" + hex.EncodeToString(s[:])
}
