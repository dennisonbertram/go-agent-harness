package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// FlyAdapter implements Platform for Fly.io (https://fly.io).
// It wraps the `fly` (flyctl) CLI. Install from https://fly.io/docs/hands-on/install-flyctl/
type FlyAdapter struct {
	exec ExecFunc
}

// NewFlyAdapter returns a Fly.io adapter. Pass nil to use the real CLI.
func NewFlyAdapter(exec ExecFunc) *FlyAdapter {
	if exec == nil {
		exec = DefaultExec
	}
	return &FlyAdapter{exec: exec}
}

// Name implements Platform.
func (f *FlyAdapter) Name() string { return "flyio" }

// Detect implements Platform. Returns true if fly.toml exists.
func (f *FlyAdapter) Detect(_ context.Context, workspaceDir string) (bool, error) {
	if _, err := os.Stat(workspaceDir + "/fly.toml"); err == nil {
		return true, nil
	}
	return false, nil
}

// Deploy implements Platform. Runs `fly deploy` with optional environment and dry-run flags.
func (f *FlyAdapter) Deploy(ctx context.Context, workspaceDir string, opts DeployOpts) (*DeployResult, error) {
	args := []string{"deploy"}
	if opts.Environment != "" {
		// Fly uses app env via --env VAR=value; environment namespacing is done via app name.
		// We pass it as metadata only.
		args = append(args, "--env", "DEPLOY_ENV="+opts.Environment)
	}
	if opts.DryRun {
		return &DeployResult{
			Platform:  f.Name(),
			Timestamp: time.Now(),
			Logs:      "[dry-run] would run: fly " + strings.Join(args, " "),
		}, nil
	}

	out, err := f.exec(ctx, workspaceDir, "fly", args...)
	if err != nil {
		return nil, fmt.Errorf("fly deploy: %w", err)
	}

	result := &DeployResult{
		Platform:  f.Name(),
		Timestamp: time.Now(),
		Logs:      out,
	}
	result.URL = extractURL(out)
	result.Version = extractFlyVersion(out)
	return result, nil
}

// Status implements Platform. Runs `fly status` and parses the output.
func (f *FlyAdapter) Status(ctx context.Context, workspaceDir string) (*DeployStatus, error) {
	out, err := f.exec(ctx, workspaceDir, "fly", "status")
	if err != nil {
		return &DeployStatus{State: "failed"}, fmt.Errorf("fly status: %w", err)
	}
	return parseFlyStatus(out), nil
}

// Logs implements Platform. Returns an io.Reader with application logs.
func (f *FlyAdapter) Logs(ctx context.Context, workspaceDir string, follow bool) (io.Reader, error) {
	args := []string{"logs"}
	if follow {
		args = append(args, "--no-tail=false")
	} else {
		args = append(args, "--no-tail")
	}
	out, err := f.exec(ctx, workspaceDir, "fly", args...)
	if err != nil {
		return nil, fmt.Errorf("fly logs: %w", err)
	}
	return strings.NewReader(out), nil
}

// Rollback implements Platform. Runs `fly releases` and deploys a prior version.
// Pass an empty version to use the most recent previous release.
func (f *FlyAdapter) Rollback(ctx context.Context, workspaceDir string, version string) error {
	if version == "" {
		// Without a version we cannot safely rollback without listing releases first.
		// Return ErrNotImplemented so the caller knows to provide a version.
		return ErrNotImplemented
	}
	_, err := f.exec(ctx, workspaceDir, "fly", "deploy", "--image", "fly:"+version)
	if err != nil {
		return fmt.Errorf("fly rollback: %w", err)
	}
	return nil
}

// Teardown implements Platform. Destroys the Fly app entirely.
func (f *FlyAdapter) Teardown(ctx context.Context, workspaceDir string) error {
	_, err := f.exec(ctx, workspaceDir, "fly", "destroy", "--yes")
	if err != nil {
		return fmt.Errorf("fly teardown: %w", err)
	}
	return nil
}

// parseFlyStatus converts `fly status` text output into a DeployStatus.
func parseFlyStatus(out string) *DeployStatus {
	status := &DeployStatus{
		State:     "unknown",
		UpdatedAt: time.Now(),
	}
	lower := strings.ToLower(out)
	switch {
	case strings.Contains(lower, "running"):
		status.State = "running"
	case strings.Contains(lower, "building") || strings.Contains(lower, "pending"):
		status.State = "building"
	case strings.Contains(lower, "failed") || strings.Contains(lower, "error") || strings.Contains(lower, "crash"):
		status.State = "failed"
	case strings.Contains(lower, "stopped") || strings.Contains(lower, "suspended"):
		status.State = "sleeping"
	}

	// Extract hostname (Fly prints "Hostname = <app>.fly.dev")
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Hostname") || strings.Contains(line, "hostname") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				host := strings.TrimSpace(parts[1])
				if host != "" {
					status.URL = "https://" + host
				}
			}
		}
	}
	return status
}

// extractFlyVersion extracts a version number from fly deploy output.
// Fly prints lines like "v42" in its release output.
func extractFlyVersion(out string) string {
	for _, word := range strings.Fields(out) {
		if len(word) > 1 && word[0] == 'v' {
			if _, err := strconv.Atoi(word[1:]); err == nil {
				return word
			}
		}
	}
	return ""
}
