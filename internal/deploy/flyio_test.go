package deploy

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestFlyAdapterName verifies the platform name.
func TestFlyAdapterName(t *testing.T) {
	f := NewFlyAdapter(nil)
	if f.Name() != "flyio" {
		t.Errorf("expected 'flyio', got %q", f.Name())
	}
}

// TestFlyDetect verifies Detect returns true for fly.toml.
func TestFlyDetect(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "fly.toml")
	f := NewFlyAdapter(nil)
	ok, err := f.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true for fly.toml")
	}
}

// TestFlyDetectFalse verifies Detect returns false with no fly config.
func TestFlyDetectFalse(t *testing.T) {
	dir := t.TempDir()
	f := NewFlyAdapter(nil)
	ok, err := f.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false for empty dir")
	}
}

// TestFlyDeployCommand verifies the correct CLI args are passed.
func TestFlyDeployCommand(t *testing.T) {
	fe := &fakeExec{output: "v42 deployed to https://myapp.fly.dev"}
	f := NewFlyAdapter(fe.Exec)
	result, err := f.Deploy(context.Background(), "/workspace", DeployOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fe.command != "fly" {
		t.Errorf("expected command 'fly', got %q", fe.command)
	}
	if len(fe.args) == 0 || fe.args[0] != "deploy" {
		t.Errorf("expected first arg 'deploy', got %v", fe.args)
	}
	if result.Platform != "flyio" {
		t.Errorf("expected platform 'flyio', got %q", result.Platform)
	}
}

// TestFlyDeployWithEnvironment verifies --env DEPLOY_ENV is passed.
func TestFlyDeployWithEnvironment(t *testing.T) {
	fe := &fakeExec{output: "done"}
	f := NewFlyAdapter(fe.Exec)
	_, err := f.Deploy(context.Background(), "/workspace", DeployOpts{Environment: "staging"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := strings.Join(fe.args, " ")
	if !strings.Contains(args, "--env") || !strings.Contains(args, "staging") {
		t.Errorf("expected --env DEPLOY_ENV=staging in args, got %v", fe.args)
	}
}

// TestFlyDryRun verifies dry-run does not call exec.
func TestFlyDryRun(t *testing.T) {
	called := false
	exec := func(ctx context.Context, dir, cmd string, args ...string) (string, error) {
		called = true
		return "", nil
	}
	f := NewFlyAdapter(exec)
	result, err := f.Deploy(context.Background(), "/workspace", DeployOpts{DryRun: true})
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

// TestFlyDeployError verifies errors are propagated.
func TestFlyDeployError(t *testing.T) {
	fe := &fakeExec{err: errors.New("not authenticated")}
	f := NewFlyAdapter(fe.Exec)
	_, err := f.Deploy(context.Background(), "/workspace", DeployOpts{})
	if err == nil {
		t.Fatal("expected error from failed exec")
	}
	if !strings.Contains(err.Error(), "fly deploy") {
		t.Errorf("expected 'fly deploy' in error, got %q", err.Error())
	}
}

// TestFlyStatus verifies Status parses running state.
func TestFlyStatus(t *testing.T) {
	fe := &fakeExec{output: "App 'myapp'\nStatus running\nHostname = myapp.fly.dev"}
	f := NewFlyAdapter(fe.Exec)
	status, err := f.Status(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != "running" {
		t.Errorf("expected state 'running', got %q", status.State)
	}
	if status.URL != "https://myapp.fly.dev" {
		t.Errorf("expected URL 'https://myapp.fly.dev', got %q", status.URL)
	}
}

// TestFlyStatusFailed verifies failed state is parsed.
func TestFlyStatusFailed(t *testing.T) {
	fe := &fakeExec{output: "App failed: error deploying"}
	f := NewFlyAdapter(fe.Exec)
	status, err := f.Status(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.State != "failed" {
		t.Errorf("expected state 'failed', got %q", status.State)
	}
}

// TestFlyStatusError verifies Status returns failed state on CLI error.
func TestFlyStatusError(t *testing.T) {
	fe := &fakeExec{err: errors.New("app not found")}
	f := NewFlyAdapter(fe.Exec)
	status, err := f.Status(context.Background(), "/workspace")
	if err == nil {
		t.Fatal("expected error from failed exec")
	}
	if status.State != "failed" {
		t.Errorf("expected state 'failed' on error, got %q", status.State)
	}
}

// TestFlyLogs verifies Logs returns an io.Reader with log output.
func TestFlyLogs(t *testing.T) {
	fe := &fakeExec{output: "app log line 1\napp log line 2"}
	f := NewFlyAdapter(fe.Exec)
	reader, err := f.Logs(context.Background(), "/workspace", false)
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
	if !strings.Contains(buf.String(), "app log line 1") {
		t.Errorf("expected log content, got %q", buf.String())
	}
}

// TestFlyLogsError verifies Logs propagates exec errors.
func TestFlyLogsError(t *testing.T) {
	fe := &fakeExec{err: errors.New("no logs")}
	f := NewFlyAdapter(fe.Exec)
	_, err := f.Logs(context.Background(), "/workspace", false)
	if err == nil {
		t.Fatal("expected error from failed exec")
	}
}

// TestFlyRollback verifies Rollback returns ErrNotImplemented for empty version.
func TestFlyRollback(t *testing.T) {
	f := NewFlyAdapter(nil)
	err := f.Rollback(context.Background(), "/workspace", "")
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented for empty version, got %v", err)
	}
}

// TestFlyRollbackWithVersion verifies Rollback calls fly deploy --image for a versioned rollback.
func TestFlyRollbackWithVersion(t *testing.T) {
	fe := &fakeExec{output: "rolled back"}
	f := NewFlyAdapter(fe.Exec)
	err := f.Rollback(context.Background(), "/workspace", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fe.command != "fly" {
		t.Errorf("expected command 'fly', got %q", fe.command)
	}
}

// TestFlyTeardown verifies Teardown calls `fly destroy --yes`.
func TestFlyTeardown(t *testing.T) {
	fe := &fakeExec{output: "app destroyed"}
	f := NewFlyAdapter(fe.Exec)
	err := f.Teardown(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fe.command != "fly" {
		t.Errorf("expected command 'fly', got %q", fe.command)
	}
	args := strings.Join(fe.args, " ")
	if !strings.Contains(args, "destroy") {
		t.Errorf("expected 'destroy' in args, got %v", fe.args)
	}
}

// TestFlyTeardownError verifies Teardown propagates exec errors.
func TestFlyTeardownError(t *testing.T) {
	fe := &fakeExec{err: errors.New("destroy failed")}
	f := NewFlyAdapter(fe.Exec)
	err := f.Teardown(context.Background(), "/workspace")
	if err == nil {
		t.Fatal("expected error from failed exec")
	}
}

// TestExtractFlyVersion verifies version extraction from fly output.
func TestExtractFlyVersion(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"Deploying v42 to myapp", "v42"},
		{"Release v1 created", "v1"},
		{"no version here", ""},
		{"v0 initial deploy", "v0"},
		{"", ""},
	}
	for _, tc := range tests {
		got := extractFlyVersion(tc.text)
		if got != tc.want {
			t.Errorf("extractFlyVersion(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// TestParseFlyStatusRunning verifies running state is detected.
func TestParseFlyStatusRunning(t *testing.T) {
	s := parseFlyStatus("1 desired, 1 placed, 1 healthy, 0 unhealthy [running]")
	if s.State != "running" {
		t.Errorf("expected 'running', got %q", s.State)
	}
}

// TestParseFlyStatusSleeping verifies sleeping/stopped state is detected.
func TestParseFlyStatusSleeping(t *testing.T) {
	s := parseFlyStatus("App is stopped")
	if s.State != "sleeping" {
		t.Errorf("expected 'sleeping', got %q", s.State)
	}
}

// TestParseFlyStatusUnknown verifies unknown state for unrecognized output.
func TestParseFlyStatusUnknown(t *testing.T) {
	s := parseFlyStatus("some random text")
	if s.State != "unknown" {
		t.Errorf("expected 'unknown', got %q", s.State)
	}
}

// TestConcurrentFlyDeploy verifies concurrent deploys to Fly are safe.
func TestConcurrentFlyDeploy(t *testing.T) {
	fe := &fakeExec{output: "v10 deployed"}
	f := NewFlyAdapter(fe.Exec)
	const goroutines = 10
	done := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := f.Deploy(context.Background(), "/workspace", DeployOpts{DryRun: true})
			done <- err
		}()
	}
	for i := 0; i < goroutines; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent deploy error: %v", err)
		}
	}
}
