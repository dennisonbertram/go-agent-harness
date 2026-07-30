package tui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	harnessconfig "go-agent-harness/cmd/harnesscli/config"
	"go-agent-harness/cmd/harnesscli/tui/components/transcriptexport"
	"go-agent-harness/internal/forensics/redaction"
)

const (
	maxFeedbackScreenshotBytes      = 10 << 20
	maxFeedbackTranscriptEntries    = 200
	maxFeedbackTranscriptEntryBytes = 32 << 10
	feedbackIssueRepository         = "dennisonbertram/go-code"
)

type feedbackOptions struct {
	Request        string
	ScreenshotPath string
	OpenIssue      bool
}

// newFeedbackRedactor extends the built-in secret patterns with exact matches
// for every configured API-key value. This protects short or unusual provider
// keys if they reappear in a request, transcript, rollout, log, or issue body.
func newFeedbackRedactor(cfg *harnessconfig.Config) *redaction.Redactor {
	if cfg == nil {
		return redaction.NewRedactor(nil)
	}
	custom := make([]*regexp.Regexp, 0, len(cfg.APIKeys))
	for _, value := range cfg.APIKeys {
		if value != "" {
			custom = append(custom, regexp.MustCompile(regexp.QuoteMeta(value)))
		}
	}
	return redaction.NewRedactor(custom)
}

func parseFeedbackOptions(raw string) (feedbackOptions, error) {
	tokens, err := splitFeedbackCommand(raw)
	if err != nil {
		return feedbackOptions{}, err
	}
	if len(tokens) == 0 || strings.ToLower(strings.TrimPrefix(tokens[0], "/")) != "feedback" {
		return feedbackOptions{}, fmt.Errorf("invalid feedback command")
	}

	var result feedbackOptions
	var request []string
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "--":
			request = append(request, tokens[i+1:]...)
			i = len(tokens)
		case token == "--issue":
			result.OpenIssue = true
		case token == "--screenshot":
			if i+1 >= len(tokens) {
				return feedbackOptions{}, fmt.Errorf("--screenshot needs a path")
			}
			i++
			result.ScreenshotPath = tokens[i]
		case strings.HasPrefix(token, "--screenshot="):
			result.ScreenshotPath = strings.TrimPrefix(token, "--screenshot=")
			if result.ScreenshotPath == "" {
				return feedbackOptions{}, fmt.Errorf("--screenshot needs a path")
			}
		case strings.HasPrefix(token, "--"):
			return feedbackOptions{}, fmt.Errorf("unknown option %s", token)
		default:
			request = append(request, tokens[i:]...)
			i = len(tokens)
		}
	}
	result.Request = strings.TrimSpace(strings.Join(request, " "))
	return result, nil
}

