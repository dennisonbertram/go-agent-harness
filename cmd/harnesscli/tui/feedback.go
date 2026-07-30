package tui

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	harnessconfig "go-agent-harness/cmd/harnesscli/config"
	"go-agent-harness/cmd/harnesscli/tui/components/transcriptexport"
	"go-agent-harness/internal/forensics/redaction"
)

// maxFeedbackRollouts caps how many of the newest rollout JSONL files go into
// a feedback bundle.
const maxFeedbackRollouts = 5

const (
	maxFeedbackRolloutBytes = 1 << 20
	maxFeedbackLogBytes     = 256 << 10
)

// feedbackInput carries everything buildFeedbackBundle needs; it is a plain
// value so the write path is testable without a TUI model.
type feedbackInput struct {
	// CLIConfig is the persistent harnesscli config (nil tolerated).
	CLIConfig *harnessconfig.Config
	// RolloutDir is the harness rollout directory ("" means not configured).
	RolloutDir string
	BaseURL    string
	Model      string
	// Version is the harnesscli build version stamp; "" means unstamped.
	Version string
	// Notes are extra human-readable caveats recorded in version.json.
	Notes []string
	// Now overrides the timestamp (zero → time.Now()).
	Now time.Time
	// Request is the user's explicit feedback or fix request.
	Request string
	// Workspace and run fields snapshot the active TUI state at invocation.
	Workspace      string
	RunID          string
	ConversationID string
	RunActive      bool
	LastEventID    string
	Transcript     []transcriptexport.TranscriptEntry
	// ScreenshotPath is an optional user-selected PNG or JPEG.
	ScreenshotPath string
	// ServiceLogPaths maps stable bundle names to local service log paths.
	ServiceLogPaths map[string]string
}

// executeFeedbackCommand implements:
//
//	/feedback [--issue] [--screenshot <png-or-jpeg>] [--] [request]
//
// Bundle creation is local and synchronous so the evidence snapshot matches
// the invocation point. The optional GitHub browser handoff is asynchronous
// and only happens after the user explicitly supplies --issue.
func executeFeedbackCommand(m *Model, command Command) ([]tea.Cmd, bool) {
	options, err := parseFeedbackOptions(command.Raw)
	if err != nil {
		return []tea.Cmd{m.setStatusMsg("Feedback usage: /feedback [--issue] [--screenshot <path>] [--] [request] (" + err.Error() + ")")}, false
	}

	var notes []string
	cfg, err := harnessconfig.Load()
	if err != nil {
		cfg = nil
		notes = append(notes, "cli config unreadable: "+err.Error())
	}
	rolloutDir := strings.TrimSpace(os.Getenv("HARNESS_ROLLOUT_DIR"))

	outDir := filepath.Join(defaultSessionConfigDir(), "feedback")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return []tea.Cmd{m.setStatusMsg("Could not create feedback dir: " + err.Error())}, false
	}
	now := time.Now()
	reserved, err := os.CreateTemp(outDir, "go-code-feedback-"+now.Format("20060102-150405")+"-*.zip")
	if err != nil {
		return []tea.Cmd{m.setStatusMsg("Could not reserve feedback bundle: " + err.Error())}, false
	}
	outPath := reserved.Name()
	if err := reserved.Close(); err != nil {
		_ = os.Remove(outPath)
		return []tea.Cmd{m.setStatusMsg("Could not reserve feedback bundle: " + err.Error())}, false
	}

	input := feedbackInput{
		CLIConfig:       cfg,
		RolloutDir:      rolloutDir,
		BaseURL:         m.config.BaseURL,
		Model:           m.selectedModel,
		Notes:           notes,
		Now:             now,
		Request:         options.Request,
		Workspace:       m.config.Workspace,
		RunID:           m.RunID,
		ConversationID:  m.conversationID,
		RunActive:       m.runActive,
		LastEventID:     m.lastEventID,
		Transcript:      append([]transcriptexport.TranscriptEntry{}, m.transcript...),
		ScreenshotPath:  options.ScreenshotPath,
		ServiceLogPaths: defaultFeedbackServiceLogPaths(),
	}
	err = buildFeedbackBundle(outPath, input)
	if err != nil {
		_ = os.Remove(outPath)
		return []tea.Cmd{m.setStatusMsg("Could not write feedback bundle: " + err.Error())}, false
	}

	issuePath := strings.TrimSuffix(outPath, ".zip") + "-issue.md"
	if err := writeFeedbackIssueDraft(issuePath, outPath, input); err != nil {
		return []tea.Cmd{m.setStatusMsg("Feedback bundle written to " + outPath + "; issue draft failed: " + err.Error())}, false
	}
	if !options.OpenIssue {
		return []tea.Cmd{m.setStatusMsg("Feedback bundle written to " + outPath)}, false
	}
	title := feedbackIssueTitle(options.Request, input.CLIConfig)
	return []tea.Cmd{
		m.setStatusMsg("Feedback bundle written to " + outPath + "; opening GitHub issue draft"),
		feedbackIssueDraftCmd(m.config.Workspace, title, issuePath, outPath, options.ScreenshotPath),
	}, false
}

