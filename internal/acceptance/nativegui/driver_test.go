package nativegui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderedDriverFailsClosedBeforeLifecycleWithoutBothTCCGrants(t *testing.T) {
	for _, report := range []PermissionReport{
		{State: PermissionPromptRequired, Accessibility: false, ScreenRecording: true, Source: "test"},
		{State: PermissionPromptRequired, Accessibility: true, ScreenRecording: false, Source: "test"},
		{State: PermissionUnavailable, Source: "test"},
	} {
		started := false
		d := RenderedDriver{
			Permissions: func(context.Context) (PermissionReport, error) { return report, nil },
			Start:       func(context.Context) error { started = true; return nil },
		}
		if err := d.Run(context.Background()); err == nil {
			t.Fatalf("expected TCC failure for %#v", report)
		}
		if started {
			t.Fatalf("lifecycle started after TCC preflight failure %#v", report)
		}
	}
}

func TestRenderedDriverStartsOnlyForConsistentAvailableReport(t *testing.T) {
	started := false
	d := RenderedDriver{
		Permissions: func(context.Context) (PermissionReport, error) {
			return PermissionReport{State: PermissionAvailable, Accessibility: true, ScreenRecording: true, Source: "test"}, nil
		},
		Start: func(context.Context) error { started = true; return nil },
	}
	if err := d.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("available TCC report did not start owned lifecycle")
	}
	d.Permissions = func(context.Context) (PermissionReport, error) {
		return PermissionReport{State: PermissionAvailable, Accessibility: true, ScreenRecording: false, Source: "test"}, nil
	}
	if err := d.Run(context.Background()); err == nil {
		t.Fatal("inconsistent available report must fail closed")
	}
}

func TestPlatformPermissionStateIgnoresEnvironmentAttestation(t *testing.T) {
	t.Setenv("HARNESS_NATIVE_TCC_STATE", "available")
	report, err := PlatformPermissionState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Source == "" || (report.State != PermissionAvailable && report.State != PermissionPromptRequired && report.State != PermissionUnavailable) {
		t.Fatalf("unexpected platform permission report %#v", report)
	}
}

