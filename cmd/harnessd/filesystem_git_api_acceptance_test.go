package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/acceptance/apisserunner"
	"go-agent-harness/internal/fakeprovider"
	"go-agent-harness/internal/harness"
)

// TestIssue1231FilesystemAndGitToolsUseOneDurableConversation is deliberately
// acceptance-level: it must cross a production-composed default registry and
// a real HTTP/SSE daemon.  A tool event or narration does not pass this test;
// the driver verifies raw event ordering and each independent fixture probe.
func TestIssue1231FilesystemAndGitToolsUseOneDurableConversation(t *testing.T) {
	runFilesystemGitAPISSEAcceptance(t)
}

func TestFilesystemGitArtifactsPersistInConfiguredPrivateRoot(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(filesystemGitArtifactRootEnv, parent)
	bundle := newFilesystemGitArtifactBundle(t)
	if filepath.Dir(bundle.root) != parent {
		t.Fatalf("retained artifact root parent=%q want %q", filepath.Dir(bundle.root), parent)
	}
	info, err := os.Stat(bundle.root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("retained artifact root mode=%#o want 0700", info.Mode().Perm())
	}
	raw := []byte(`{"retained":true}`)
	bundle.retain(t, "raw.json", raw)
	fixture := filepath.Join(t.TempDir(), "disposable-fixture")
	if err := os.Mkdir(fixture, 0o700); err != nil {
		t.Fatal(err)
	}
	bundle.retainFixtureCleanup(t, fixture)
	bundle.finish(t)
	manifestPath := filepath.Join(bundle.root, "manifest.json")
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest did not persist after finish: %v", err)
	}
	var manifest struct {
		Artifacts []struct {
			Path   string `json:"path"`
			Digest string `json:"digest"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256(raw)
	gotDigests := make(map[string]string, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		gotDigests[artifact.Path] = artifact.Digest
	}
	if gotDigests["raw.json"] != fmt.Sprintf("sha256:%x", wantDigest[:]) {
		t.Fatalf("retained raw artifact digest=%q", gotDigests["raw.json"])
	}
	if gotDigests["fixture-cleanup.json"] == "" {
		t.Fatal("manifest omitted retained fixture cleanup evidence")
	}
}

func TestFilesystemGitToolCallLifecycleValidator(t *testing.T) {
	write := filesystemGitExpectedCall{"write", `{"path":"notes.txt","content":"marker=written\nphase=one\n"}`, "bytes_written"}
	read := filesystemGitExpectedCall{"read", `{"path":"notes.txt"}`, "marker=written"}
	ls := filesystemGitExpectedCall{"ls", `{"path":"."}`, "notes.txt"}
	tests := []struct {
		name     string
		runID    string
		events   []harness.Event
		expected []filesystemGitExpectedCall
		wantErr  string
	}{
		{
			name:  "rejects completion before matching start",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-1", 1, harness.EventToolCallCompleted, map[string]any{"call_id": "call-write", "tool": "write", "output": "bytes_written"}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-1", 3, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
			wantErr:  "orphan completion",
		},
		{
			name:  "rejects orphan completion",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-1", 1, harness.EventToolCallCompleted, map[string]any{"call_id": "missing", "tool": "write", "output": "bytes_written"}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
			wantErr:  "orphan completion",
		},
		{
			name:  "rejects duplicate start in one run",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-1", 1, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-1", 3, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
			wantErr:  "duplicate start",
		},
		{
			name:  "rejects duplicate completion in one run",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-1", 1, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventToolCallCompleted, map[string]any{"call_id": "call-write", "tool": "write", "output": "bytes_written"}),
				filesystemGitLifecycleEvent("run-1", 3, harness.EventToolCallCompleted, map[string]any{"call_id": "call-write", "tool": "write", "output": "bytes_written"}),
				filesystemGitLifecycleEvent("run-1", 4, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
			wantErr:  "duplicate completion",
		},
		{
			name:  "rejects completion tool mismatch",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-1", 1, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventToolCallCompleted, map[string]any{"call_id": "call-write", "tool": "read", "output": "bytes_written"}),
				filesystemGitLifecycleEvent("run-1", 3, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
			wantErr:  "tool mismatch",
		},
		{
			name:  "rejects wrong run frame",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-2", 1, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
			wantErr:  "wrong run",
		},
		{
			name:  "rejects unfinished start",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-1", 1, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
			wantErr:  "unfinished",
		},
		{
			name:  "permits valid concurrent interleaving",
			runID: "run-1",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-1", 1, harness.EventToolCallStarted, map[string]any{"call_id": "call-read", "tool": "read", "arguments": read.arguments}),
				filesystemGitLifecycleEvent("run-1", 2, harness.EventToolCallStarted, map[string]any{"call_id": "call-ls", "tool": "ls", "arguments": ls.arguments}),
				filesystemGitLifecycleEvent("run-1", 3, harness.EventToolCallCompleted, map[string]any{"call_id": "call-ls", "tool": "ls", "output": "notes.txt"}),
				filesystemGitLifecycleEvent("run-1", 4, harness.EventToolCallCompleted, map[string]any{"call_id": "call-read", "tool": "read", "output": "marker=written"}),
				filesystemGitLifecycleEvent("run-1", 5, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{read, ls},
		},
		{
			name:  "permits call ID reuse in a separate run",
			runID: "run-2",
			events: filesystemGitLifecycleEvents(
				filesystemGitLifecycleEvent("run-2", 1, harness.EventToolCallStarted, map[string]any{"call_id": "call-write", "tool": "write", "arguments": write.arguments}),
				filesystemGitLifecycleEvent("run-2", 2, harness.EventToolCallCompleted, map[string]any{"call_id": "call-write", "tool": "write", "output": "bytes_written"}),
				filesystemGitLifecycleEvent("run-2", 3, harness.EventRunCompleted, nil),
			),
			expected: []filesystemGitExpectedCall{write},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := filesystemGitLifecycleRawSSE(t, test.events)
			events := decodeFilesystemGitSSE(t, raw)
			err := validateFilesystemGitToolCallLifecycle(test.runID, events, test.expected)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validate valid lifecycle: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validate error=%v want substring %q", err, test.wantErr)
			}
		})
	}
}

func filesystemGitLifecycleEvent(runID string, sequence int, eventType harness.EventType, payload map[string]any) harness.Event {
	return harness.Event{ID: fmt.Sprintf("%s:%d", runID, sequence), RunID: runID, Type: eventType, Payload: payload}
}

func filesystemGitLifecycleEvents(events ...harness.Event) []harness.Event { return events }

func filesystemGitLifecycleRawSSE(t *testing.T, events []harness.Event) string {
	t.Helper()
	var raw strings.Builder
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&raw, "id: %s\ndata: %s\n\n", event.ID, encoded)
	}
	return raw.String()
}

type filesystemGitExpectedCall struct {
	name       string
	arguments  string
	resultPart string
}

const filesystemGitArtifactRootEnv = "HARNESS_ISSUE_1231_ARTIFACT_ROOT"

// runFilesystemGitAPISSEAcceptance is the reusable default-registry acceptance
// driver for this bounded fixture family. It deliberately starts only one
// daemon and one durable conversation: later tool calls cannot pass by reading
// state from a fresh conversation or by relying on agent narration.
func runFilesystemGitAPISSEAcceptance(t *testing.T) {
	t.Helper()
	artifacts := newFilesystemGitArtifactBundle(t)
	defer artifacts.finish(t)
	repo := newFilesystemGitFixture(t)
	defer artifacts.retainFixtureCleanup(t, repo)
	artifacts.retainFixtureProbe(t, "fixture-initial-git.json", repo, []string{"status", "--porcelain=v1"})
	env := baseEnv("127.0.0.1:0")
	env["HARNESS_WORKSPACE"] = repo
	disableCallbacksForUnrelatedHarnessFixture(env)

	provider := fakeprovider.New([]fakeprovider.Turn{
		{ToolCalls: []harness.ToolCall{{ID: "fs-write", Name: "write", Arguments: `{"path":"notes.txt","content":"marker=written\nphase=one\n"}`}}},
		{Content: "wrote the marker"},
		{ToolCalls: []harness.ToolCall{
			{ID: "fs-read", Name: "read", Arguments: `{"path":"notes.txt"}`},
			{ID: "fs-ls", Name: "ls", Arguments: `{"path":"."}`},
			{ID: "fs-glob", Name: "glob", Arguments: `{"pattern":"*.txt"}`},
			{ID: "fs-grep", Name: "grep", Arguments: `{"query":"marker=written","path":"notes.txt","literal_text":true}`},
			{ID: "fs-inspect", Name: "file_inspect", Arguments: `{"path":"notes.txt","preview_lines":2}`},
		}}, {Content: "confirmed the marker"},
		{ToolCalls: []harness.ToolCall{
			{ID: "fs-edit", Name: "edit", Arguments: `{"path":"notes.txt","old_text":"phase=one","new_text":"phase=two"}`},
			{ID: "fs-patch", Name: "apply_patch", Arguments: `{"path":"notes.txt","find":"marker=written","replace":"marker=patched"}`},
		}}, {Content: "edited and patched the marker"},
		{ToolCalls: []harness.ToolCall{
			{ID: "git-status", Name: "git_status", Arguments: `{}`},
			{ID: "git-diff", Name: "git_diff", Arguments: `{"path":"notes.txt"}`},
			{ID: "git-range", Name: "git_diff_range", Arguments: `{"from":"HEAD~1","to":"HEAD","path":"notes.txt"}`},
			{ID: "git-log", Name: "git_log_search", Arguments: `{"query":"fixture baseline","mode":"message","path":"notes.txt"}`},
			{ID: "git-history", Name: "git_file_history", Arguments: `{"path":"notes.txt","max_commits":10}`},
			{ID: "git-blame", Name: "git_blame_context", Arguments: `{"path":"notes.txt","start_line":1,"end_line":1}`},
			{ID: "git-contributors", Name: "git_contributor_context", Arguments: `{"path":"notes.txt","max_authors":5}`},
		}}, {Content: "verified the fixture history"},
	})

	runHarnessdProfileAcceptance(t, env, provider, func(baseURL string) {
		inventoryHash := assertFilesystemGitInventory(t, baseURL, artifacts)
		first := startProfileAcceptanceRun(t, baseURL, "write the acceptance marker")
		awaitRunTerminalState(t, baseURL, first, 5*time.Second)
		firstState := artifacts.retainRunState(t, baseURL, first, "run-1.json")
		conversation := requiredCompletedConversation(t, firstState, "write", "wrote the marker")
		firstRaw := profileAcceptanceRunEvents(t, baseURL, first)
		assertFilesystemGitToolCalls(t, artifacts, first, "run-1.sse", firstRaw, []filesystemGitExpectedCall{{"write", `{"path":"notes.txt","content":"marker=written\nphase=one\n"}`, "bytes_written"}})
		assertFixtureText(t, repo, "marker=written\nphase=one\n")
		artifacts.retainFixtureProbe(t, "fixture-after-write.json", repo, []string{"diff", "--", "notes.txt"})

		second := continueProfileAcceptanceRun(t, baseURL, first, "read, list, glob, grep, and inspect the marker")
		awaitRunTerminalState(t, baseURL, second, 5*time.Second)
		secondState := artifacts.retainRunState(t, baseURL, second, "run-2.json")
		assertSameCompletedConversation(t, conversation, secondState, "inspection", "confirmed the marker")
		secondRaw := profileAcceptanceRunEvents(t, baseURL, second)
		assertFilesystemGitToolCalls(t, artifacts, second, "run-2.sse", secondRaw, []filesystemGitExpectedCall{
			{"read", `{"path":"notes.txt"}`, "marker=written"},
			{"ls", `{"path":"."}`, "notes.txt"},
			{"glob", `{"pattern":"*.txt"}`, "notes.txt"},
			{"grep", `{"query":"marker=written","path":"notes.txt","literal_text":true}`, "marker=written"},
			{"file_inspect", `{"path":"notes.txt","preview_lines":2}`, "marker=written"},
		})
		assertFixtureText(t, repo, "marker=written\nphase=one\n")

		third := continueProfileAcceptanceRun(t, baseURL, second, "edit then patch the marker")
		awaitRunTerminalState(t, baseURL, third, 5*time.Second)
		thirdState := artifacts.retainRunState(t, baseURL, third, "run-3.json")
		assertSameCompletedConversation(t, conversation, thirdState, "mutation", "edited and patched the marker")
		thirdRaw := profileAcceptanceRunEvents(t, baseURL, third)
		assertFilesystemGitToolCalls(t, artifacts, third, "run-3.sse", thirdRaw, []filesystemGitExpectedCall{
			{"edit", `{"path":"notes.txt","old_text":"phase=one","new_text":"phase=two"}`, "replacements"},
			{"apply_patch", `{"path":"notes.txt","find":"marker=written","replace":"marker=patched"}`, "replacements"},
		})
		assertFixtureText(t, repo, "marker=patched\nphase=two\n")

		fourth := continueProfileAcceptanceRun(t, baseURL, third, "verify the fixture Git status, diff, history, blame, and contributors")
		awaitRunTerminalState(t, baseURL, fourth, 5*time.Second)
		fourthState := artifacts.retainRunState(t, baseURL, fourth, "run-4.json")
		assertSameCompletedConversation(t, conversation, fourthState, "Git", "verified the fixture history")
		fourthRaw := profileAcceptanceRunEvents(t, baseURL, fourth)
		assertFilesystemGitToolCalls(t, artifacts, fourth, "run-4.sse", fourthRaw, []filesystemGitExpectedCall{
			{"git_status", `{}`, "notes.txt"},
			{"git_diff", `{"path":"notes.txt"}`, "marker=patched"},
			{"git_diff_range", `{"from":"HEAD~1","to":"HEAD","path":"notes.txt"}`, "marker=baseline"},
			{"git_log_search", `{"query":"fixture baseline","mode":"message","path":"notes.txt"}`, "fixture baseline"},
			{"git_file_history", `{"path":"notes.txt","max_commits":10}`, "fixture baseline"},
			{"git_blame_context", `{"path":"notes.txt","start_line":1,"end_line":1}`, "fixture baseline"},
			{"git_contributor_context", `{"path":"notes.txt","max_authors":5}`, "Acceptance Fixture"},
		})
		assertFixtureText(t, repo, "marker=patched\nphase=two\n")
		assertConversationStoreEvidence(t, artifacts, baseURL, conversation, []string{"wrote the marker", "confirmed the marker", "edited and patched the marker", "verified the fixture history"})
		assertFilesystemGitExternalProbes(t, artifacts, repo)
		artifacts.retainJSON(t, "evidence-binding.json", map[string]any{"inventory_hash": inventoryHash, "conversation_id": conversation, "runs": []string{first, second, third, fourth}})
	})
}

// filesystemGitArtifactBundle keeps raw API, SSE, and external-probe evidence
// separate from assertions. Every retained payload is content-addressed in the
// final manifest, so a future fixture change cannot turn a prior log path into
// unbound evidence.
type filesystemGitArtifactBundle struct {
	root       string
	configured string
	digests    map[string]string
}

func newFilesystemGitArtifactBundle(t *testing.T) *filesystemGitArtifactBundle {
	t.Helper()
	parent := strings.TrimSpace(os.Getenv(filesystemGitArtifactRootEnv))
	if parent == "" {
		parent = filepath.Join(os.TempDir(), "go-agent-harness-issue-1231-artifacts")
	}
	if !filepath.IsAbs(parent) {
		t.Fatalf("retained artifact parent must be absolute: %q", parent)
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatalf("create retained artifact parent %q: %v", parent, err)
	}
	parentInfo, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat retained artifact parent %q: %v", parent, err)
	}
	if !parentInfo.IsDir() || parentInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("retained artifact parent %q must be a private directory, mode=%#o", parent, parentInfo.Mode().Perm())
	}
	root, err := os.MkdirTemp(parent, "run-")
	if err != nil {
		t.Fatalf("create retained artifact root in %q: %v", parent, err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatalf("make retained artifact root private: %v", err)
	}
	return &filesystemGitArtifactBundle{root: root, configured: parent, digests: make(map[string]string)}
}

func (b *filesystemGitArtifactBundle) retain(t *testing.T, name string, data []byte) {
	t.Helper()
	if filepath.Base(name) != name || name == "" {
		t.Fatalf("artifact name must be a plain filename: %q", name)
	}
	if _, exists := b.digests[name]; exists {
		t.Fatalf("duplicate retained artifact %q", name)
	}
	if err := os.WriteFile(filepath.Join(b.root, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	b.digests[name] = fmt.Sprintf("sha256:%x", sum[:])
}

func (b *filesystemGitArtifactBundle) retainJSON(t *testing.T, name string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b.retain(t, name, append(data, '\n'))
}

func (b *filesystemGitArtifactBundle) retainRunState(t *testing.T, baseURL, runID, name string) map[string]any {
	t.Helper()
	response, err := http.Get(baseURL + "/v1/runs/" + runID)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET retained run %s status=%d body=%s", runID, response.StatusCode, data)
	}
	b.retain(t, name, data)
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func (b *filesystemGitArtifactBundle) retainFixtureProbe(t *testing.T, name, repo string, args []string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture probe git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	b.retainJSON(t, name, map[string]any{"command": append([]string{"git", "-C", repo}, args...), "output": string(output)})
}

func (b *filesystemGitArtifactBundle) retainFixtureCleanup(t *testing.T, repo string) {
	t.Helper()
	if err := os.RemoveAll(repo); err != nil {
		t.Fatalf("remove disposable fixture %q: %v", repo, err)
	}
	if _, err := os.Lstat(repo); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fixture cleanup %q err=%v want not-exist", repo, err)
	}
	b.retainJSON(t, "fixture-cleanup.json", map[string]any{"fixture_root": repo, "removed": true})
}

func (b *filesystemGitArtifactBundle) finish(t *testing.T) {
	t.Helper()
	names := make([]string, 0, len(b.digests))
	for name := range b.digests {
		names = append(names, name)
	}
	sort.Strings(names)
	manifest := make([]map[string]string, 0, len(names))
	for _, name := range names {
		manifest = append(manifest, map[string]string{"path": name, "digest": b.digests[name]})
	}
	data, err := json.MarshalIndent(map[string]any{"artifact_root": b.root, "configured_parent": b.configured, "artifacts": manifest}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.root, "manifest.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("Issue #1231 retained evidence root=%s artifacts=%d manifest_sha256=%x", b.root, len(names), sha256.Sum256(data))
}

func newFilesystemGitFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	path := filepath.Join(repo, "notes.txt")
	if err := os.WriteFile(path, []byte("marker=seed\nphase=zero\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"config", "user.name", "Acceptance Fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"add", "notes.txt"}, {"commit", "-m", "fixture seed"}} {
		runFixtureGit(t, repo, args...)
	}
	if err := os.WriteFile(path, []byte("marker=baseline\nphase=baseline\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "notes.txt"}, {"commit", "-m", "fixture baseline"}} {
		runFixtureGit(t, repo, args...)
	}
	return repo
}

func runFixtureGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}

func assertFilesystemGitInventory(t *testing.T, baseURL string, artifacts *filesystemGitArtifactBundle) string {
	t.Helper()
	response, err := http.Get(baseURL + "/v1/tools")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("GET /v1/tools status=%d body=%s", response.StatusCode, body)
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	artifacts.retain(t, "live-tools.json", raw)
	var payload struct {
		Tools []struct {
			Name      string `json:"name"`
			Owner     string `json:"owner"`
			Condition string `json:"condition"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	rawSum := sha256.Sum256(raw)
	rawHash := fmt.Sprintf("sha256:%x", rawSum[:])
	compiled, err := (apisserunner.Runner{BaseURL: baseURL}).LoadLiveInventory(context.Background())
	if err != nil {
		t.Fatalf("compile live /v1/tools inventory: %v", err)
	}
	if compiled.Hash == "" {
		t.Fatal("compiled live /v1/tools inventory omitted hash")
	}
	inventoryHash := compiled.Hash
	found := make(map[string]struct {
		owner, condition string
	}, len(payload.Tools))
	for _, tool := range payload.Tools {
		found[tool.Name] = struct{ owner, condition string }{tool.Owner, tool.Condition}
	}
	rows := make([]map[string]string, 0, 15)
	for _, name := range []string{"ls", "glob", "grep", "read", "write", "edit", "apply_patch", "file_inspect", "git_status", "git_diff", "git_diff_range", "git_log_search", "git_file_history", "git_blame_context", "git_contributor_context"} {
		metadata, exists := found[name]
		if !exists {
			t.Fatalf("live default registry omitted required tool %q", name)
		}
		if metadata.owner == "" || metadata.condition == "" {
			t.Fatalf("live tool %q has incomplete owner/condition metadata: %#v", name, metadata)
		}
		rows = append(rows, map[string]string{"name": name, "owner": metadata.owner, "condition": metadata.condition, "provenance": "GET /v1/tools compiled_inventory=" + inventoryHash + " raw=" + rawHash})
	}
	artifacts.retainJSON(t, "inventory-scoped-rows.json", map[string]any{"inventory_hash": inventoryHash, "raw_sha256": rawHash, "rows": rows})
	return inventoryHash
}

func requiredCompletedConversation(t *testing.T, state map[string]any, step, expectedReply string) string {
	t.Helper()
	if state["status"] != string(harness.RunStatusCompleted) {
		t.Fatalf("%s run status=%#v", step, state)
	}
	conversation, _ := state["conversation_id"].(string)
	if conversation == "" {
		t.Fatalf("%s run omitted conversation: %#v", step, state)
	}
	if output, _ := state["output"].(string); output != expectedReply {
		t.Fatalf("%s run assistant output=%q want %q", step, output, expectedReply)
	}
	return conversation
}

func assertSameCompletedConversation(t *testing.T, want string, state map[string]any, step, expectedReply string) {
	t.Helper()
	if got := requiredCompletedConversation(t, state, step, expectedReply); got != want {
		t.Fatalf("%s conversation=%q want %q", step, got, want)
	}
}

func assertFixtureText(t *testing.T, repo, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(repo, "notes.txt"))
	if err != nil || string(got) != want {
		t.Fatalf("fixture notes.txt=%q err=%v want=%q", got, err, want)
	}
}

