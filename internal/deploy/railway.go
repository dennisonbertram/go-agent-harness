package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// RailwayAdapter implements Platform for Railway (https://railway.app).
// It wraps the `railway` CLI. Install with: npm install -g @railway/cli
type RailwayAdapter struct {
	exec ExecFunc
}

// NewRailwayAdapter returns a Railway adapter. Pass nil to use the real CLI.
func NewRailwayAdapter(exec ExecFunc) *RailwayAdapter {
	if exec == nil {
		exec = DefaultExec
	}
	return &RailwayAdapter{exec: exec}
}

// Name implements Platform.
func (r *RailwayAdapter) Name() string { return "railway" }

// Detect implements Platform. Returns true if railway.json or railway.toml exists.
func (r *RailwayAdapter) Detect(_ context.Context, workspaceDir string) (bool, error) {
	for _, f := range []string{"railway.json", "railway.toml"} {
		if _, err := os.Stat(workspaceDir + "/" + f); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// Deploy implements Platform. Runs `railway up` with optional environment and dry-run flags.
func (r *RailwayAdapter) Deploy(ctx context.Context, workspaceDir string, opts DeployOpts) (*DeployResult, error) {
	args := []string{"up"}
	if opts.Environment != "" {
		args = append(args, "--environment", opts.Environment)
	}
	if opts.DryRun {
		// Railway CLI does not have a native dry-run flag; we signal intent via detach.
		// Just return a preview result without executing.
		return &DeployResult{
			Platform:  r.Name(),
			Timestamp: time.Now(),
			Logs:      "[dry-run] would run: railway " + strings.Join(args, " "),
		}, nil
	}
	if opts.Force {
		args = append(args, "--no-gitignore")
	}

	out, err := r.exec(ctx, workspaceDir, "railway", args...)
	if err != nil {
		return nil, fmt.Errorf("railway deploy: %w", err)
	}

	result := &DeployResult{
		Platform:  r.Name(),
		Timestamp: time.Now(),
		Logs:      out,
	}

	// Extract URL from output (Railway prints "Deployment live at https://...")
	result.URL = extractURL(out)
	return result, nil
}

// Status implements Platform. Runs `railway status` and parses the output.
func (r *RailwayAdapter) Status(ctx context.Context, workspaceDir string) (*DeployStatus, error) {
	out, err := r.exec(ctx, workspaceDir, "railway", "status")
	if err != nil {
		return &DeployStatus{State: "failed"}, fmt.Errorf("railway status: %w", err)
	}
	return parseRailwayStatus(out), nil
}

// Logs implements Platform. Returns an io.Reader with deployment logs.
func (r *RailwayAdapter) Logs(ctx context.Context, workspaceDir string, follow bool) (io.Reader, error) {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	out, err := r.exec(ctx, workspaceDir, "railway", args...)
	if err != nil {
		return nil, fmt.Errorf("railway logs: %w", err)
	}
	return strings.NewReader(out), nil
}

// Rollback implements Platform. Railway does not expose a direct rollback CLI command.
func (r *RailwayAdapter) Rollback(_ context.Context, _ string, _ string) error {
	return ErrNotImplemented
}

// Teardown implements Platform. Runs `railway down` to remove the deployment.
func (r *RailwayAdapter) Teardown(ctx context.Context, workspaceDir string) error {
	_, err := r.exec(ctx, workspaceDir, "railway", "down")
	if err != nil {
		return fmt.Errorf("railway teardown: %w", err)
	}
	return nil
}

// parseRailwayStatus converts `railway status` text output into a DeployStatus.
func parseRailwayStatus(out string) *DeployStatus {
	status := &DeployStatus{
		State:     "unknown",
		UpdatedAt: time.Now(),
	}
	lower := strings.ToLower(out)
	switch {
	case strings.Contains(lower, "success") || strings.Contains(lower, "live") || strings.Contains(lower, "running"):
		status.State = "running"
	case strings.Contains(lower, "building") || strings.Contains(lower, "deploying"):
		status.State = "building"
	case strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(lower, "crash"):
		status.State = "failed"
	case strings.Contains(lower, "sleeping") || strings.Contains(lower, "stopped"):
		status.State = "sleeping"
	}
	status.URL = extractURL(out)
	return status
}

// extractURL finds the first https:// URL in a block of text.
func extractURL(text string) string {
	for _, word := range strings.Fields(text) {
		if strings.HasPrefix(word, "https://") || strings.HasPrefix(word, "http://") {
			// Strip trailing punctuation and anything after an embedded comma or colon
			// that would not be a valid URL suffix.
			word = strings.TrimRight(word, ".,;:)")
			// If the word contains a comma (e.g., "https://a.fly.dev,extra"), take only the URL part.
			if idx := strings.Index(word, ","); idx >= 0 {
				word = word[:idx]
			}
			return word
		}
	}
	return ""
}