// splitFeedbackCommand is intentionally local to /feedback. The general slash
// parser keeps its established whitespace behavior, while screenshot paths can
// use shell-style single/double quotes and backslash escaping.
func splitFeedbackCommand(input string) ([]string, error) {
	var tokens []string
	var token strings.Builder
	var quote rune
	escaped := false
	started := false

	flush := func() {
		if started {
			tokens = append(tokens, token.String())
			token.Reset()
			started = false
		}
	}
	for _, r := range strings.TrimSpace(input) {
		if escaped {
			token.WriteRune(r)
			started = true
			escaped = false
			continue
		}
		if quote != '\'' && r == '\\' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				started = true
			} else {
				token.WriteRune(r)
				started = true
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		token.WriteRune(r)
		started = true
	}
	if escaped {
		return nil, fmt.Errorf("unfinished escape")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	flush()
	return tokens, nil
}

type feedbackScreenshotMetadata struct {
	OriginalName   string `json:"original_name"`
	MediaType      string `json:"media_type"`
	SHA256         string `json:"sha256"`
	SizeBytes      int64  `json:"size_bytes"`
	PixelRedaction string `json:"pixel_redaction"`
}

type feedbackScreenshot struct {
	member   string
	data     []byte
	metadata feedbackScreenshotMetadata
}

func loadFeedbackScreenshot(path string) (*feedbackScreenshot, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("resolve screenshot: %w", err)
	}
	info, err := os.Lstat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read screenshot: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("screenshot must be a regular PNG or JPEG file")
	}
	if info.Size() > maxFeedbackScreenshotBytes {
		return nil, fmt.Errorf("screenshot exceeds the %d MiB limit", maxFeedbackScreenshotBytes>>20)
	}
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("read screenshot: %w", err)
	}
	if int64(len(data)) > maxFeedbackScreenshotBytes {
		return nil, fmt.Errorf("screenshot exceeds the %d MiB limit", maxFeedbackScreenshotBytes>>20)
	}

	mediaType := http.DetectContentType(data)
	extension := ""
	switch mediaType {
	case "image/png":
		if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("screenshot must be a valid PNG or JPEG: %w", err)
		}
		extension = ".png"
	case "image/jpeg":
		if _, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("screenshot must be a valid PNG or JPEG: %w", err)
		}
		extension = ".jpg"
	default:
		return nil, fmt.Errorf("screenshot must be a valid PNG or JPEG")
	}
	sum := sha256.Sum256(data)
	return &feedbackScreenshot{
		member: "attachments/screenshot" + extension,
		data:   data,
		metadata: feedbackScreenshotMetadata{
			OriginalName:   filepath.Base(cleanPath),
			MediaType:      mediaType,
			SHA256:         hex.EncodeToString(sum[:]),
			SizeBytes:      int64(len(data)),
			PixelRedaction: "screenshot pixels are not redacted; review the image before sharing",
		},
	}, nil
}

type feedbackTranscriptEntry struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	ToolName  string    `json:"tool_name,omitempty"`
}

func marshalFeedbackTranscript(entries []transcriptexport.TranscriptEntry, r *redaction.Redactor) ([]byte, error) {
	sourceEntryCount := len(entries)
	if len(entries) > maxFeedbackTranscriptEntries {
		entries = entries[len(entries)-maxFeedbackTranscriptEntries:]
	}
	out := make([]feedbackTranscriptEntry, 0, len(entries))
	for _, entry := range entries {
		content := entry.Content
		if len(content) > maxFeedbackTranscriptEntryBytes {
			content = content[:maxFeedbackTranscriptEntryBytes] + "\n[entry truncated]"
		}
		out = append(out, feedbackTranscriptEntry{
			Role:      r.Redact(entry.Role),
			Content:   r.Redact(content),
			Timestamp: entry.Timestamp.UTC(),
			ToolName:  r.Redact(entry.ToolName),
		})
	}
	data, err := json.MarshalIndent(map[string]any{
		"source_entry_count":   sourceEntryCount,
		"included_entry_count": len(out),
		"truncated":            sourceEntryCount > len(out),
		"entries":              out,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal transcript.json: %w", err)
	}
	return data, nil
}

type feedbackLogMember struct {
	name string
	data []byte
}

func defaultFeedbackServiceLogPaths() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	logDir := filepath.Join(home, ".harness", "logs")
	return map[string]string{
		"harnessd.stdout.log": filepath.Join(logDir, "harnessd.stdout.log"),
		"harnessd.stderr.log": filepath.Join(logDir, "harnessd.stderr.log"),
	}
}

func collectFeedbackLogMembers(paths map[string]string, r *redaction.Redactor) []feedbackLogMember {
	names := make([]string, 0, len(paths))
	for name := range paths {
		names = append(names, name)
	}
	sort.Strings(names)
	members := make([]feedbackLogMember, 0, len(names))
	for _, name := range names {
		path := paths[name]
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		members = append(members, feedbackLogMember{
			name: "logs/" + filepath.Base(name),
			data: redactFileTail(path, maxFeedbackLogBytes, r),
		})
	}
	return members
}

