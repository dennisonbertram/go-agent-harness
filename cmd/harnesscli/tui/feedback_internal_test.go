package tui

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	harnessconfig "go-agent-harness/cmd/harnesscli/config"
	"go-agent-harness/cmd/harnesscli/tui/components/inputarea"
	"go-agent-harness/cmd/harnesscli/tui/components/transcriptexport"
)

// ─── zip test helpers ─────────────────────────────────────────────────────────

func zipMemberNames(t *testing.T, path string) []string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer r.Close()
	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

func readZipMember(t *testing.T, path, name string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip %s: %v", path, err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open member %s: %v", name, err)
			}
			defer rc.Close()
			buf := new(strings.Builder)
			if _, err := io.Copy(buf, rc); err != nil {
				t.Fatalf("read member %s: %v", name, err)
			}
			return buf.String()
		}
	}
	t.Fatalf("member %s not found in %s", name, path)
	return ""
}

func writeRolloutFile(t *testing.T, dir, date, name, content string, mod time.Time) string {
	t.Helper()
	dateDir := filepath.Join(dir, date)
	if err := os.MkdirAll(dateDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dateDir, err)
	}
	p := filepath.Join(dateDir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatalf("chtimes %s: %v", p, err)
	}
	return p
}

func writeFeedbackPNG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create screenshot: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode screenshot: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close screenshot: %v", err)
	}
	return path
}

func TestExecuteFeedbackCommand_AttachedImageIsBundledAndPublishesByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNESS_ROLLOUT_DIR", "")

	imagePath := writeFeedbackPNG(t, t.TempDir(), "message-upload.png")
	m := New(DefaultTUIConfig())
	m.input = m.input.AddAttachment(inputarea.Attachment{
		Path:      imagePath,
		MediaType: "image/png",
	})

	cmds, quit := executeFeedbackCommand(&m, Command{
		Name: "feedback",
		Raw:  "/feedback Fix the broken export",
		Args: []string{"Fix", "the", "broken", "export"},
	})
	if quit {
		t.Fatal("/feedback must not quit the TUI")
	}
	if len(cmds) != 2 {
		t.Fatalf("publish-by-default /feedback commands = %d, want status + GitHub publisher", len(cmds))
	}

	matches, err := filepath.Glob(filepath.Join(home, ".config", "harnesscli", "feedback", "*.zip"))
	if err != nil {
		t.Fatalf("glob feedback bundles: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("feedback bundles = %v, want one", matches)
	}
	names := strings.Join(zipMemberNames(t, matches[0]), "\n")
	if !strings.Contains(names, "attachments/screenshot.png") {
		t.Fatalf("attached message image was not bundled:\n%s", names)
	}
}

func TestExecuteFeedbackCommand_LocalConsumesCapturedImagesButPreservesOtherChips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HARNESS_ROLLOUT_DIR", "")

	imagePath := writeFeedbackPNG(t, t.TempDir(), "message-upload.png")
	m := New(DefaultTUIConfig())
	m.input = m.input.
		AddAttachment(inputarea.Attachment{Path: imagePath, MediaType: "image/png"}).
		AddAttachment(inputarea.Attachment{Path: "/tmp/newer-context.txt", MediaType: "text/plain"})

	_, _ = executeFeedbackCommand(&m, Command{
		Name: "feedback",
		Raw:  "/feedback --local Keep this report local",
	})

	remaining := m.input.Attachments()
	if len(remaining) != 1 || remaining[0].Path != "/tmp/newer-context.txt" {
		t.Fatalf("remaining local feedback attachments = %+v, want only uncaptured file chip", remaining)
	}
}

func TestParseFeedbackOptions_PublishesByDefaultAndLocalOptsOut(t *testing.T) {
	t.Parallel()

	publish, err := parseFeedbackOptions("/feedback Fix export")
	if err != nil {
		t.Fatalf("parse default feedback: %v", err)
	}
	if !publish.OpenIssue || publish.Request != "Fix export" {
		t.Fatalf("default feedback options = %+v, want publish with request", publish)
	}

	local, err := parseFeedbackOptions("/feedback --local Keep this on disk")
	if err != nil {
		t.Fatalf("parse local feedback: %v", err)
	}
	if local.OpenIssue || local.Request != "Keep this on disk" {
		t.Fatalf("--local feedback options = %+v, want local-only with request", local)
	}

	compat, err := parseFeedbackOptions("/feedback --issue --screenshot image.png Legacy syntax")
	if err != nil {
		t.Fatalf("parse compatibility feedback: %v", err)
	}
	if !compat.OpenIssue || compat.ScreenshotPath != "image.png" {
		t.Fatalf("compatibility feedback options = %+v", compat)
	}
}

