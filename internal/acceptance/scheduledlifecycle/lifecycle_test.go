package scheduledlifecycle

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStartProvidesOneOwnedDaemonForAPIEventsAndPTY(t *testing.T) {
	if os.Getenv("GO_WANT_SCHEDULED_LIFECYCLE_HELPER") == "1" {
		runLifecycleHelper(t)
		return
	}

	root := repositoryRoot(t)
	lifecycle, err := Start(context.Background(), Config{
		Command:      os.Args[0],
		Arguments:    []string{"-test.run=TestStartProvidesOneOwnedDaemonForAPIEventsAndPTY", "--"},
		SourceRoot:   root,
		ArtifactRoot: t.TempDir(),
		Environment:  []string{"GO_WANT_SCHEDULED_LIFECYCLE_HELPER=1"},
		Timeout:      3 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = lifecycle.Close() })

	if lifecycle.Provenance.SourceSHA == "" {
		t.Fatal("source SHA was not recorded")
	}
	if lifecycle.Provenance.CommandPath == "" || lifecycle.Provenance.CommandSHA256 == "" {
		t.Fatalf("executable identity was not recorded: %+v", lifecycle.Provenance)
	}
	provenanceBytes, err := os.ReadFile(filepath.Join(lifecycle.ArtifactRoot, "provenance.json"))
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	if !strings.Contains(string(provenanceBytes), lifecycle.Provenance.CommandSHA256) {
		t.Fatalf("provenance file does not bind executable digest: %s", provenanceBytes)
	}
	if lifecycle.PublicURL == "" || lifecycle.SSEURL("conversation-1") == "" {
		t.Fatalf("public API/SSE endpoints missing: %+v", lifecycle)
	}
	if got, want := lifecycle.PTY().BaseURL, lifecycle.PublicURL; got != want {
		t.Fatalf("PTY is not attached to lifecycle daemon: got %q, want %q", got, want)
	}
	if got, want := lifecycle.PTY().SourceSHA, lifecycle.Provenance.SourceSHA; got != want {
		t.Fatalf("PTY source identity = %q, want lifecycle source %q", got, want)
	}
	for name, path := range map[string]string{
		"workspace":     lifecycle.Resources.Workspace,
		"conversations": lifecycle.Resources.ConversationDB,
		"runs":          lifecycle.Resources.RunDB,
		"cron":          lifecycle.Resources.CronDB,
		"callbacks":     lifecycle.Resources.CallbackDB,
	} {
		if !strings.HasPrefix(path, lifecycle.ArtifactRoot+string(filepath.Separator)) {
			t.Fatalf("%s escapes owned artifact root: %q", name, path)
		}
	}

	assertHTTPStatus(t, lifecycle.PublicURL+"/healthz", http.StatusOK)
	assertHTTPStatus(t, lifecycle.SSEURL("conversation-1"), http.StatusOK)
}

func TestStartEarlyExitNotifiesAllObserversAndCloseDoesNotWait(t *testing.T) {
	if os.Getenv("GO_WANT_SCHEDULED_LIFECYCLE_HELPER") == "1" {
		runLifecycleHelper(t)
		return
	}
	lifecycle, err := Start(context.Background(), helperConfig(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := lifecycle.command.Process.Kill(); err != nil {
		t.Fatalf("kill owned child: %v", err)
	}
	select {
	case <-lifecycle.done:
	case <-time.After(time.Second):
		t.Fatal("first observer did not receive child exit")
	}
	select {
	case <-lifecycle.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second observer blocked after child exit")
	}
	var signals int
	lifecycle.signalProcessGroup = func(_ int, _ syscall.Signal) error {
		signals++
		return nil
	}
	started := time.Now()
	if err := lifecycle.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("Close waited after observed child exit: %s", elapsed)
	}
	if signals != 0 {
		t.Fatalf("Close signaled an already reaped process group %d time(s)", signals)
	}
}

func TestCloseWaitsForStubbornChildReapAfterEscalation(t *testing.T) {
	if os.Getenv("GO_WANT_SCHEDULED_LIFECYCLE_HELPER") == "1" {
		runLifecycleHelper(t)
		return
	}
	lifecycle, err := Start(context.Background(), helperConfig(t, "LIFECYCLE_IGNORE_TERM=1"))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- lifecycle.Close() }()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not complete bounded SIGKILL escalation")
	}
	select {
	case <-lifecycle.done:
	default:
		t.Fatal("Close returned before the escalated child was reaped")
	}
}

