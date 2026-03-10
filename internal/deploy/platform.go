// Package deploy provides a platform-agnostic interface for deploying projects
// to various cloud providers (Cloudflare, Vercel, Railway, Fly.io, etc.).
// Each adapter wraps the provider's CLI tool using injected command execution,
// making adapters fully testable without requiring CLIs to be installed.
package deploy

import (
	"context"
	"io"
	"time"
)

// DeployOpts holds options for a deploy operation.
type DeployOpts struct {
	// Environment is the target environment (e.g., "staging", "production").
	Environment string
	// Force skips pre-deploy checks when true.
	Force bool
	// DryRun previews what would happen without executing.
	DryRun bool
}

// DeployResult is the output from a successful deploy.
type DeployResult struct {
	// URL is the public URL of the deployment.
	URL string `json:"url"`
	// Version is the version or commit identifier.
	Version string `json:"version,omitempty"`
	// Platform is the name of the platform (e.g., "cloudflare", "railway").
	Platform string `json:"platform"`
	// Timestamp is when the deployment occurred.
	Timestamp time.Time `json:"timestamp"`
	// Logs contains build/deploy log output.
	Logs string `json:"logs,omitempty"`
}

// DeployStatus represents the current state of a deployment.
type DeployStatus struct {
	// State is the deployment state: "running", "building", "failed", "sleeping", "unknown".
	State string `json:"state"`
	// URL is the public URL if available.
	URL string `json:"url,omitempty"`
	// Version is the version or commit identifier.
	Version string `json:"version,omitempty"`
	// UpdatedAt is when the status was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// Platform abstracts deployment operations across cloud providers.
// Each implementation wraps the provider's CLI tool.
type Platform interface {
	// Name returns the platform identifier (e.g., "cloudflare", "vercel", "railway").
	Name() string

	// Detect checks if the given workspace is configured for this platform.
	// Returns true if platform config files are present (wrangler.toml, fly.toml, etc.).
	Detect(ctx context.Context, workspaceDir string) (bool, error)

	// Deploy pushes the current project to the platform.
	Deploy(ctx context.Context, workspaceDir string, opts DeployOpts) (*DeployResult, error)

	// Status checks the current deployment state.
	Status(ctx context.Context, workspaceDir string) (*DeployStatus, error)

	// Logs streams deployment logs. If follow is true, streams continuously.
	// Callers should close the reader when done.
	Logs(ctx context.Context, workspaceDir string, follow bool) (io.Reader, error)

	// Rollback reverts to a previous version. An empty version string means
	// the most recent previous version.
	Rollback(ctx context.Context, workspaceDir string, version string) error

	// Teardown removes the deployment entirely.
	Teardown(ctx context.Context, workspaceDir string) error
}

// ErrNotImplemented is returned by methods that are not yet implemented for
// a given platform adapter.
var ErrNotImplemented error = errNotImplemented("operation not implemented for this platform")

type errNotImplemented string

func (e errNotImplemented) Error() string { return string(e) }
