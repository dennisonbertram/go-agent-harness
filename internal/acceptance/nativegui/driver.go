package nativegui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PermissionState is deliberately explicit. PlatformPermissionState never
// requests a grant: prompt_required means at least one capability is not
// already granted and a live driver therefore must not start.
type PermissionState string

const (
	PermissionAvailable      PermissionState = "available"
	PermissionUnavailable    PermissionState = "unavailable"
	PermissionPromptRequired PermissionState = "prompt_required"
)

// PermissionReport retains the independently observed macOS capabilities so
// an operator knows which System Settings grant is missing. Source names the
// non-prompting platform probe that supplied the result.
type PermissionReport struct {
	State           PermissionState `json:"state"`
	Accessibility   bool            `json:"accessibility"`
	ScreenRecording bool            `json:"screen_recording"`
	Source          string          `json:"source"`
}

// RenderedDriver owns the admission boundary. Start must be the owner-created
// lifecycle; it is never called unless both permissions are already available.
type RenderedDriver struct {
	Permissions func(context.Context) (PermissionReport, error)
	Start       func(context.Context) error
}

func (d RenderedDriver) Run(ctx context.Context) error {
	if d.Permissions == nil {
		return fmt.Errorf("native rendered driver lacks TCC preflight")
	}
	report, err := d.Permissions(ctx)
	if err != nil {
		return fmt.Errorf("native rendered driver TCC preflight: %w", err)
	}
	switch report.State {
	case PermissionAvailable:
		if !report.Accessibility || !report.ScreenRecording {
			return fmt.Errorf("native rendered driver received inconsistent available TCC report")
		}
	case PermissionPromptRequired:
		return fmt.Errorf("native rendered driver requires pre-granted TCC permissions (accessibility=%t screen_recording=%t); it will not request or accept a prompt", report.Accessibility, report.ScreenRecording)
	case PermissionUnavailable:
		return fmt.Errorf("native rendered driver is unavailable on this platform (%s)", report.Source)
	default:
		return fmt.Errorf("native rendered driver received unknown TCC state %q", report.State)
	}
	if d.Start == nil {
		return fmt.Errorf("native rendered driver lacks owner-created lifecycle")
	}
	return d.Start(ctx)
}

type CoreArtifactKind string

const (
	CoreArtifactScreenshot CoreArtifactKind = "screenshot"
	CoreArtifactAX         CoreArtifactKind = "accessibility"
	CoreArtifactRawSSE     CoreArtifactKind = "raw_sse"
	CoreArtifactAPIStore   CoreArtifactKind = "api_store"
	CoreArtifactDaemonLog  CoreArtifactKind = "daemon_log"
	CoreArtifactAppLog     CoreArtifactKind = "app_log"
)

// CoreArtifact is one owner-private signal. Path is absolute while a proof is
// assembled in memory and is serialized as a path relative to ArtifactRoot.
type CoreArtifact struct {
	Kind   CoreArtifactKind `json:"kind"`
	Path   string           `json:"path"`
	Digest string           `json:"sha256"`
	Bytes  int64            `json:"bytes"`
}

type CoreCleanup struct {
	Verified bool   `json:"verified"`
	Detail   string `json:"detail"`
}

// CoreProof is the minimum evidence contract for this slice. It proves one
// conversation with two distinct completed runs; it is not the #1089 matrix.
type CoreProof struct {
	SchemaVersion  string         `json:"schema_version"`
	Nonce          string         `json:"nonce"`
	ArtifactRoot   string         `json:"artifact_root"`
	ConversationID string         `json:"conversation_id"`
	RunIDs         []string       `json:"run_ids"`
	FirstPrompt    string         `json:"first_prompt"`
	SecondPrompt   string         `json:"second_prompt"`
	FirstReply     string         `json:"first_reply"`
	SecondReply    string         `json:"second_reply"`
	DaemonPID      int            `json:"daemon_pid"`
	AppPID         int            `json:"app_pid"`
	Artifacts      []CoreArtifact `json:"artifacts"`
	Cleanup        CoreCleanup    `json:"cleanup"`
}

func (p *CoreProof) SealArtifacts() error {
	if p == nil {
		return fmt.Errorf("core rendered proof is required")
	}
	for i := range p.Artifacts {
		path, err := canonicalCoreArtifact(p.ArtifactRoot, p.Artifacts[i].Path)
		if err != nil {
			return fmt.Errorf("core rendered artifact %d is not an owned regular file", i)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return fmt.Errorf("core rendered artifact %q is empty", p.Artifacts[i].Kind)
		}
		sum := sha256.Sum256(data)
		p.Artifacts[i].Digest = hex.EncodeToString(sum[:])
		p.Artifacts[i].Bytes = int64(len(data))
		rel, err := filepath.Rel(p.ArtifactRoot, path)
		if err != nil || !safeRelativePath(rel) {
			return fmt.Errorf("core rendered artifact %d path is not safely relative", i)
		}
		p.Artifacts[i].Path = filepath.ToSlash(rel)
	}
	return nil
}