func TestStartImmediateChildExitReturnsBoundedly(t *testing.T) {
	if os.Getenv("GO_WANT_SCHEDULED_LIFECYCLE_HELPER") == "1" {
		runLifecycleHelper(t)
		return
	}
	started := time.Now()
	_, err := Start(context.Background(), Config{
		Command:      "/usr/bin/true",
		SourceRoot:   repositoryRoot(t),
		ArtifactRoot: t.TempDir(),
		Timeout:      3 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "exited before readiness") {
		t.Fatalf("Start error = %v, want immediate exit", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("Start waited after immediate child exit: %s", elapsed)
	}
}

func TestStartScrubsInheritedResourceEnvironment(t *testing.T) {
	for key, value := range map[string]string{
		"HARNESS_ADDR":            "127.0.0.1:1",
		"HARNESS_WORKSPACE":       "/parent/workspace",
		"HARNESS_CONVERSATION_DB": "/parent/conversations.db",
		"HARNESS_RUN_DB":          "/parent/runs.db",
		"CRONSD_DB_PATH":          "/parent/cron.db",
	} {
		t.Setenv(key, value)
	}
	dumpPath := filepath.Join(t.TempDir(), "environment.json")
	lifecycle, err := Start(context.Background(), helperConfig(t, "LIFECYCLE_ENV_DUMP="+dumpPath))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = lifecycle.Close() })
	raw, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatalf("read child environment: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode child environment: %v", err)
	}
	if got["HARNESS_ADDR"] != strings.TrimPrefix(lifecycle.PublicURL, "http://") {
		t.Fatalf("HARNESS_ADDR = %q, want lifecycle listener", got["HARNESS_ADDR"])
	}
	if got["HARNESS_WORKSPACE"] != lifecycle.Resources.Workspace || got["CRONSD_DB_PATH"] != lifecycle.Resources.CronDB {
		t.Fatalf("resource environment retained parent values: %+v", got)
	}
}

func TestExecutableIdentityDistinguishesCommandsWithSameSource(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first-helper")
	second := filepath.Join(t.TempDir(), "second-helper")
	if err := os.WriteFile(first, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("#!/bin/sh\necho stale\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	firstPath, firstDigest, err := executableIdentity(first)
	if err != nil {
		t.Fatalf("first identity: %v", err)
	}
	secondPath, secondDigest, err := executableIdentity(second)
	if err != nil {
		t.Fatalf("second identity: %v", err)
	}
	if firstPath == secondPath || firstDigest == secondDigest {
		t.Fatalf("distinct command artifacts share provenance identity: %q/%q %q/%q", firstPath, firstDigest, secondPath, secondDigest)
	}
}

func TestStartRejectsSourceMismatchBeforeLaunchingProcess(t *testing.T) {
	root := repositoryRoot(t)
	marker := filepath.Join(t.TempDir(), "started")
	_, err := Start(context.Background(), Config{
		Command:           os.Args[0],
		Arguments:         []string{"-test.run=TestStartProvidesOneOwnedDaemonForAPIEventsAndPTY", "--"},
		SourceRoot:        root,
		ArtifactRoot:      t.TempDir(),
		ExpectedSourceSHA: strings.Repeat("0", 40),
		Environment:       []string{"GO_WANT_SCHEDULED_LIFECYCLE_HELPER=1", "LIFECYCLE_MARKER=" + marker},
	})
	if err == nil || !strings.Contains(err.Error(), "source SHA mismatch") {
		t.Fatalf("Start error = %v, want source mismatch", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("source mismatch launched command: stat marker = %v", statErr)
	}
}

func TestStartFailsClosedWithoutTouchingUnrelatedListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()

	_, err = Start(context.Background(), Config{
		Command:      os.Args[0],
		SourceRoot:   repositoryRoot(t),
		ArtifactRoot: t.TempDir(),
		Address:      listener.Addr().String(),
	})
	if err == nil || !strings.Contains(err.Error(), "reserve listener") {
		t.Fatalf("Start error = %v, want listener reservation failure", err)
	}
	assertHTTPStatus(t, "http://"+listener.Addr().String()+"/healthz", http.StatusNoContent)
}

func runLifecycleHelper(t *testing.T) {
	if marker := os.Getenv("LIFECYCLE_MARKER"); marker != "" {
		if err := os.WriteFile(marker, []byte("started"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	file := os.NewFile(uintptr(3), "harness-listener")
	listener, err := net.FileListener(file)
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	mux := http.NewServeMux()
	if os.Getenv("LIFECYCLE_IGNORE_TERM") == "1" {
		signal.Ignore(syscall.SIGTERM)
	}
	if dumpPath := os.Getenv("LIFECYCLE_ENV_DUMP"); dumpPath != "" {
		values := map[string]string{}
		for _, key := range []string{"HARNESS_ADDR", "HARNESS_WORKSPACE", "HARNESS_CONVERSATION_DB", "HARNESS_RUN_DB", "CRONSD_DB_PATH"} {
			values[key] = os.Getenv(key)
		}
		raw, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dumpPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1/conversations/conversation-1/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: ready\ndata: {}\n\n")
	})
	if err := http.Serve(listener, mux); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		t.Fatal(err)
	}
}

func helperConfig(t *testing.T, environment ...string) Config {
	t.Helper()
	return Config{
		Command:      os.Args[0],
		Arguments:    []string{"-test.run=TestStartProvidesOneOwnedDaemonForAPIEventsAndPTY", "--"},
		SourceRoot:   repositoryRoot(t),
		ArtifactRoot: t.TempDir(),
		Environment:  append([]string{"GO_WANT_SCHEDULED_LIFECYCLE_HELPER=1"}, environment...),
		Timeout:      3 * time.Second,
	}
}

func assertHTTPStatus(t *testing.T, rawURL string, want int) {
	t.Helper()
	response, err := (&http.Client{Timeout: time.Second}).Get(rawURL)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		t.Fatalf("GET %s status = %d, want %d", rawURL, response.StatusCode, want)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