func assertFilesystemGitToolCalls(t *testing.T, artifacts *filesystemGitArtifactBundle, runID, artifactName, raw string, expected []filesystemGitExpectedCall) {
	t.Helper()
	artifacts.retain(t, artifactName, []byte(raw))
	events := decodeFilesystemGitSSE(t, raw)
	if err := validateFilesystemGitToolCallLifecycle(runID, events, expected); err != nil {
		t.Fatalf("raw SSE tool lifecycle %q: %v; raw=%s", artifactName, err, raw)
	}
}

type filesystemGitCallKey struct {
	runID  string
	callID string
}

type filesystemGitPendingCall struct {
	expectedIndex int
	name          string
}

// validateFilesystemGitToolCallLifecycle validates the original raw-SSE order,
// not independently collected event classes. A call is identified only by its
// `(runID, callID)` pair, so a later run may legitimately reuse an ID while a
// duplicate or re-used ID inside one run fails closed.
func validateFilesystemGitToolCallLifecycle(runID string, events []harness.Event, expected []filesystemGitExpectedCall) error {
	if runID == "" {
		return errors.New("raw SSE first event omitted run ID")
	}
	if len(events) == 0 {
		return errors.New("raw SSE contained no events")
	}
	seenEventIDs := make(map[string]struct{}, len(events))
	pending := make(map[filesystemGitCallKey]filesystemGitPendingCall, len(expected))
	completed := make(map[filesystemGitCallKey]struct{}, len(expected))
	usedExpected := make([]bool, len(expected))
	for index, event := range events {
		if event.ID == "" {
			return fmt.Errorf("event[%d] omitted event ID", index)
		}
		if _, duplicate := seenEventIDs[event.ID]; duplicate {
			return fmt.Errorf("raw SSE reused event ID %q", event.ID)
		}
		seenEventIDs[event.ID] = struct{}{}
		if event.RunID != runID {
			return fmt.Errorf("event[%d] wrong run %q want %q", index, event.RunID, runID)
		}
		switch event.Type {
		case harness.EventToolCallStarted:
			name, callID, arguments, err := filesystemGitStartFields(event.Payload)
			if err != nil {
				return fmt.Errorf("event[%d] start: %w", index, err)
			}
			key := filesystemGitCallKey{runID: runID, callID: callID}
			if _, exists := pending[key]; exists {
				return fmt.Errorf("event[%d] duplicate start for run=%q call_id=%q", index, runID, callID)
			}
			if _, exists := completed[key]; exists {
				return fmt.Errorf("event[%d] duplicate start for completed run=%q call_id=%q", index, runID, callID)
			}
			expectedIndex := -1
			for candidate, want := range expected {
				if !usedExpected[candidate] && want.name == name && sameJSON(arguments, want.arguments) {
					expectedIndex = candidate
					break
				}
			}
			if expectedIndex < 0 {
				return fmt.Errorf("event[%d] unexpected start tool=%q arguments=%s", index, name, arguments)
			}
			usedExpected[expectedIndex] = true
			pending[key] = filesystemGitPendingCall{expectedIndex: expectedIndex, name: name}
		case harness.EventToolCallCompleted:
			name, callID, output, err := filesystemGitCompletionFields(event.Payload)
			if err != nil {
				return fmt.Errorf("event[%d] completion: %w", index, err)
			}
			key := filesystemGitCallKey{runID: runID, callID: callID}
			started, exists := pending[key]
			if !exists {
				if _, duplicate := completed[key]; duplicate {
					return fmt.Errorf("event[%d] duplicate completion for run=%q call_id=%q", index, runID, callID)
				}
				return fmt.Errorf("event[%d] orphan completion for run=%q call_id=%q", index, runID, callID)
			}
			if name != started.name {
				return fmt.Errorf("event[%d] completion tool mismatch for call_id=%q: got %q want %q", index, callID, name, started.name)
			}
			want := expected[started.expectedIndex]
			if !strings.Contains(output, want.resultPart) {
				return fmt.Errorf("event[%d] completion tool=%q output lacks %q: %s", index, name, want.resultPart, output)
			}
			delete(pending, key)
			completed[key] = struct{}{}
		}
	}
	if terminal := events[len(events)-1]; terminal.Type != harness.EventRunCompleted {
		return fmt.Errorf("raw SSE terminal event=%q want %q", terminal.Type, harness.EventRunCompleted)
	}
	if len(pending) != 0 {
		for key, started := range pending {
			return fmt.Errorf("unfinished start for run=%q call_id=%q tool=%q", key.runID, key.callID, started.name)
		}
	}
	for index, used := range usedExpected {
		if !used {
			return fmt.Errorf("expected tool invocation missing start: tool=%q arguments=%s", expected[index].name, expected[index].arguments)
		}
	}
	return nil
}