func TestBuildFeedbackBundle_MultipleMessageImagesPreserveOrder(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	first := writeFeedbackPNG(t, tmp, "first.png")
	second := writeFeedbackPNG(t, tmp, "second.png")
	out := filepath.Join(tmp, "bundle.zip")
	if err := buildFeedbackBundle(out, feedbackInput{
		ScreenshotPaths: []string{first, second},
	}); err != nil {
		t.Fatalf("build bundle with multiple images: %v", err)
	}

	names := strings.Join(zipMemberNames(t, out), "\n")
	for _, want := range []string{
		"attachments/screenshot-1.png",
		"attachments/screenshot-1.json",
		"attachments/screenshot-2.png",
		"attachments/screenshot-2.json",
	} {
		if !strings.Contains(names, want) {
			t.Errorf("bundle missing ordered image member %s:\n%s", want, names)
		}
	}
	if firstMeta := readZipMember(t, out, "attachments/screenshot-1.json"); !strings.Contains(firstMeta, "first.png") {
		t.Errorf("first image provenance lost: %s", firstMeta)
	}
	if secondMeta := readZipMember(t, out, "attachments/screenshot-2.json"); !strings.Contains(secondMeta, "second.png") {
		t.Errorf("second image provenance lost: %s", secondMeta)
	}
}

// ─── Command options ──────────────────────────────────────────────────────────

func TestParseFeedbackOptions_RequestScreenshotAndIssue(t *testing.T) {
	t.Parallel()

	got, err := parseFeedbackOptions(`/feedback --issue --screenshot "/tmp/a screenshot.png" Fix exporting after a run`)
	if err != nil {
		t.Fatalf("parseFeedbackOptions: %v", err)
	}
	if !got.OpenIssue {
		t.Error("--issue must request an explicit GitHub issue draft")
	}
	if got.ScreenshotPath != "/tmp/a screenshot.png" {
		t.Errorf("ScreenshotPath = %q", got.ScreenshotPath)
	}
	if got.Request != "Fix exporting after a run" {
		t.Errorf("Request = %q", got.Request)
	}
}

func TestParseFeedbackOptions_DelimiterPreservesRequestFlags(t *testing.T) {
	t.Parallel()

	got, err := parseFeedbackOptions(`/feedback --local -- --issue is text in my request`)
	if err != nil {
		t.Fatalf("parseFeedbackOptions: %v", err)
	}
	if got.OpenIssue {
		t.Error("--issue after -- must be request text, not an option")
	}
	if got.Request != "--issue is text in my request" {
		t.Errorf("Request = %q", got.Request)
	}
}

// ─── Bundle members ───────────────────────────────────────────────────────────

// TestBuildFeedbackBundle_ContainsExpectedMembers verifies the bundle holds
// version.json, config.json, and the newest rollout files under rollouts/,
// capped at five.
func TestBuildFeedbackBundle_ContainsExpectedMembers(t *testing.T) {
	t.Parallel()

	rolloutDir := t.TempDir()
	base := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	// Seven files with strictly increasing modtimes; the two oldest (run-1,
	// run-2) must be excluded by the newest-5 cap.
	for i := 1; i <= 7; i++ {
		date := "2026-07-01"
		if i > 4 {
			date = "2026-07-02"
		}
		writeRolloutFile(t, rolloutDir, date, "run-"+string(rune('0'+i))+".jsonl", `{"event":"run.started"}`+"\n", base.Add(time.Duration(i)*time.Hour))
	}

	out := filepath.Join(t.TempDir(), "bundle.zip")
	err := buildFeedbackBundle(out, feedbackInput{
		CLIConfig:  &harnessconfig.Config{StarredModels: []string{"gpt-4o"}},
		RolloutDir: rolloutDir,
		BaseURL:    "http://localhost:8080",
		Model:      "gpt-4o",
	})
	if err != nil {
		t.Fatalf("buildFeedbackBundle: %v", err)
	}

	names := zipMemberNames(t, out)
	has := func(want string) bool {
		for _, n := range names {
			if n == want {
				return true
			}
		}
		return false
	}
	if !has("version.json") {
		t.Errorf("bundle missing version.json: %v", names)
	}
	if !has("config.json") {
		t.Errorf("bundle missing config.json: %v", names)
	}
	rolloutCount := 0
	for _, n := range names {
		if strings.HasPrefix(n, "rollouts/") && strings.HasSuffix(n, ".jsonl") {
			rolloutCount++
		}
	}
	if rolloutCount != 5 {
		t.Errorf("bundle must contain the newest 5 rollout files, got %d: %v", rolloutCount, names)
	}
	if has("rollouts/2026-07-01/run-1.jsonl") || has("rollouts/2026-07-01/run-2.jsonl") {
		t.Errorf("oldest rollout files must be excluded by the cap: %v", names)
	}

	// version.json content.
	var info struct {
		HarnesscliVersion string   `json:"harnesscli_version"`
		GoVersion         string   `json:"go_version"`
		GOOS              string   `json:"goos"`
		GOARCH            string   `json:"goarch"`
		BaseURL           string   `json:"base_url"`
		Model             string   `json:"model"`
		Notes             []string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(readZipMember(t, out, "version.json")), &info); err != nil {
		t.Fatalf("version.json does not parse: %v", err)
	}
	if info.HarnesscliVersion != "unstamped" {
		t.Errorf("harnesscli_version = %q, want unstamped (no version stamp landed yet)", info.HarnesscliVersion)
	}
	if info.GoVersion != runtime.Version() || info.GOOS != runtime.GOOS || info.GOARCH != runtime.GOARCH {
		t.Errorf("runtime info wrong: %+v", info)
	}
	if info.BaseURL != "http://localhost:8080" || info.Model != "gpt-4o" {
		t.Errorf("base_url/model not carried through: %+v", info)
	}

	// config.json content.
	var cfg map[string]any
	if err := json.Unmarshal([]byte(readZipMember(t, out, "config.json")), &cfg); err != nil {
		t.Fatalf("config.json does not parse: %v", err)
	}
	if !strings.Contains(readZipMember(t, out, "config.json"), "gpt-4o") {
		t.Errorf("config.json should carry non-secret fields: %v", cfg)
	}
}

