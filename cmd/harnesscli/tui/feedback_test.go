package tui_test

import (
	"archive/zip"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	harnessconfig "go-agent-harness/cmd/harnesscli/config"
	tui "go-agent-harness/cmd/harnesscli/tui"
)

// findFeedbackZip returns the single zip under the feedback dir of the given
// HOME, failing the test otherwise.
func findFeedbackZip(t *testing.T, home string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(home, ".config", "harnesscli", "feedback", "*.zip"))
	if err != nil {
		t.Fatalf("glob feedback zips: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one feedback zip under %s, got %v", home, matches)
	}
	return matches[0]
}

func writeFeedbackScreenshot(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create screenshot: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(1, 1, color.RGBA{G: 0xff, A: 0xff})
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode screenshot: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close screenshot: %v", err)
	}
}

func readZipText(t *testing.T, path, name string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open member: %v", err)
			}
			defer rc.Close()
			buf := new(strings.Builder)
			if _, err := io.Copy(buf, rc); err != nil {
				t.Fatalf("read member: %v", err)
			}
			return buf.String()
		}
	}
	t.Fatalf("member %s not found", name)
	return ""
}

// TestFeedbackCommand_WritesBundleAndReportsPath is the acceptance-level test:
// /feedback writes a zip under <config-dir>/feedback/, prints its path, and
// the bundled config contains no secrets.
func TestFeedbackCommand_WritesBundleAndReportsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Seed a CLI config holding a canary API key.
	if err := harnessconfig.Save(&harnessconfig.Config{
		APIKeys: map[string]string{"openai": "sk-feedbackcanary1234567890"},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	// Point HARNESS_ROLLOUT_DIR at a temp rollout dir with one dated file.
	rolloutDir := t.TempDir()
	t.Setenv("HARNESS_ROLLOUT_DIR", rolloutDir)
	dateDir := filepath.Join(rolloutDir, "2026-07-19")
	if err := os.MkdirAll(dateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, "run-1.jsonl"), []byte(`{"event":"run.started"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/feedback")

	got := m.StatusMsg()
	if !strings.Contains(got, ".zip") || !strings.Contains(got, filepath.Join(".config", "harnesscli", "feedback")) {
		t.Fatalf("StatusMsg() = %q, want the feedback zip path under the config dir", got)
	}

	bundle := findFeedbackZip(t, home)
	configJSON := readZipText(t, bundle, "config.json")
	if strings.Contains(configJSON, "sk-feedbackcanary1234567890") {
		t.Fatalf("canary API key survived into the bundled config:\n%s", configJSON)
	}
	rollout := readZipText(t, bundle, "rollouts/2026-07-19/run-1.jsonl")
	if !strings.Contains(rollout, "run.started") {
		t.Errorf("bundled rollout missing its content: %q", rollout)
	}
}

func TestFeedbackCommand_WorksDuringActiveRunAndBundlesRequestContextAndScreenshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNESS_ROLLOUT_DIR", "")

	screenshot := filepath.Join(t.TempDir(), "active run.png")
	writeFeedbackScreenshot(t, screenshot)

	m := initModel(t, 100, 30)
	updated, _ := m.Update(tui.RunStartedMsg{RunID: "run-active-123"})
	m = updated.(tui.Model)
	updated, _ = m.Update(tui.TranscriptEntryMsg{
		Role:    "user",
		Content: "The export button stopped responding",
	})
	m = updated.(tui.Model)

	m = sendSlashCommand(m, `/feedback --screenshot "`+screenshot+`" Fix export without losing the current run`)

	if !m.RunActive() {
		t.Fatal("/feedback must not interrupt an active run")
	}
	bundle := findFeedbackZip(t, home)
	request := readZipText(t, bundle, "request.md")
	if !strings.Contains(request, "Fix export without losing the current run") {
		t.Errorf("request.md lost the user's request: %q", request)
	}
	contextJSON := readZipText(t, bundle, "context.json")
	if !strings.Contains(contextJSON, "run-active-123") || !strings.Contains(contextJSON, `"run_active": true`) {
		t.Errorf("context.json lost active run context:\n%s", contextJSON)
	}
	transcriptJSON := readZipText(t, bundle, "transcript.json")
	if !strings.Contains(transcriptJSON, "export button stopped responding") {
		t.Errorf("transcript.json lost conversation context:\n%s", transcriptJSON)
	}
	if screenshotMember := readZipText(t, bundle, "attachments/screenshot.png"); screenshotMember == "" {
		t.Error("bundle screenshot is empty")
	}
}

// TestFeedbackCommand_WorksWithoutRolloutDir verifies /feedback still writes a
// bundle when HARNESS_ROLLOUT_DIR is unset.
func TestFeedbackCommand_WorksWithoutRolloutDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNESS_ROLLOUT_DIR", "")

	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/feedback")

	if got := m.StatusMsg(); !strings.Contains(got, ".zip") {
		t.Fatalf("StatusMsg() = %q, want the bundle path even without a rollout dir", got)
	}
	bundle := findFeedbackZip(t, home)
	marker := readZipText(t, bundle, "rollouts/NOT_PRESENT.txt")
	if !strings.Contains(marker, "rollout") {
		t.Errorf("bundle must note the missing rollouts, marker: %q", marker)
	}
}

func TestFeedbackCommand_ConsecutiveCapturesDoNotOverwrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNESS_ROLLOUT_DIR", "")

	m := initModel(t, 80, 24)
	m = sendSlashCommand(m, "/feedback first report")
	m = sendSlashCommand(m, "/feedback second report")

	matches, err := filepath.Glob(filepath.Join(home, ".config", "harnesscli", "feedback", "*.zip"))
	if err != nil {
		t.Fatalf("glob feedback zips: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("consecutive /feedback captures must produce two bundles, got %v", matches)
	}
	first := readZipText(t, matches[0], "request.md")
	second := readZipText(t, matches[1], "request.md")
	if first == second {
		t.Fatalf("consecutive bundles were overwritten or duplicated: %q", first)
	}
}

// TestFeedbackCommand_Registered verifies the feedback command is registered.
func TestFeedbackCommand_Registered(t *testing.T) {
	r := tui.NewCommandRegistry()
	if !r.IsRegistered("feedback") {
		t.Fatal("built-in registry must register the feedback command")
	}
	entry, ok := r.Lookup("feedback")
	if !ok || entry.Description == "" {
		t.Fatal("feedback command must have a description for /help and autocomplete")
	}
}

// TestFeedbackCommand_InSlashComplete verifies /feedback appears in autocomplete.
func TestFeedbackCommand_InSlashComplete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := initModel(t, 120, 40)
	m = typeIntoModel(m, "/fee")
	if v := m.View(); !strings.Contains(v, "feedback") {
		t.Errorf("slash-complete must contain 'feedback' when typing '/fee'; got:\n%s", v)
	}
}