func filesystemGitStartFields(payload map[string]any) (name, callID, arguments string, err error) {
	name, _ = payload["tool"].(string)
	callID, _ = payload["call_id"].(string)
	arguments, _ = payload["arguments"].(string)
	if name == "" || callID == "" || arguments == "" {
		return "", "", "", fmt.Errorf("missing start fields tool=%q call_id=%q arguments=%q", name, callID, arguments)
	}
	return name, callID, arguments, nil
}

func filesystemGitCompletionFields(payload map[string]any) (name, callID, output string, err error) {
	name, _ = payload["tool"].(string)
	callID, _ = payload["call_id"].(string)
	output, _ = payload["output"].(string)
	if name == "" || callID == "" {
		return "", "", "", fmt.Errorf("missing completion fields tool=%q call_id=%q", name, callID)
	}
	if errValue := fmt.Sprint(payload["error"]); errValue != "<nil>" && errValue != "" {
		return "", "", "", fmt.Errorf("completion error for tool=%q call_id=%q: %s", name, callID, errValue)
	}
	return name, callID, output, nil
}

func decodeFilesystemGitSSE(t *testing.T, raw string) []harness.Event {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var events []harness.Event
	var id, data string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if data != "" {
				var event harness.Event
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					t.Fatalf("decode SSE event %q: %v", data, err)
				}
				if event.ID == "" {
					event.ID = id
				}
				if event.ID == "" {
					t.Fatal("raw SSE event omitted id")
				}
				events = append(events, event)
			}
			id, data = "", ""
			continue
		}
		if value, ok := strings.CutPrefix(line, "id: "); ok {
			id = value
		}
		if value, ok := strings.CutPrefix(line, "data: "); ok {
			data = value
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("raw SSE contained no events")
	}
	return events
}