func TestCoreProofRequiresTypedHashedCorrelatedSignals(t *testing.T) {
	proof := validCoreProof(t)
	if err := proof.SealArtifacts(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCoreProof(proof); err != nil {
		t.Fatal(err)
	}
	if err := WriteCoreProof(filepath.Join(proof.ArtifactRoot, "proof.json"), proof); err != nil {
		t.Fatal(err)
	}
}

func TestCoreProofRejectsUnsafeOrUncorrelatedEvidence(t *testing.T) {
	t.Run("duplicate kind", func(t *testing.T) {
		proof := sealedCoreProof(t)
		proof.Artifacts[1].Kind = proof.Artifacts[0].Kind
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected duplicate kind rejection")
		}
	})
	t.Run("wrong hash", func(t *testing.T) {
		proof := sealedCoreProof(t)
		proof.Artifacts[0].Digest = strings.Repeat("0", 64)
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected hash rejection")
		}
	})
	t.Run("duplicate path", func(t *testing.T) {
		proof := sealedCoreProof(t)
		proof.Artifacts[1].Path = proof.Artifacts[0].Path
		proof.Artifacts[1].Digest = proof.Artifacts[0].Digest
		proof.Artifacts[1].Bytes = proof.Artifacts[0].Bytes
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected duplicate path rejection")
		}
	})
	t.Run("escaped path", func(t *testing.T) {
		proof := sealedCoreProof(t)
		proof.Artifacts[0].Path = "../screen.png"
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected escaped path rejection")
		}
	})
	t.Run("missing kind", func(t *testing.T) {
		proof := sealedCoreProof(t)
		proof.Artifacts = proof.Artifacts[:len(proof.Artifacts)-1]
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected missing kind rejection")
		}
	})
	t.Run("empty artifact", func(t *testing.T) {
		proof := validCoreProof(t)
		if err := os.WriteFile(proof.Artifacts[1].Path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := proof.SealArtifacts(); err == nil {
			t.Fatal("expected empty artifact rejection")
		}
	})
	t.Run("invalid screenshot", func(t *testing.T) {
		proof := validCoreProof(t)
		if err := os.WriteFile(proof.Artifacts[0].Path, []byte("not a PNG"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := proof.SealArtifacts(); err != nil {
			t.Fatal(err)
		}
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected PNG signature rejection")
		}
	})
	t.Run("missing semantic marker", func(t *testing.T) {
		proof := validCoreProof(t)
		for _, artifact := range proof.Artifacts {
			if artifact.Kind == CoreArtifactAX {
				if err := os.WriteFile(artifact.Path, []byte(`{"tree":"rendered but unrelated"}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := proof.SealArtifacts(); err != nil {
			t.Fatal(err)
		}
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected AX correlation rejection")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		proof := validCoreProof(t)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
			t.Fatal(err)
		}
		proof.Artifacts[0].Path = filepath.Join(proof.ArtifactRoot, "screen-link.png")
		if err := os.Symlink(outside, proof.Artifacts[0].Path); err != nil {
			t.Fatal(err)
		}
		if err := proof.SealArtifacts(); err == nil {
			t.Fatal("expected symlink rejection")
		}
	})
	t.Run("contained symlink", func(t *testing.T) {
		proof := validCoreProof(t)
		target := proof.Artifacts[0].Path
		link := filepath.Join(proof.ArtifactRoot, "screen-link.png")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		proof.Artifacts[0].Path = link
		if err := proof.SealArtifacts(); err == nil {
			t.Fatal("expected contained symlink rejection")
		}
	})
	t.Run("cleanup", func(t *testing.T) {
		proof := sealedCoreProof(t)
		proof.Cleanup.Verified = false
		if err := ValidateCoreProof(proof); err == nil {
			t.Fatal("expected cleanup rejection")
		}
	})
}

func sealedCoreProof(t *testing.T) CoreProof {
	t.Helper()
	proof := validCoreProof(t)
	if err := proof.SealArtifacts(); err != nil {
		t.Fatal(err)
	}
	return proof
}

func validCoreProof(t *testing.T) CoreProof {
	t.Helper()
	root := t.TempDir()
	nonce := strings.Repeat("n", 32)
	proof := CoreProof{
		SchemaVersion: "native-core-rendered-v1", Nonce: nonce,
		ArtifactRoot: root, ConversationID: "conversation-1",
		RunIDs:      []string{"run-1", "run-2"},
		FirstPrompt: "first " + nonce, SecondPrompt: "second " + nonce,
		FirstReply: "first reply " + nonce, SecondReply: "second reply " + nonce,
		DaemonPID: 101, AppPID: 102,
		Cleanup: CoreCleanup{Verified: true, Detail: "stopped owner-created app and daemon"},
	}
	markers := strings.Join([]string{proof.Nonce, proof.ConversationID, proof.RunIDs[0], proof.RunIDs[1], proof.FirstPrompt, proof.SecondPrompt, proof.FirstReply, proof.SecondReply}, "\n")
	fixtures := []struct {
		kind CoreArtifactKind
		name string
		data []byte
	}{
		{CoreArtifactScreenshot, "screen.png", append([]byte("\x89PNG\r\n\x1a\n"), []byte("pixels")...)},
		{CoreArtifactAX, "accessibility.json", []byte(markers)},
		{CoreArtifactRawSSE, "events.sse", []byte(markers)},
		{CoreArtifactAPIStore, "api-store.json", []byte(markers)},
		{CoreArtifactDaemonLog, "daemon.log", []byte("owned daemon log")},
		{CoreArtifactAppLog, "app.log", []byte("owned app log")},
	}
	for _, fixture := range fixtures {
		path := filepath.Join(root, fixture.name)
		if err := os.WriteFile(path, fixture.data, 0600); err != nil {
			t.Fatal(err)
		}
		proof.Artifacts = append(proof.Artifacts, CoreArtifact{Kind: fixture.kind, Path: path})
	}
	return proof
}
