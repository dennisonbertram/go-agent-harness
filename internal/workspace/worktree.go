package workspace

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultHarnessURL = "http://localhost:8080"

// sanitizeBranchRe matches any character that is not alphanumeric, dot, underscore, or hyphen.
var sanitizeBranchRe = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// sanitizeBranch replaces all characters not in [A-Za-z0-9._-] with '-'.
// If the result is empty, it returns "workspace".
func sanitizeBranch(id string) string {
	result := sanitizeBranchRe.ReplaceAllString(id, "-")
	if result == "" {
		return "workspace"
	}
	return result
}

// WorktreeWorkspace implements Workspace using git worktrees.
// Each workspace gets its own branch checked out in a separate directory,
// enabling parallel work on the same repo without conflicts.
type WorktreeWorkspace struct {
	harnessURL string
	repoPath   string // path to the git repo
	id         string
	branch     string // sanitized branch name, set after Provision
	path       string // worktree path, set after Provision
}

// NewWorktree creates a new unprovisioned WorktreeWorkspace.
// harnessURL is the HTTP endpoint of the harnessd instance; if empty, the
// default "http://localhost:8080" is used. repoPath is the path to the git
// repository; if empty it will be derived from opts.BaseDir during Provision.
func NewWorktree(harnessURL, repoPath string) *WorktreeWorkspace {
	if harnessURL == "" {
		harnessURL = defaultHarnessURL
	}
	return &WorktreeWorkspace{
		harnessURL: harnessURL,
		repoPath:   repoPath,
	}
}

// Provision sets up the git worktree for this workspace.
// It creates a new branch derived from opts.ID and checks it out into a
// subdirectory under <repoPath>/worktrees/.
func (w *WorktreeWorkspace) Provision(ctx context.Context, opts Options) error {
	if opts.ID == "" {
		return ErrInvalidID
	}

	w.id = opts.ID

	// Resolve harnessURL from environment if provided.
	if u, ok := opts.Env["HARNESS_URL"]; ok && u != "" {
		w.harnessURL = u
	}
	if w.harnessURL == "" {
		w.harnessURL = defaultHarnessURL
	}

	// Resolve repoPath: prefer opts.BaseDir, fall back to existing repoPath.
	if opts.BaseDir != "" {
		w.repoPath = opts.BaseDir
	}
	if w.repoPath == "" {
		return fmt.Errorf("workspace: repoPath must be set (via opts.BaseDir or NewWorktree)")
	}

	// Compute branch and worktree path.
	sanitized := sanitizeBranch(opts.ID)
	w.branch = "workspace-" + sanitized
	w.path = filepath.Join(w.repoPath, "worktrees", sanitized)

	// Containment check: prevent path traversal attacks.
	// filepath.Join cleans the path, so ".." in the ID gets collapsed.
	// We verify the resolved path still sits under repoPath.
	absRepo, err := filepath.Abs(w.repoPath)
	if err != nil {
		return fmt.Errorf("workspace: resolving repoPath: %w", err)
	}
	absPath, err := filepath.Abs(w.path)
	if err != nil {
		return fmt.Errorf("workspace: resolving worktree path: %w", err)
	}
	if !strings.HasPrefix(absPath, absRepo+string(filepath.Separator)) {
		return fmt.Errorf("workspace: worktree path %q escapes repository root %q", absPath, absRepo)
	}

	// Create the worktree.
	cmd := exec.CommandContext(ctx, "git", "-C", w.repoPath, "worktree", "add", w.path, "-b", w.branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("workspace: git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// HarnessURL returns the HTTP endpoint of the harnessd instance for this workspace.
func (w *WorktreeWorkspace) HarnessURL() string {
	if w.harnessURL == "" {
		return defaultHarnessURL
	}
	return w.harnessURL
}

// WorkspacePath returns the filesystem path of the worktree root.
// Returns an empty string if Provision has not been called.
func (w *WorktreeWorkspace) WorkspacePath() string {
	return w.path
}

// Destroy tears down the git worktree and deletes the associated branch.
// If the workspace has not been provisioned (path is empty), Destroy is a no-op.
// Errors from "not found" conditions (already removed worktrees/branches) are
// silently ignored.
func (w *WorktreeWorkspace) Destroy(ctx context.Context) error {
	if w.path == "" {
		return nil
	}

	// Remove the worktree directory.
	rmCmd := exec.CommandContext(ctx, "git", "-C", w.repoPath, "worktree", "remove", "--force", w.path)
	if out, err := rmCmd.CombinedOutput(); err != nil {
		msg := strings.ToLower(strings.TrimSpace(string(out)))
		// Ignore errors if the worktree is already gone.
		if !strings.Contains(msg, "is not a working tree") &&
			!strings.Contains(msg, "no such file") &&
			!strings.Contains(msg, "does not exist") {
			return fmt.Errorf("workspace: git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}

	// Delete the branch.
	branchCmd := exec.CommandContext(ctx, "git", "-C", w.repoPath, "branch", "-D", w.branch)
	if out, err := branchCmd.CombinedOutput(); err != nil {
		msg := strings.ToLower(strings.TrimSpace(string(out)))
		// Ignore errors if the branch is already gone.
		if !strings.Contains(msg, "not found") &&
			!strings.Contains(msg, "error: branch") {
			return fmt.Errorf("workspace: git branch -D: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

func init() {
	// Register the "worktree" implementation in the package-level default registry.
	// Any error here means a duplicate registration, which is a programming error.
	_ = Register("worktree", func() Workspace {
		return &WorktreeWorkspace{}
	})
}