func TestBuildFeedbackBundle_CapturesActiveContextRequestTranscriptLogsAndScreenshot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	screenshot := writeFeedbackPNG(t, tmp, "sh0rt-screen.png")
	stdoutLog := filepath.Join(tmp, "harnessd.stdout.log")
	if err := os.WriteFile(stdoutLog, []byte("request failed with sk-logcanary1234567890abcdef\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	out := filepath.Join(tmp, "bundle.zip")
	err := buildFeedbackBundle(out, feedbackInput{
		CLIConfig:      &harnessconfig.Config{APIKeys: map[string]string{"custom": "sh0rt"}},
		Request:        "Please fix export; tokens sk-requestcanary1234567890 and sh0rt",
		Workspace:      "/work/acme",
		RunID:          "run-123",
		ConversationID: "conversation-456",
		RunActive:      true,
		LastEventID:    "event-789",
		Transcript: []transcriptexport.TranscriptEntry{
			{Role: "user", Content: "Export the report", Timestamp: time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)},
			{Role: "assistant", Content: "failed using sk-transcriptcanary1234567890 and sh0rt", Timestamp: time.Date(2026, 7, 30, 10, 0, 1, 0, time.UTC)},
		},
		ScreenshotPath: screenshot,
		ServiceLogPaths: map[string]string{
			"harnessd.stdout.log": stdoutLog,
		},
	})
	if err != nil {
		t.Fatalf("buildFeedbackBundle: %v", err)
	}

	names := strings.Join(zipMemberNames(t, out), "\n")
	for _, want := range []string{
		"request.md",
		"context.json",
		"transcript.json",
		"logs/harnessd.stdout.log",
		"attachments/screenshot.png",
		"attachments/screenshot.json",
	} {
		if !strings.Contains(names, want) {
			t.Errorf("bundle missing %s; members:\n%s", want, names)
		}
	}

	request := readZipMember(t, out, "request.md")
	if !strings.Contains(request, "Please fix export") ||
		strings.Contains(request, "sk-requestcanary") || strings.Contains(request, "sh0rt") {
		t.Errorf("request.md must preserve the request and redact its secret:\n%s", request)
	}
	transcript := readZipMember(t, out, "transcript.json")
	if !strings.Contains(transcript, "Export the report") ||
		strings.Contains(transcript, "sk-transcriptcanary") || strings.Contains(transcript, "sh0rt") {
		t.Errorf("transcript.json must preserve transcript context and redact its secret:\n%s", transcript)
	}
	logText := readZipMember(t, out, "logs/harnessd.stdout.log")
	if strings.Contains(logText, "sk-logcanary") {
		t.Errorf("service log secret survived:\n%s", logText)
	}

	var contextInfo struct {
		Workspace      string `json:"workspace"`
		RunID          string `json:"run_id"`
		ConversationID string `json:"conversation_id"`
		RunActive      bool   `json:"run_active"`
		LastEventID    string `json:"last_event_id"`
	}
	if err := json.Unmarshal([]byte(readZipMember(t, out, "context.json")), &contextInfo); err != nil {
		t.Fatalf("context.json does not parse: %v", err)
	}
	if contextInfo.Workspace != "/work/acme" || contextInfo.RunID != "run-123" ||
		contextInfo.ConversationID != "conversation-456" || !contextInfo.RunActive ||
		contextInfo.LastEventID != "event-789" {
		t.Errorf("context.json lost active execution context: %+v", contextInfo)
	}

	var attachmentInfo struct {
		OriginalName string `json:"original_name"`
		MediaType    string `json:"media_type"`
		SHA256       string `json:"sha256"`
		SizeBytes    int64  `json:"size_bytes"`
		PixelNote    string `json:"pixel_redaction"`
	}
	if err := json.Unmarshal([]byte(readZipMember(t, out, "attachments/screenshot.json")), &attachmentInfo); err != nil {
		t.Fatalf("screenshot metadata does not parse: %v", err)
	}
	if strings.Contains(attachmentInfo.OriginalName, "sh0rt") || attachmentInfo.MediaType != "image/png" ||
		attachmentInfo.SHA256 == "" || attachmentInfo.SizeBytes <= 0 ||
		!strings.Contains(attachmentInfo.PixelNote, "not redacted") {
		t.Errorf("screenshot metadata incomplete: %+v", attachmentInfo)
	}
}

