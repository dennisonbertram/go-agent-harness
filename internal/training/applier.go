package training

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ApplierConfig controls which findings are automatically applied.
type ApplierConfig struct {
	ConfidenceThreshold Confidence // only apply if >= this (default: CERTAIN)
	DryRun              bool
	RepoPath            string // git repo root (default: current dir)
	MinEvidenceCount    int    // default: 3
}

// ApplyResult describes what happened during an Apply call.
type ApplyResult struct {
	FindingsConsidered int
	FindingsApplied    int
	FindingsSkipped    int
	BranchName         string
	CommitHash         string
	DryRun             bool
	SkipReasons        []string
}

// Applier takes high-confidence Trainer findings and applies them to
// system prompts / tool descriptions via git branches.
type Applier struct {
	cfg   ApplierConfig
	store *Store
}

// NewApplier creates an Applier with the given config and optional store.
func NewApplier(cfg ApplierConfig, store *Store) *Applier {
	if cfg.ConfidenceThreshold == "" {
		cfg.ConfidenceThreshold = ConfidenceCertain
	}
	if cfg.MinEvidenceCount <= 0 {
		cfg.MinEvidenceCount = 3
	}
	if cfg.RepoPath == "" {
		cfg.RepoPath = "."
	}
	return &Applier{cfg: cfg, store: store}
}

// Apply applies eligible findings from a batch to prompts/descriptions.
func (a *Applier) Apply(ctx context.Context, findings []Finding) (*ApplyResult, error) {
	result := &ApplyResult{
		FindingsConsidered: len(findings),
		DryRun:             a.cfg.DryRun,
	}

	// Partition findings into applicable vs skipped.
	var applicable []Finding
	for _, f := range findings {
		ok, reason := a.canAutoApply(f)
		if !ok {
			result.FindingsSkipped++
			result.SkipReasons = append(result.SkipReasons, reason)
			continue
		}
		applicable = append(applicable, f)
	}

	if len(applicable) == 0 {
		return result, nil
	}

	if a.cfg.DryRun {
		result.FindingsApplied = len(applicable)
		return result, nil
	}

	applyFn := func() error {
		for _, f := range applicable {
			if err := a.applyFinding(f); err != nil {
				return fmt.Errorf("apply finding (target=%s): %w", f.Target, err)
			}
			result.FindingsApplied++
		}
		return nil
	}

	if err := a.gitBranch(ctx, result, applyFn); err != nil {
		return result, err
	}

	// Record applied changes to store if available.
	if a.store != nil && result.CommitHash != "" {
		for _, f := range applicable {
			desc := fmt.Sprintf("type=%s target=%s issue=%s", f.Type, f.Target, f.Issue)
			// Best-effort: don't fail the whole operation if store write fails.
			_ = a.store.SaveAppliedChange(result.CommitHash, 0, desc)
		}
	}

	return result, nil
}

// canAutoApply returns true if a finding meets the auto-apply threshold.
func (a *Applier) canAutoApply(f Finding) (bool, string) {
	// Confidence must be CERTAIN.
	if f.Confidence != ConfidenceCertain {
		return false, fmt.Sprintf("confidence %s below threshold %s", f.Confidence, a.cfg.ConfidenceThreshold)
	}

	// EvidenceCount must meet minimum.
	if f.EvidenceCount < a.cfg.MinEvidenceCount {
		return false, fmt.Sprintf("evidence count %d below minimum %d", f.EvidenceCount, a.cfg.MinEvidenceCount)
	}

	// Critical priority requires human review.
	if f.Priority == "critical" {
		return false, "critical priority requires human review"
	}

	// Only system_prompt and tool_description types can be auto-applied.
	if f.Type != "system_prompt" && f.Type != "tool_description" {
		return false, fmt.Sprintf("type %q not eligible for auto-apply", f.Type)
	}

	return true, ""
}

// applyFinding applies a single finding to the appropriate file.
func (a *Applier) applyFinding(f Finding) error {
	switch f.Type {
	case "system_prompt":
		return a.applySystemPrompt(f)
	case "tool_description":
		return a.applyToolDescription(f)
	default:
		return fmt.Errorf("unsupported finding type: %s", f.Type)
	}
}

// applySystemPrompt creates a new behavior file in prompts/behaviors/.
func (a *Applier) applySystemPrompt(f Finding) error {
	behaviorsDir := filepath.Join(a.cfg.RepoPath, "prompts", "behaviors")
	if err := os.MkdirAll(behaviorsDir, 0o755); err != nil {
		return fmt.Errorf("create behaviors dir: %w", err)
	}

	sanitized := sanitizeTarget(f.Target)
	ts := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("training-%s-%s.md", sanitized, ts)
	path := filepath.Join(behaviorsDir, filename)

	content := fmt.Sprintf("# %s\n\n<!-- training: %s -->\n\n%s\n", f.Target, ts, f.Proposed)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write behavior file: %w", err)
	}
	return nil
}

// applyToolDescription appends to an existing tool description file.
func (a *Applier) applyToolDescription(f Finding) error {
	descPath := filepath.Join(a.cfg.RepoPath, "internal", "harness", "tools", "descriptions", f.Target+".md")
	if _, err := os.Stat(descPath); os.IsNotExist(err) {
		return fmt.Errorf("tool description file not found: %s", descPath)
	}

	existing, err := os.ReadFile(descPath)
	if err != nil {
		return fmt.Errorf("read tool description: %w", err)
	}

	ts := time.Now().UTC().Format("20060102-150405")
	marker := fmt.Sprintf("\n\n<!-- training: %s -->\n%s\n", ts, f.Proposed)
	updated := string(existing) + marker

	if err := os.WriteFile(descPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write tool description: %w", err)
	}
	return nil
}

// gitBranch creates a training branch, applies changes via applyFn, commits.
func (a *Applier) gitBranch(ctx context.Context, result *ApplyResult, applyFn func() error) error {
	ts := time.Now().UTC().Format("2006-01-02")
	hash := shortHash()
	branchName := fmt.Sprintf("training/auto-%s-%s", ts, hash)

	// Create and checkout the branch.
	if _, err := a.runGit("checkout", "-b", branchName); err != nil {
		return fmt.Errorf("create branch: %w", err)
	}

	// Apply changes.
	if err := applyFn(); err != nil {
		// Attempt to go back to the previous branch.
		_, _ = a.runGit("checkout", "-")
		_, _ = a.runGit("branch", "-D", branchName)
		return err
	}

	// Stage all changes.
	if _, err := a.runGit("add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Commit.
	msg := fmt.Sprintf("training: auto-apply %d findings", result.FindingsApplied)
	if _, err := a.runGit("commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Get commit hash.
	commitHash, err := a.runGit("rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("get commit hash: %w", err)
	}

	result.BranchName = branchName
	result.CommitHash = strings.TrimSpace(commitHash)

	// Switch back to the previous branch.
	_, _ = a.runGit("checkout", "-")

	return nil
}

// runGit runs a git command in the repo path.
func (a *Applier) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = a.cfg.RepoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// sanitizeTarget replaces non-alphanumeric chars with hyphens.
func sanitizeTarget(target string) string {
	var b strings.Builder
	for _, r := range target {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// shortHash returns a short hex hash for branch uniqueness.
func shortHash() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return fmt.Sprintf("%x", h[:4])
}
