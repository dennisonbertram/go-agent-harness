package deploy

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeExec records the most recent call for assertion.
type fakeExec struct {
	dir     string
	command string
	args    []string
	output  string
	err     error
}

func (f *fakeExec) Exec(ctx context.Context, dir, command string, args ...string) (string, error) {
	f.dir = dir
	f.command = command
	f.args = args
	return f.output, f.err
}

// TestRailwayAdapterName verifies the platform name.
func TestRailwayAdapterName(t *testing.T) {
	r := NewRailwayAdapter(nil)
	if r.Name() != "railway" {
		t.Errorf("expected 'railway', got %q", r.Name())
	}
}

// TestRailwayDetect verifies Detect returns true for railway.json.
func TestRailwayDetect(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "railway.json")
	r := NewRailwayAdapter(nil)
	ok, err := r.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for railway.json")
	}
}

// TestRailwayDetectFalse verifies Detect returns false with no railway config.
func TestRailwayDetectFalse(t *testing.T) {
	dir := t.TempDir()
	r := NewRailwayAdapter(nil)
	ok, err := r.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for empty dir")
	}
}

// TestRailwayDeployCommand verifies the correct CLI args are passed to the executor.
func TestRailwayDeployCommand(t *testing.T) {
	fe := &fakeExec{output: "Deployment live at https://myapp.railway.app"}
	r := NewRailwayAdapter(fe.Exec)
	result, err := r.Deploy(context.Background(), "/workspace", DeployOpts{Environment: "production"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fe.command != "railway" {
		t.Errorf("expected command 'railway', got %q", fe.command)
	}
	if len(fe.args) == 0 || fe.args[0] != "up" {
		t.Errorf("expected first arg 'up', got %v", fe.args)
	}
	if result.Platform != "railway" {
		t.Errorf("expected platform 'railway', got %q", result.Platform)
	}
	if result.URL != "https://myapp.railway.app" {
		t.Errorf("expected URL 'https://myapp.railway.app', got %q", result.URL)
	}
}

// TestRailwayDeployWithEnvironment verifies --environment flag is passed.
func TestRailwayDeployWithEnvironment(t *testing.T) {
	fe := &fakeExec{output: "done"}
	r := NewRailwayAdapter(fe.Exec)
	_, err := r.Deploy(context.Background(), "/workspace", DeployOpts{Environment: "staging"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := strings.Join(fe.args, " ")
	if !strings.Contains(args, "--environment") || !strings.Contains(args, "staging") {
		t.Errorf("expected --environment staging in args, got %v", fe.args)
	}
}

// TestRailwayDryRun verifies dry-run returns a preview result without calling exec.
func TestRailwayDryRun(t *testing.T) {
	called := false
	exec := func(ctx context.Context, dir, cmd string, args ...string) (string, error) {
		called = true
		return "", nil
	}
	r := NewRailwayAdapter(exec)
	result, err := r.Deploy(context.Background(), "/workspace", DeployOpts{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("dry-run should not call exec")
	}
	if !strings.Contains(result.Logs, "dry-run") {
		t.Errorf("expected 'dry-run' in logs, got %q", result.Logs)
	}
}

// TestRailwayDeployError verifies errors from the CLI are propagated.
func TestRailwayDeployError(t *testing.T) {
	fe := &fakeExec{err: errors.New("auth required")}
	r := NewRailwayAdapter(fe.Exec)
	_, err := r.Deploy(context.Background(), "/workspace", DeployOpts{})
	if err == nil {
		t.Fatal("expected error from failed exec")
	}
	if !strings.Contains(err.Error(), "railway deploy") {
		t.Errorf("expected 'railway deploy' in error, got %q", err.Error())
	}
}

// TestRailwayStatus verifies Status parses running state.
func TestRailwayStatus(t *testing.T) {
	fe := &fakeExec{output: "Service is running at https://app.railway.app"}
	r := NewRailwayAdapter(fe.Exec)
	status, err := r.Status(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != "running" {
		t.Errorf("expected state 'running', got %q", status.State)
	}
	if status.URL != "https://app.railway.app" {
		t.Errorf("expected URL 'https://app.railway.app', got %q", status.URL)
	}
}

// TestRailwayStatusBuilding verifies Status parses building state.
func TestRailwayStatusBuilding(t *testing.T) {
	fe := &fakeExec{output: "Service is deploying build in progress"}
	r := NewRailwayAdapter(fe.Exec)
	status, err := r.Status(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != "building" {
		t.Errorf("expected state 'building', got %q", status.State)
	}
}

// TestRailwayStatusFailed verifies Status parses failed state.
func TestRailwayStatusFailed(t *testing.T) {
	fe := &fakeExec{output: "Deployment error: crash looping"}
	r := NewRailwayAdapter(fe.Exec)
	status, err := r.Status(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != "failed" {
		t.Errorf("expected state 'failed', got %q", status.State)
	}
}

// TestRailwayStatusError verifies Status returns failed state on CLI error.
func TestRailwayStatusError(t *testing.T) {
	fe := &fakeExec{err: errors.New("not logged in")}
	r := NewRailwayAdapter(fe.Exec)
	status, err := r.Status(context.Background(), "/workspace")
	if err == nil {
		t.Fatal("expected error from failed exec")
	}
	if status.State != "failed" {
		t.Errorf("expected state 'failed' on error, got %q", status.State)
	}
}

// TestRailwayLogs verifies Logs returns an io.Reader with CLI output.
func TestRailwayLogs(t *testing.T) {
	fe := &fakeExec{output: "2024-01-01 server started\n2024-01-01 request received"}
	r := NewRailwayAdapter(fe.Exec)
	reader, err := r.Logs(context.Background(), "/workspace", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var buf strings.Builder
	tmp := make([]byte, 256)
	for {
		n, readErr := reader.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if readErr != nil {
			break
		}
	}
	if !strings.Contains(buf.String(), "server started") {
		t.Errorf("expected log content, got %q", buf.String())
	}
}

// TestRailwayRollback verifies Rollback returns ErrNotImplemented.
func TestRailwayRollback(t *testing.T) {
	r := NewRailwayAdapter(nil)
	err := r.Rollback(context.Background(), "/workspace", "")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
}

// TestRailwayTeardown verifies Teardown passes `railway down` to exec.
func TestRailwayTeardown(t *testing.T) {
	fe := &fakeExec{output: "service deleted"}
	r := NewRailwayAdapter(fe.Exec)
	err := r.Teardown(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fe.command != "railway" {
		t.Errorf("expected command 'railway', got %q", fe.command)
	}
	if len(fe.args) == 0 || fe.args[0] != "down" {
		t.Errorf("expected first arg 'down', got %v", fe.args)
	}
}

// TestRailwayTeardownError verifies Teardown propagates exec errors.
func TestRailwayTeardownError(t *testing.T) {
	fe := &fakeExec{err: errors.New("service not found")}
	r := NewRailwayAdapter(fe.Exec)
	err := r.Teardown(context.Background(), "/workspace")
	if err == nil {
		t.Fatal("expected error from failed exec")
	}
}

// TestRailwayForceFlag verifies the --no-gitignore flag is passed when Force=true.
func TestRailwayForceFlag(t *testing.T) {
	fe := &fakeExec{output: "done"}
	r := NewRailwayAdapter(fe.Exec)
	_, err := r.Deploy(context.Background(), "/workspace", DeployOpts{Force: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := strings.Join(fe.args, " ")
	if !strings.Contains(args, "--no-gitignore") {
		t.Errorf("expected --no-gitignore in args, got %v", fe.args)
	}
}

// TestParseRailwayStatusLive verifies live output maps to running.
func TestParseRailwayStatusLive(t *testing.T) {
	s := parseRailwayStatus("Deployment live at https://example.railway.app")
	if s.State != "running" {
		t.Errorf("expected 'running', got %q", s.State)
	}
}

// TestParseRailwayStatusUnknown verifies unrecognized output maps to unknown.
func TestParseRailwayStatusUnknown(t *testing.T) {
	s := parseRailwayStatus("some random output")
	if s.State != "unknown" {
		t.Errorf("expected 'unknown', got %q", s.State)
	}
}