func sameJSON(left, right string) bool {
	var a, b any
	return json.Unmarshal([]byte(left), &a) == nil && json.Unmarshal([]byte(right), &b) == nil && fmt.Sprintf("%#v", a) == fmt.Sprintf("%#v", b)
}

func assertConversationStoreEvidence(t *testing.T, artifacts *filesystemGitArtifactBundle, baseURL, conversation string, expectedReplies []string) {
	t.Helper()
	response, err := http.Get(baseURL + "/v1/conversations/" + conversation + "/messages")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("GET conversation messages status=%d body=%s", response.StatusCode, body)
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	artifacts.retain(t, "conversation-messages.json", raw)
	var payload struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	var assistants []string
	for _, message := range payload.Messages {
		if message["role"] == "assistant" {
			content, _ := message["content"].(string)
			if content != "" {
				assistants = append(assistants, content)
			}
		}
	}
	if len(assistants) != len(expectedReplies) {
		t.Fatalf("persisted assistant messages=%#v want=%#v", assistants, expectedReplies)
	}
	for i, want := range expectedReplies {
		if assistants[i] != want {
			t.Fatalf("persisted assistant[%d]=%q want %q", i, assistants[i], want)
		}
	}
}

// assertFilesystemGitExternalProbes independently asks Git about each of the
// seven Git-tool facts. It never accepts a corresponding tool result as proof.
func assertFilesystemGitExternalProbes(t *testing.T, artifacts *filesystemGitArtifactBundle, repo string) {
	t.Helper()
	probes := []struct {
		name, assertion string
		args            []string
	}{
		{"probe-git-status.json", " M notes.txt", []string{"status", "--porcelain=v1"}},
		{"probe-git-diff.json", "marker=patched", []string{"diff", "--", "notes.txt"}},
		{"probe-git-diff-range.json", "marker=baseline", []string{"diff", "HEAD~1", "HEAD", "--", "notes.txt"}},
		{"probe-git-log-search.json", "fixture baseline", []string{"log", "--format=%s", "--grep=fixture baseline", "--", "notes.txt"}},
		{"probe-git-file-history.json", "fixture baseline", []string{"log", "--format=%s", "--follow", "--", "notes.txt"}},
		{"probe-git-blame.json", "marker=baseline", []string{"blame", "HEAD", "-L", "1,1", "--", "notes.txt"}},
		{"probe-git-contributor.json", "Acceptance Fixture", []string{"log", "--format=%an <%ae>", "--", "notes.txt"}},
	}
	for _, probe := range probes {
		command := exec.Command("git", append([]string{"-C", repo}, probe.args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("external Git probe %s: %v: %s", probe.name, err, output)
		}
		if !strings.Contains(string(output), probe.assertion) {
			t.Fatalf("external Git probe %s lacks %q: %s", probe.name, probe.assertion, output)
		}
		artifacts.retainJSON(t, probe.name, map[string]any{"command": append([]string{"git", "-C", repo}, probe.args...), "assertion": probe.assertion, "output": string(output)})
	}
}