func TestBuildFeedbackBundle_RejectsDisguisedScreenshot(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "not-really.png")
	if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
		t.Fatalf("write fake screenshot: %v", err)
	}
	err := buildFeedbackBundle(filepath.Join(t.TempDir(), "bundle.zip"), feedbackInput{ScreenshotPath: path})
	if err == nil || !strings.Contains(err.Error(), "PNG or JPEG") {
		t.Fatalf("buildFeedbackBundle error = %v, want PNG/JPEG validation failure", err)
	}
}

func TestBuildFeedbackBundle_RejectsSymlinkAndOversizedScreenshot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	target := writeFeedbackPNG(t, tmp, "target.png")
	link := filepath.Join(tmp, "link.png")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink screenshot: %v", err)
	}
	err := buildFeedbackBundle(filepath.Join(tmp, "symlink.zip"), feedbackInput{ScreenshotPath: link})
	if err == nil || !strings.Contains(err.Error(), "regular PNG or JPEG") {
		t.Fatalf("symlink screenshot error = %v", err)
	}

	oversized := filepath.Join(tmp, "oversized.png")
	f, err := os.Create(oversized)
	if err != nil {
		t.Fatalf("create oversized screenshot: %v", err)
	}
	if err := f.Truncate(maxFeedbackScreenshotBytes + 1); err != nil {
		f.Close()
		t.Fatalf("truncate oversized screenshot: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close oversized screenshot: %v", err)
	}
	err = buildFeedbackBundle(filepath.Join(tmp, "oversized.zip"), feedbackInput{ScreenshotPath: oversized})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversized screenshot error = %v", err)
	}
}