func redactFileTail(path string, maxBytes int64, r *redaction.Redactor) []byte {
	f, err := os.Open(path)
	if err != nil {
		return []byte(r.Redact("could not read file: " + err.Error()))
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return []byte(r.Redact("could not stat file: " + err.Error()))
	}
	truncated := info.Size() > maxBytes
	if truncated {
		if _, err := f.Seek(-maxBytes, io.SeekEnd); err != nil {
			return []byte(r.Redact("could not seek file: " + err.Error()))
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil {
		return []byte(r.Redact("could not read file: " + err.Error()))
	}
	text := r.Redact(string(data))
	if truncated {
		text = fmt.Sprintf("[truncated to last %d bytes]\n%s", maxBytes, text)
	}
	return []byte(text)
}

func feedbackIssueTitle(request string, cfg *harnessconfig.Config) string {
	title := newFeedbackRedactor(cfg).Redact(strings.TrimSpace(strings.Split(request, "\n")[0]))
	if title == "" {
		return "Feedback from go-code"
	}
	const maxTitleRunes = 80
	titleRunes := []rune(title)
	if len(titleRunes) > maxTitleRunes {
		title = strings.TrimSpace(string(titleRunes[:maxTitleRunes-1])) + "…"
	}
	return "Feedback: " + title
}

func writeFeedbackIssueDraft(path, bundlePath string, in feedbackInput) error {
	r := newFeedbackRedactor(in.CLIConfig)
	request := strings.TrimSpace(in.Request)
	if request == "" {
		request = "No explicit request supplied."
	}
	workspace := filepath.Base(filepath.Clean(in.Workspace))
	if strings.TrimSpace(in.Workspace) == "" || workspace == "." {
		workspace = "not configured"
	}
	screenshotLine := "- No screenshot was selected."
	if in.ScreenshotPath != "" {
		screenshotLine = fmt.Sprintf(
			"- Screenshot selected: `%s` (also inside the bundle; pixels are not redacted).",
			r.Redact(filepath.Base(in.ScreenshotPath)),
		)
	}
	body := fmt.Sprintf(`# Feedback request

%s

## Captured context

- Workspace: %s
- Model: %s
- Run ID: %s
- Conversation ID: %s
- Run active when captured: %t
- Generated: %s

## Evidence to attach before submitting

- [ ] Attach %s
%s

The bundle contains the request, active run/session identifiers, the bounded redacted transcript, runtime/config metadata, up to %d recent bounded redacted rollouts, available bounded redacted harnessd logs, and screenshot provenance.
`,
		r.Redact(request),
		r.Redact(workspace),
		r.Redact(in.Model),
		r.Redact(in.RunID),
		r.Redact(in.ConversationID),
		in.RunActive,
		in.Now.UTC().Format(time.RFC3339),
		filepath.Base(bundlePath),
		screenshotLine,
		maxFeedbackRollouts,
	)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write issue draft: %w", err)
	}
	return nil
}

type feedbackCommandRunner func(dir, name string, args ...string) error

func openFeedbackIssueDraft(workspace, title, bodyPath string, runner feedbackCommandRunner) error {
	if runner == nil {
		return fmt.Errorf("GitHub issue opener is not configured")
	}
	return runner(
		workspace,
		"gh",
		"issue", "create",
		"--repo", feedbackIssueRepository,
		"--web",
		"--title", title,
		"--body-file", bodyPath,
	)
}

func runFeedbackCommand(dir, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	if strings.TrimSpace(dir) != "" {
		command.Dir = dir
	}
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if ctx.Err() != nil {
			return fmt.Errorf("%s timed out: %w", name, ctx.Err())
		}
		if message != "" {
			return fmt.Errorf("%s: %w: %s", name, err, message)
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

type feedbackIssueDraftResultMsg struct {
	bundlePath     string
	screenshotPath string
	err            error
}

func feedbackIssueDraftCmd(workspace, title, bodyPath, bundlePath, screenshotPath string) tea.Cmd {
	return func() tea.Msg {
		return feedbackIssueDraftResultMsg{
			bundlePath:     bundlePath,
			screenshotPath: screenshotPath,
			err:            openFeedbackIssueDraft(workspace, title, bodyPath, runFeedbackCommand),
		}
	}
}