func ValidateCoreProof(p CoreProof) error {
	if p.SchemaVersion != "native-core-rendered-v1" || len(strings.TrimSpace(p.Nonce)) < 32 {
		return fmt.Errorf("core rendered proof lacks schema or launcher nonce")
	}
	root, err := canonicalDirectory(p.ArtifactRoot, "core rendered artifact root")
	if err != nil {
		return err
	}
	if p.DaemonPID <= 0 || p.AppPID <= 0 || p.DaemonPID == p.AppPID {
		return fmt.Errorf("core rendered proof lacks distinct owned child identities")
	}
	if strings.TrimSpace(p.ConversationID) == "" || len(p.RunIDs) != 2 || p.RunIDs[0] == "" || p.RunIDs[1] == "" || p.RunIDs[0] == p.RunIDs[1] {
		return fmt.Errorf("core rendered proof requires one conversation and two distinct runs")
	}
	for label, value := range map[string]string{"first prompt": p.FirstPrompt, "second prompt": p.SecondPrompt, "first reply": p.FirstReply, "second reply": p.SecondReply} {
		if strings.TrimSpace(value) == "" || !strings.Contains(value, p.Nonce) {
			return fmt.Errorf("core rendered proof %s lacks nonce correlation", label)
		}
	}
	if !p.Cleanup.Verified || strings.TrimSpace(p.Cleanup.Detail) == "" {
		return fmt.Errorf("core rendered proof lacks verified bounded cleanup")
	}
	required := map[CoreArtifactKind]bool{
		CoreArtifactScreenshot: false, CoreArtifactAX: false,
		CoreArtifactRawSSE: false, CoreArtifactAPIStore: false,
		CoreArtifactDaemonLog: false, CoreArtifactAppLog: false,
	}
	seenPaths := map[string]struct{}{}
	contents := map[CoreArtifactKind][]byte{}
	for _, artifact := range p.Artifacts {
		if _, ok := required[artifact.Kind]; !ok {
			return fmt.Errorf("core rendered proof has unsupported artifact kind %q", artifact.Kind)
		}
		if required[artifact.Kind] {
			return fmt.Errorf("core rendered proof duplicates artifact kind %q", artifact.Kind)
		}
		required[artifact.Kind] = true
		if !safeRelativePath(artifact.Path) {
			return fmt.Errorf("core rendered proof artifact path %q is unsafe", artifact.Path)
		}
		path, err := canonicalCoreArtifact(root, filepath.Join(root, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return fmt.Errorf("core rendered proof artifact %q is not owner-contained", artifact.Kind)
		}
		if _, exists := seenPaths[path]; exists {
			return fmt.Errorf("core rendered proof artifacts must be distinct")
		}
		seenPaths[path] = struct{}{}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if int64(len(data)) != artifact.Bytes || len(data) == 0 {
			return fmt.Errorf("core rendered proof artifact %q has wrong byte length", artifact.Kind)
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(artifact.Digest, hex.EncodeToString(sum[:])) {
			return fmt.Errorf("core rendered proof artifact %q hash mismatch", artifact.Kind)
		}
		contents[artifact.Kind] = data
	}
	for kind, present := range required {
		if !present {
			return fmt.Errorf("core rendered proof lacks %q artifact", kind)
		}
	}
	if !bytes.HasPrefix(contents[CoreArtifactScreenshot], []byte("\x89PNG\r\n\x1a\n")) {
		return fmt.Errorf("core rendered screenshot is not PNG evidence")
	}
	semanticMarkers := []string{p.Nonce, p.ConversationID, p.RunIDs[0], p.RunIDs[1], p.FirstPrompt, p.SecondPrompt, p.FirstReply, p.SecondReply}
	for _, kind := range []CoreArtifactKind{CoreArtifactAX, CoreArtifactRawSSE, CoreArtifactAPIStore} {
		for _, marker := range semanticMarkers {
			if !bytes.Contains(contents[kind], []byte(marker)) {
				return fmt.Errorf("core rendered artifact %q lacks correlation marker %q", kind, marker)
			}
		}
	}
	return nil
}

func canonicalCoreArtifact(root, path string) (string, error) {
	entry, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
		return "", fmt.Errorf("core rendered artifact must be a regular non-symlink file")
	}
	canonical, err := canonicalRegularFile(path, "core rendered artifact")
	if err != nil || !contained(root, canonical) {
		return "", fmt.Errorf("core rendered artifact escapes owner root")
	}
	return canonical, nil
}

func WriteCoreProof(path string, proof CoreProof) error {
	if err := ValidateCoreProof(proof); err != nil {
		return err
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal core rendered proof: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write core rendered proof: %w", err)
	}
	return nil
}
