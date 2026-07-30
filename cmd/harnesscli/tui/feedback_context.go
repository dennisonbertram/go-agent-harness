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
	"net/url"
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
	feedbackAssetReleaseTag         = "go-code-feedback-assets"
	feedbackAssetReleaseTitle       = "go-code feedback assets"
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

	result := feedbackOptions{OpenIssue: true}
	var request []string
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "--":
			request = append(request, tokens[i+1:]...)
			i = len(tokens)
		case token == "--issue":
			result.OpenIssue = true
		case token == "--local":
			result.OpenIssue = false
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

func feedbackScreenshotPaths(in feedbackInput) []string {
	paths := make([]string, 0, len(in.ScreenshotPaths)+1)
	seen := make(map[string]struct{}, len(in.ScreenshotPaths)+1)
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	add(in.ScreenshotPath)
	for _, path := range in.ScreenshotPaths {
		add(path)
	}
	return paths
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

func loadFeedbackScreenshots(paths []string) ([]*feedbackScreenshot, error) {
	screenshots := make([]*feedbackScreenshot, 0, len(paths))
	for _, path := range paths {
		screenshot, err := loadFeedbackScreenshot(path)
		if err != nil {
			return nil, err
		}
		if screenshot != nil {
			screenshots = append(screenshots, screenshot)
		}
	}
	if len(screenshots) <= 1 {
		return screenshots, nil
	}
	for i, screenshot := range screenshots {
		extension := filepath.Ext(screenshot.member)
		screenshot.member = fmt.Sprintf("attachments/screenshot-%d%s", i+1, extension)
	}
	return screenshots, nil
}

// copyFeedbackScreenshots writes durable, private sidecars beside bundlePath.
// Clipboard attachment paths are temporary and user interaction can remove
// them while the asynchronous GitHub upload is running, so publication must
// own stable copies before returning control to the TUI.
func copyFeedbackScreenshots(bundlePath string, paths []string) ([]string, error) {
	paths = feedbackScreenshotPaths(feedbackInput{ScreenshotPaths: paths})
	screenshots, err := loadFeedbackScreenshots(paths)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(bundlePath, filepath.Ext(bundlePath))
	sidecars := make([]string, 0, len(screenshots))
	cleanup := func() {
		for _, path := range sidecars {
			_ = os.Remove(path)
		}
	}
	for i, screenshot := range screenshots {
		extension := filepath.Ext(screenshot.member)
		path := fmt.Sprintf("%s-image-%d%s", base, i+1, extension)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("create feedback image copy: %w", err)
		}
		if _, err := f.Write(screenshot.data); err != nil {
			_ = f.Close()
			_ = os.Remove(path)
			cleanup()
			return nil, fmt.Errorf("write feedback image copy: %w", err)
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(path)
			cleanup()
			return nil, fmt.Errorf("close feedback image copy: %w", err)
		}
		sidecars = append(sidecars, path)
	}
	return sidecars, nil
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
	screenshotLine := "- No screenshot was captured."
	if paths := feedbackScreenshotPaths(in); len(paths) > 0 {
		lines := make([]string, 0, len(paths))
		for _, path := range paths {
			lines = append(lines, fmt.Sprintf(
				"- Captured screenshot: `%s` (also inside the bundle; pixels are not redacted).",
				r.Redact(filepath.Base(path)),
			))
		}
		screenshotLine = strings.Join(lines, "\n")
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

## Captured evidence

- Diagnostic bundle: %s
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

type feedbackCommandRunner func(dir, name string, args ...string) (string, error)

type feedbackPublishRequest struct {
	Workspace  string
	Title      string
	BodyPath   string
	BundlePath string
	ImagePaths []string
}

const (
	feedbackPublishedEvidenceStart = "<!-- go-code-feedback-upload:start -->"
	feedbackPublishedEvidenceEnd   = "<!-- go-code-feedback-upload:end -->"
)

func feedbackAssetURL(path string) string {
	return fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/%s",
		feedbackIssueRepository,
		feedbackAssetReleaseTag,
		url.PathEscape(filepath.Base(path)),
	)
}

func writeFeedbackPublishedEvidence(bodyPath, bundlePath string, imagePaths []string) error {
	data, err := os.ReadFile(bodyPath)
	if err != nil {
		return fmt.Errorf("read issue body: %w", err)
	}
	body := string(data)
	if start := strings.Index(body, feedbackPublishedEvidenceStart); start >= 0 {
		if end := strings.Index(body[start:], feedbackPublishedEvidenceEnd); end >= 0 {
			end += start + len(feedbackPublishedEvidenceEnd)
			body = strings.TrimSpace(body[:start] + body[end:])
		}
	}

	var evidence strings.Builder
	evidence.WriteString("\n\n")
	evidence.WriteString(feedbackPublishedEvidenceStart)
	evidence.WriteString("\n## Uploaded evidence\n\n")
	fmt.Fprintf(
		&evidence,
		"- [Download diagnostic bundle (`%s`)](%s)\n",
		filepath.Base(bundlePath),
		feedbackAssetURL(bundlePath),
	)
	for i, path := range imagePaths {
		fmt.Fprintf(
			&evidence,
			"\n### Feedback image %d\n\n![Feedback image %d](%s)\n",
			i+1,
			i+1,
			feedbackAssetURL(path),
		)
	}
	evidence.WriteString(feedbackPublishedEvidenceEnd)
	evidence.WriteString("\n")
	if err := os.WriteFile(bodyPath, []byte(strings.TrimSpace(body)+evidence.String()), 0o600); err != nil {
		return fmt.Errorf("write published issue body: %w", err)
	}
	return nil
}

func ensureFeedbackAssetRelease(workspace string, runner feedbackCommandRunner) error {
	if _, err := runner(
		workspace,
		"gh",
		"release", "view", feedbackAssetReleaseTag,
		"--repo", feedbackIssueRepository,
		"--json", "tagName",
	); err == nil {
		return nil
	}

	if _, err := runner(
		workspace,
		"gh",
		"release", "create", feedbackAssetReleaseTag,
		"--repo", feedbackIssueRepository,
		"--title", feedbackAssetReleaseTitle,
		"--notes", "Binary evidence uploaded automatically by the go-code /feedback command.",
		"--prerelease",
		"--target", "main",
	); err == nil {
		return nil
	}

	// A concurrent first capture may have created the release after our view
	// failed. One final read makes provisioning idempotent without hiding a real
	// auth/network failure.
	if _, err := runner(
		workspace,
		"gh",
		"release", "view", feedbackAssetReleaseTag,
		"--repo", feedbackIssueRepository,
		"--json", "tagName",
	); err != nil {
		return fmt.Errorf("ensure GitHub feedback asset release: %w", err)
	}
	return nil
}

func publishFeedbackIssue(request feedbackPublishRequest, runner feedbackCommandRunner) (string, error) {
	if runner == nil {
		return "", fmt.Errorf("GitHub feedback publisher is not configured")
	}
	if err := ensureFeedbackAssetRelease(request.Workspace, runner); err != nil {
		return "", err
	}

	uploadArgs := []string{
		"release", "upload", feedbackAssetReleaseTag,
		"--repo", feedbackIssueRepository,
	}
	uploadArgs = append(uploadArgs, request.BundlePath)
	uploadArgs = append(uploadArgs, request.ImagePaths...)
	if _, err := runner(request.Workspace, "gh", uploadArgs...); err != nil {
		return "", fmt.Errorf("upload feedback evidence: %w", err)
	}

	if err := writeFeedbackPublishedEvidence(request.BodyPath, request.BundlePath, request.ImagePaths); err != nil {
		return "", err
	}
	output, err := runner(
		request.Workspace,
		"gh",
		"issue", "create",
		"--repo", feedbackIssueRepository,
		"--title", request.Title,
		"--body-file", request.BodyPath,
	)
	if err != nil {
		return "", fmt.Errorf("create feedback issue: %w", err)
	}
	issueURL := strings.TrimSpace(output)
	if issueURL == "" {
		return "", fmt.Errorf("create feedback issue: gh returned no issue URL")
	}
	return issueURL, nil
}

func runFeedbackCommand(dir, name string, args ...string) (string, error) {
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
			return "", fmt.Errorf("%s timed out: %w", name, ctx.Err())
		}
		if message != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, message)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return string(output), nil
}

type feedbackIssuePublishResultMsg struct {
	bundlePath              string
	imagePaths              []string
	capturedAttachmentPaths []string
	issueURL                string
	err                     error
}

func feedbackIssuePublishCmd(request feedbackPublishRequest, capturedAttachmentPaths []string) tea.Cmd {
	capturedPaths := append([]string{}, capturedAttachmentPaths...)
	imagePaths := append([]string{}, request.ImagePaths...)
	request.ImagePaths = imagePaths
	return func() tea.Msg {
		issueURL, err := publishFeedbackIssue(request, runFeedbackCommand)
		return feedbackIssuePublishResultMsg{
			bundlePath:              request.BundlePath,
			imagePaths:              imagePaths,
			capturedAttachmentPaths: capturedPaths,
			issueURL:                issueURL,
			err:                     err,
		}
	}
}