func TestBuildFeedbackBundle_RecordsTranscriptTruncation(t *testing.T) {
	t.Parallel()

	entries := make([]transcriptexport.TranscriptEntry, maxFeedbackTranscriptEntries+3)
	for i := range entries {
		entries[i] = transcriptexport.TranscriptEntry{
			Role:      "user",
			Content:   "entry",
			Timestamp: time.Date(2026, 7, 30, 12, 0, i, 0, time.UTC),
		}
	}
	out := filepath.Join(t.TempDir(), "bundle.zip")
	if err := buildFeedbackBundle(out, feedbackInput{Transcript: entries}); err != nil {
		t.Fatalf("buildFeedbackBundle: %v", err)
	}
	var manifest struct {
		SourceEntryCount   int  `json:"source_entry_count"`
		IncludedEntryCount int  `json:"included_entry_count"`
		Truncated          bool `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(readZipMember(t, out, "transcript.json")), &manifest); err != nil {
		t.Fatalf("transcript.json does not parse: %v", err)
	}
	if manifest.SourceEntryCount != maxFeedbackTranscriptEntries+3 ||
		manifest.IncludedEntryCount != maxFeedbackTranscriptEntries || !manifest.Truncated {
		t.Errorf("transcript truncation provenance missing: %+v", manifest)
	}
}

func TestFeedbackIssueTitle_RedactsSecrets(t *testing.T) {
	t.Parallel()

	title := feedbackIssueTitle(
		"Fix export with sh0rt and sk-titlecanary1234567890abcdef",
		&harnessconfig.Config{APIKeys: map[string]string{"custom": "sh0rt"}},
	)
	if strings.Contains(title, "sh0rt") || strings.Contains(title, "sk-titlecanary") {
		t.Fatalf("issue title leaked configured or patterned secret: %q", title)
	}
}

func TestFeedbackIssueDraft_RedactsTextAndExternalFailureKeepsBundleStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tmp := t.TempDir()
	issuePath := filepath.Join(tmp, "feedback-issue.md")
	bundlePath := filepath.Join(tmp, "feedback.zip")
	if err := writeFeedbackIssueDraft(issuePath, bundlePath, feedbackInput{
		CLIConfig:      &harnessconfig.Config{APIKeys: map[string]string{"custom": "sh0rt"}},
		Request:        "Fix this with sk-issuecanary1234567890abcdef and sh0rt",
		Workspace:      "/work/private-project",
		Model:          "gpt-test",
		RunID:          "run-1",
		ConversationID: "conversation-1",
		Now:            time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("writeFeedbackIssueDraft: %v", err)
	}
	body, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue draft: %v", err)
	}
	if strings.Contains(string(body), "sk-issuecanary") || strings.Contains(string(body), "sh0rt") ||
		!strings.Contains(string(body), filepath.Base(bundlePath)) {
		t.Errorf("issue draft must redact request secrets and name recoverable bundle:\n%s", body)
	}

	m := New(DefaultTUIConfig())
	updated, _ := m.Update(feedbackIssuePublishResultMsg{
		bundlePath: bundlePath,
		err:        errors.New("gh unavailable"),
	})
	got := updated.(Model).StatusMsg()
	if !strings.Contains(got, bundlePath) || !strings.Contains(got, "gh unavailable") {
		t.Errorf("partial-success status must retain bundle path and external error: %q", got)
	}
}

func TestFeedbackIssuePublishCmd_ExecutesGHAndReportsIssueURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := t.TempDir()
	argsPath := filepath.Join(t.TempDir(), "gh-args.txt")
	t.Setenv("FEEDBACK_ARGS_PATH", argsPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	fakeGH := filepath.Join(binDir, "gh")
	script := `#!/bin/sh
printf 'CALL\n' >> "$FEEDBACK_ARGS_PATH"
printf '%s\n' "$@" >> "$FEEDBACK_ARGS_PATH"
if [ "$1 $2" = "release view" ]; then
  exit 1
fi
if [ "$1 $2" = "issue create" ]; then
  printf 'https://github.com/dennisonbertram/go-code/issues/999\n'
fi
`
	if err := os.WriteFile(fakeGH, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}

	workspace := t.TempDir()
	bodyPath := filepath.Join(t.TempDir(), "issue.md")
	if err := os.WriteFile(bodyPath, []byte("# Feedback\n"), 0o600); err != nil {
		t.Fatalf("write issue body: %v", err)
	}
	bundlePath := filepath.Join(t.TempDir(), "bundle.zip")
	screenshotPath := filepath.Join(t.TempDir(), "screen.png")
	imagePaths := []string{screenshotPath}

	command := feedbackIssuePublishCmd(feedbackPublishRequest{
		Workspace:  workspace,
		Title:      "Feedback: export failed",
		BodyPath:   bodyPath,
		BundlePath: bundlePath,
		ImagePaths: imagePaths,
	}, []string{"/tmp/original-attachment.png"})
	imagePaths[0] = "/tmp/mutated-after-command-construction.png"
	message, ok := command().(feedbackIssuePublishResultMsg)
	if !ok {
		t.Fatalf("feedbackIssuePublishCmd returned %T", command())
	}
	if message.err != nil {
		t.Fatalf("feedbackIssuePublishCmd: %v", message.err)
	}
	if message.issueURL != "https://github.com/dennisonbertram/go-code/issues/999" {
		t.Fatalf("issue URL = %q", message.issueURL)
	}
	body, err := os.ReadFile(bodyPath)
	if err != nil {
		t.Fatalf("read issue body: %v", err)
	}
	if !strings.Contains(string(body), filepath.Base(screenshotPath)) ||
		strings.Contains(string(body), "mutated-after-command-construction") {
		t.Errorf("async publisher did not own a stable image-path snapshot:\n%s", body)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read fake gh args: %v", err)
	}
	joined := string(args)
	for _, want := range []string{
		"release\nview\n",
		"release\ncreate\n",
		"release\nupload\n",
		"issue\ncreate\n",
		"--repo\n" + feedbackIssueRepository + "\n",
		"--body-file\n" + bodyPath + "\n",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("fake gh args missing %q:\n%s", want, joined)
		}
	}

	model := New(DefaultTUIConfig())
	updated, _ := model.Update(message)
	status := updated.(Model).StatusMsg()
	if !strings.Contains(status, "/issues/999") {
		t.Errorf("success status must name the created issue: %q", status)
	}
}

func TestCopyFeedbackScreenshots_WritesDurableOrderedSidecars(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	first := writeFeedbackPNG(t, tmp, "first local upload.png")
	second := writeFeedbackPNG(t, tmp, "second local upload.png")
	bundlePath := filepath.Join(tmp, "go-code-feedback-unique.zip")

	sidecars, err := copyFeedbackScreenshots(bundlePath, []string{first, second})
	if err != nil {
		t.Fatalf("copy feedback screenshots: %v", err)
	}
	if len(sidecars) != 2 {
		t.Fatalf("sidecars = %v, want two", sidecars)
	}
	for i, sidecar := range sidecars {
		wantSuffix := fmt.Sprintf("-image-%d.png", i+1)
		if !strings.HasSuffix(sidecar, wantSuffix) {
			t.Errorf("sidecar %d = %q, want suffix %q", i, sidecar, wantSuffix)
		}
		info, statErr := os.Stat(sidecar)
		if statErr != nil {
			t.Fatalf("stat sidecar %s: %v", sidecar, statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("sidecar %s mode = %o, want 600", sidecar, info.Mode().Perm())
		}
	}
}

func TestCopyFeedbackScreenshots_DeduplicatesCapturedPaths(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	imagePath := writeFeedbackPNG(t, tmp, "same-image.png")
	bundlePath := filepath.Join(tmp, "go-code-feedback-unique.zip")

	sidecars, err := copyFeedbackScreenshots(bundlePath, []string{imagePath, imagePath})
	if err != nil {
		t.Fatalf("copy duplicate feedback screenshots: %v", err)
	}
	if len(sidecars) != 1 {
		t.Fatalf("duplicate screenshot sidecars = %v, want one", sidecars)
	}
}

func TestPublishFeedbackIssue_UploadsAssetsAndCreatesDirectIssue(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	bodyPath := filepath.Join(tmp, "issue.md")
	if err := os.WriteFile(bodyPath, []byte("# Feedback\n"), 0o600); err != nil {
		t.Fatalf("write issue body: %v", err)
	}
	bundlePath := filepath.Join(tmp, "bundle-123.zip")
	imagePaths := []string{
		filepath.Join(tmp, "bundle-123-image-1.png"),
		filepath.Join(tmp, "bundle-123-image-2.jpg"),
	}

	type invocation struct {
		dir  string
		name string
		args []string
	}
	var calls []invocation
	runner := func(dir, name string, args ...string) (string, error) {
		calls = append(calls, invocation{dir: dir, name: name, args: append([]string{}, args...)})
		switch {
		case len(args) >= 2 && args[0] == "release" && args[1] == "view":
			return "", errors.New("release not found")
		case len(args) >= 2 && args[0] == "issue" && args[1] == "create":
			return "https://github.com/dennisonbertram/go-code/issues/999\n", nil
		default:
			return "", nil
		}
	}

	issueURL, err := publishFeedbackIssue(feedbackPublishRequest{
		Workspace:  "/work/acme",
		Title:      "Feedback: export failed",
		BodyPath:   bodyPath,
		BundlePath: bundlePath,
		ImagePaths: imagePaths,
	}, runner)
	if err != nil {
		t.Fatalf("publish feedback issue: %v", err)
	}
	if issueURL != "https://github.com/dennisonbertram/go-code/issues/999" {
		t.Fatalf("issue URL = %q", issueURL)
	}

	var sequence []string
	for _, call := range calls {
		if call.dir != "/work/acme" || call.name != "gh" {
			t.Errorf("runner call dir/name = %q/%q", call.dir, call.name)
		}
		if len(call.args) >= 2 {
			sequence = append(sequence, call.args[0]+" "+call.args[1])
		}
		if slices.Contains(call.args, "--web") {
			t.Errorf("direct issue creation must not use --web: %v", call.args)
		}
	}
	wantSequence := []string{"release view", "release create", "release upload", "issue create"}
	if !slices.Equal(sequence, wantSequence) {
		t.Fatalf("GitHub command sequence = %v, want %v", sequence, wantSequence)
	}
	uploadArgs := calls[2].args
	for _, path := range append([]string{bundlePath}, imagePaths...) {
		if !slices.Contains(uploadArgs, path) {
			t.Errorf("release upload args missing %q: %v", path, uploadArgs)
		}
	}
	issueArgs := calls[3].args
	for _, want := range []string{
		"--repo", feedbackIssueRepository,
		"--title", "Feedback: export failed",
		"--body-file", bodyPath,
	} {
		if !slices.Contains(issueArgs, want) {
			t.Errorf("issue create args missing %q: %v", want, issueArgs)
		}
	}

	body, err := os.ReadFile(bodyPath)
	if err != nil {
		t.Fatalf("read published issue body: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"/releases/download/" + feedbackAssetReleaseTag + "/" + filepath.Base(bundlePath),
		"/releases/download/" + feedbackAssetReleaseTag + "/" + filepath.Base(imagePaths[0]),
		"/releases/download/" + feedbackAssetReleaseTag + "/" + filepath.Base(imagePaths[1]),
		"![Feedback image 1]",
		"![Feedback image 2]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("published issue body missing %q:\n%s", want, text)
		}
	}
}

func TestPublishFeedbackIssue_ReusesReleaseAndStopsBeforeIssueOnUploadFailure(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	bodyPath := filepath.Join(tmp, "issue.md")
	if err := os.WriteFile(bodyPath, []byte("# Feedback\n"), 0o600); err != nil {
		t.Fatalf("write issue body: %v", err)
	}

	var sequence []string
	runner := func(_ string, _ string, args ...string) (string, error) {
		sequence = append(sequence, strings.Join(args[:2], " "))
		if args[0] == "release" && args[1] == "upload" {
			return "", errors.New("upload denied")
		}
		return feedbackAssetReleaseTag, nil
	}
	_, err := publishFeedbackIssue(feedbackPublishRequest{
		Workspace:  "/work/acme",
		Title:      "Feedback",
		BodyPath:   bodyPath,
		BundlePath: filepath.Join(tmp, "bundle.zip"),
	}, runner)
	if err == nil || !strings.Contains(err.Error(), "upload") {
		t.Fatalf("publish error = %v, want upload failure", err)
	}
	if !slices.Equal(sequence, []string{"release view", "release upload"}) {
		t.Fatalf("command sequence = %v, issue creation must not run after upload failure", sequence)
	}
}

func TestFeedbackPublishResult_ConsumesOnlyCapturedChipsOnSuccessAndKeepsAllOnFailure(t *testing.T) {
	captured := inputarea.Attachment{Path: "/tmp/captured/image.png", MediaType: "image/png"}
	newer := inputarea.Attachment{Path: "/tmp/newer/image.png", MediaType: "image/png"}

	success := New(DefaultTUIConfig())
	success.input = success.input.AddAttachment(captured).AddAttachment(newer)
	updated, _ := success.Update(feedbackIssuePublishResultMsg{
		bundlePath:              "/tmp/feedback.zip",
		issueURL:                "https://github.com/dennisonbertram/go-code/issues/999",
		capturedAttachmentPaths: []string{captured.Path},
	})
	success = updated.(Model)
	if got := success.input.Attachments(); len(got) != 1 || got[0].Path != newer.Path {
		t.Fatalf("success attachments = %+v, want only newer chip", got)
	}
	if !strings.Contains(success.StatusMsg(), "/issues/999") {
		t.Errorf("success status missing issue URL: %q", success.StatusMsg())
	}

	failed := New(DefaultTUIConfig())
	failed.input = failed.input.AddAttachment(captured).AddAttachment(newer)
	updated, _ = failed.Update(feedbackIssuePublishResultMsg{
		bundlePath:              "/tmp/feedback.zip",
		capturedAttachmentPaths: []string{captured.Path},
		err:                     errors.New("gh unavailable"),
	})
	failed = updated.(Model)
	if got := failed.input.Attachments(); len(got) != 2 {
		t.Fatalf("failure attachments = %+v, want all chips retained", got)
	}
	if !strings.Contains(failed.StatusMsg(), "/tmp/feedback.zip") ||
		!strings.Contains(failed.StatusMsg(), "gh unavailable") {
		t.Errorf("failure status missing recovery evidence: %q", failed.StatusMsg())
	}
}

// ─── Redaction canaries ───────────────────────────────────────────────────────

// TestBuildFeedbackBundle_RedactsConfigSecrets is the canary table: no secret
// placed anywhere in the CLI config may survive into the bundled config.json.
func TestBuildFeedbackBundle_RedactsConfigSecrets(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		cfg    *harnessconfig.Config
		canary string
	}{
		{
			name:   "sk- API key in api_keys",
			cfg:    &harnessconfig.Config{APIKeys: map[string]string{"openai": "sk-testcanary1234567890abcdef"}},
			canary: "sk-testcanary1234567890abcdef",
		},
		{
			name:   "short non-pattern key caught by exact-value replace",
			cfg:    &harnessconfig.Config{APIKeys: map[string]string{"weird": "sh0rt"}},
			canary: "sh0rt",
		},
		{
			name:   "JWT in api_keys",
			cfg:    &harnessconfig.Config{APIKeys: map[string]string{"svc": "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"}},
			canary: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c",
		},
		{
			name:   "AWS access key id in history",
			cfg:    &harnessconfig.Config{HistoryEntries: []string{"aws configure set AKIAIOSFODNN7EXAMPLE"}},
			canary: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:   "postgres connection string in history",
			cfg:    &harnessconfig.Config{HistoryEntries: []string{"psql postgres://admin:hunter2@db.internal:5432/prod"}},
			canary: "postgres://admin:hunter2@db.internal:5432/prod",
		},
		{
			name:   "sk- key pasted into history",
			cfg:    &harnessconfig.Config{HistoryEntries: []string{"/keys openai sk-pastedcanary1234567890"}},
			canary: "sk-pastedcanary1234567890",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(t.TempDir(), "bundle.zip")
			if err := buildFeedbackBundle(out, feedbackInput{CLIConfig: tc.cfg}); err != nil {
				t.Fatalf("buildFeedbackBundle: %v", err)
			}
			configJSON := readZipMember(t, out, "config.json")
			if strings.Contains(configJSON, tc.canary) {
				t.Errorf("canary secret survived into bundled config.json: %q\n%s", tc.canary, configJSON)
			}
			if !strings.Contains(configJSON, "[REDACTED") {
				t.Errorf("expected a redaction marker in bundled config.json: %s", configJSON)
			}
		})
	}
}

// TestBuildFeedbackBundle_RedactsRolloutContent verifies rollout files are
// redacted before bundling too.
func TestBuildFeedbackBundle_RedactsRolloutContent(t *testing.T) {
	t.Parallel()

	rolloutDir := t.TempDir()
	writeRolloutFile(t, rolloutDir, "2026-07-19", "run-x.jsonl",
		`{"event":"tool.result","output":"key was sk-rolloutcanary1234567890 ok"}`+"\n", time.Now())

	out := filepath.Join(t.TempDir(), "bundle.zip")
	if err := buildFeedbackBundle(out, feedbackInput{RolloutDir: rolloutDir}); err != nil {
		t.Fatalf("buildFeedbackBundle: %v", err)
	}
	bundled := readZipMember(t, out, "rollouts/2026-07-19/run-x.jsonl")
	if strings.Contains(bundled, "sk-rolloutcanary1234567890") {
		t.Errorf("rollout secret survived into the bundle:\n%s", bundled)
	}
}

// ─── Rollout dir absence ──────────────────────────────────────────────────────

// TestBuildFeedbackBundle_RolloutDirUnset verifies the bundle still builds
// when no rollout dir is configured, noting the absence.
func TestBuildFeedbackBundle_RolloutDirUnset(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "bundle.zip")
	if err := buildFeedbackBundle(out, feedbackInput{RolloutDir: ""}); err != nil {
		t.Fatalf("buildFeedbackBundle with unset rollout dir: %v", err)
	}

	names := zipMemberNames(t, out)
	foundMarker := false
	for _, n := range names {
		if strings.HasPrefix(n, "rollouts/") {
			foundMarker = true
		}
	}
	if !foundMarker {
		t.Fatalf("bundle must note the absent rollouts (rollouts/ marker member), got %v", names)
	}
	marker := readZipMember(t, out, "rollouts/NOT_PRESENT.txt")
	if !strings.Contains(marker, "rollout") {
		t.Errorf("absence marker should explain the missing rollouts, got: %q", marker)
	}

	var info struct {
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(readZipMember(t, out, "version.json")), &info); err != nil {
		t.Fatalf("version.json does not parse: %v", err)
	}
	joined := strings.Join(info.Notes, " ")
	if !strings.Contains(joined, "rollout") {
		t.Errorf("version.json notes should mention the rollout dir absence, got %v", info.Notes)
	}
}

// TestBuildFeedbackBundle_RolloutDirMissing verifies a configured-but-missing
// rollout dir degrades the same way.
func TestBuildFeedbackBundle_RolloutDirMissing(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "bundle.zip")
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	if err := buildFeedbackBundle(out, feedbackInput{RolloutDir: missing}); err != nil {
		t.Fatalf("buildFeedbackBundle with missing rollout dir: %v", err)
	}
	marker := readZipMember(t, out, "rollouts/NOT_PRESENT.txt")
	if !strings.Contains(marker, "rollout") {
		t.Errorf("absence marker should explain the missing rollouts, got: %q", marker)
	}
}