// buildFeedbackBundle writes the diagnostics zip to outPath. The bundle never
// contains secrets: the CLI config passes through exact-value replacement of
// every stored api_keys value plus the forensics redaction patterns, and
// rollout files pass through the same redactor.
func buildFeedbackBundle(outPath string, in feedbackInput) error {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	redactor := newFeedbackRedactor(in.CLIConfig)
	notes := append([]string{}, in.Notes...)
	screenshot, err := loadFeedbackScreenshot(in.ScreenshotPath)
	if err != nil {
		return err
	}
	if screenshot != nil {
		screenshot.metadata.OriginalName = redactor.Redact(screenshot.metadata.OriginalName)
	}

	zf, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create bundle: %w", err)
	}
	zw := zip.NewWriter(zf)
	fail := func(cause error) error {
		_ = zw.Close()
		_ = zf.Close()
		_ = os.Remove(outPath)
		return cause
	}

	// version.json — version/runtime info plus caveats.
	version := in.Version
	if version == "" {
		version = "unstamped"
	}
	rolloutFiles, rolloutNote := collectFeedbackRollouts(in.RolloutDir, maxFeedbackRollouts)
	if rolloutNote != "" {
		notes = append(notes, rolloutNote)
	}
	for i := range notes {
		notes[i] = redactor.Redact(notes[i])
	}
	info := map[string]any{
		"harnesscli_version": version,
		"go_version":         runtime.Version(),
		"goos":               runtime.GOOS,
		"goarch":             runtime.GOARCH,
		"base_url":           redactor.Redact(in.BaseURL),
		"model":              redactor.Redact(in.Model),
		"generated_at":       now.UTC().Format(time.RFC3339),
		"notes":              notes,
	}
	infoJSON, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fail(fmt.Errorf("marshal version.json: %w", err))
	}
	if err := writeZipMember(zw, "version.json", infoJSON); err != nil {
		return fail(err)
	}

	// config.json — redacted CLI config.
	if err := writeZipMember(zw, "config.json", redactCLIConfigJSON(in.CLIConfig, redactor)); err != nil {
		return fail(err)
	}

	request := strings.TrimSpace(in.Request)
	if request == "" {
		request = "No explicit request supplied."
	}
	if err := writeZipMember(zw, "request.md", []byte(redactor.Redact(request)+"\n")); err != nil {
		return fail(err)
	}

	contextInfo := map[string]any{
		"schema_version":  1,
		"generated_at":    now.UTC().Format(time.RFC3339),
		"workspace":       redactor.Redact(in.Workspace),
		"run_id":          redactor.Redact(in.RunID),
		"conversation_id": redactor.Redact(in.ConversationID),
		"run_active":      in.RunActive,
		"last_event_id":   redactor.Redact(in.LastEventID),
		"base_url":        redactor.Redact(in.BaseURL),
		"model":           redactor.Redact(in.Model),
	}
	contextJSON, err := json.MarshalIndent(contextInfo, "", "  ")
	if err != nil {
		return fail(fmt.Errorf("marshal context.json: %w", err))
	}
	if err := writeZipMember(zw, "context.json", contextJSON); err != nil {
		return fail(err)
	}

	transcriptJSON, err := marshalFeedbackTranscript(in.Transcript, redactor)
	if err != nil {
		return fail(err)
	}
	if err := writeZipMember(zw, "transcript.json", transcriptJSON); err != nil {
		return fail(err)
	}

	logMembers := collectFeedbackLogMembers(in.ServiceLogPaths, redactor)
	if len(logMembers) == 0 {
		if err := writeZipMember(zw, "logs/NOT_PRESENT.txt", []byte("no harness service logs were available\n")); err != nil {
			return fail(err)
		}
	}
	for _, logMember := range logMembers {
		if err := writeZipMember(zw, logMember.name, logMember.data); err != nil {
			return fail(err)
		}
	}

	if screenshot != nil {
		if err := writeZipMember(zw, screenshot.member, screenshot.data); err != nil {
			return fail(err)
		}
		metadata, err := json.MarshalIndent(screenshot.metadata, "", "  ")
		if err != nil {
			return fail(fmt.Errorf("marshal screenshot metadata: %w", err))
		}
		if err := writeZipMember(zw, "attachments/screenshot.json", metadata); err != nil {
			return fail(err)
		}
	}

	// rollouts/ — newest rollout files, redacted; absence marker otherwise.
	if len(rolloutFiles) == 0 {
		marker := "no rollout files included: " + rolloutNote + "\n"
		if rolloutNote == "" {
			marker = "no rollout files included\n"
		}
		if err := writeZipMember(zw, "rollouts/NOT_PRESENT.txt", []byte(marker)); err != nil {
			return fail(err)
		}
	}
	for _, rf := range rolloutFiles {
		if err := writeZipMember(zw, rf.member, redactFileTail(rf.absPath, maxFeedbackRolloutBytes, redactor)); err != nil {
			return fail(err)
		}
	}

	if err := zw.Close(); err != nil {
		_ = zf.Close()
		_ = os.Remove(outPath)
		return fmt.Errorf("finalize bundle: %w", err)
	}
	if err := zf.Close(); err != nil {
		_ = os.Remove(outPath)
		return fmt.Errorf("close bundle: %w", err)
	}
	return nil
}

// rolloutFile pairs an on-disk rollout JSONL path with its intended zip
// member name.
type rolloutFile struct {
	absPath string
	member  string
}

// collectFeedbackRollouts returns the newest max .jsonl files under dir
// (layout <dir>/<YYYY-MM-DD>/<run_id>.jsonl) as zip members preserving the
// dated subdirectory. The second return value explains an empty result:
// "" when files were found, otherwise a human-readable reason.
func collectFeedbackRollouts(dir string, max int) ([]rolloutFile, string) {
	if strings.TrimSpace(dir) == "" {
		return nil, "rollout dir not configured (HARNESS_ROLLOUT_DIR unset)"
	}
	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "rollout dir " + dir + " does not exist"
		}
		return nil, "rollout dir " + dir + " is not accessible: " + err.Error()
	}
	if !fi.IsDir() {
		return nil, "rollout dir " + dir + " is not a directory"
	}

	type candidate struct {
		absPath string
		member  string
		modTime time.Time
	}
	var found []candidate
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		found = append(found, candidate{absPath: path, member: "rollouts/" + filepath.ToSlash(rel), modTime: info.ModTime()})
		return nil
	})
	if len(found) == 0 {
		return nil, "no rollout files found under " + dir
	}

	// Newest first, then cap.
	sort.Slice(found, func(i, j int) bool { return found[i].modTime.After(found[j].modTime) })
	if len(found) > max {
		found = found[:max]
	}
	out := make([]rolloutFile, len(found))
	for i, c := range found {
		out[i] = rolloutFile{absPath: c.absPath, member: c.member}
	}
	return out, ""
}

// redactCLIConfigJSON marshals cfg and scrubs it: every stored api_keys value
// is replaced exactly (format-agnostic), then the forensics redaction
// patterns run over the whole document (catches secrets pasted into history).
func redactCLIConfigJSON(cfg *harnessconfig.Config, r *redaction.Redactor) []byte {
	if cfg == nil {
		cfg = &harnessconfig.Config{}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		data = []byte("{}")
	}
	text := string(data)
	for _, v := range cfg.APIKeys {
		if v != "" {
			text = strings.ReplaceAll(text, v, "[REDACTED:api_key]")
		}
	}
	return []byte(r.Redact(text))
}

func writeZipMember(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create member %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write member %s: %w", name, err)
	}
	return nil
}
