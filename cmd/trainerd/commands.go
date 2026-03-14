package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go-agent-harness/internal/training"
)

// newScoreCmd creates the "score" subcommand.
func newScoreCmd(dbPath *string) *cobra.Command {
	var runID string
	var rolloutDir string

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Score a single run trace",
		Long:  `Loads a TraceBundle from a rollout JSONL file, computes structural scores, and prints results.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScore(runID, rolloutDir, *dbPath)
		},
	}

	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID to score (required)")
	cmd.Flags().StringVar(&rolloutDir, "rollout-dir", defaultRolloutDir(), "Directory containing rollout JSONL files")
	_ = cmd.MarkFlagRequired("run-id")

	return cmd
}

// runScore implements the score command logic.
func runScore(runID, rolloutDir, dbPath string) error {
	path, err := findRolloutFile(rolloutDir, runID)
	if err != nil {
		return fmt.Errorf("find rollout file: %w", err)
	}

	bundle, err := training.ExportFromJSONL(path)
	if err != nil {
		return fmt.Errorf("export from JSONL: %w", err)
	}

	scorer := &training.Scorer{}
	result := scorer.Score(*bundle)

	fmt.Printf("Run ID:            %s\n", result.RunID)
	fmt.Printf("Tool Quality:      %.4f\n", result.ToolQuality)
	fmt.Printf("Efficiency:        %.4f\n", result.Efficiency)
	fmt.Printf("First-Try Rate:    %.4f\n", result.FirstTryRate)
	fmt.Printf("Anti-Pattern Count: %d\n", result.AntiPatternCount)
	fmt.Printf("Max Context Ratio: %.4f\n", result.MaxContextRatio)
	fmt.Printf("Summary:           %s\n", result.Summary)

	return nil
}

// newAnalyzeCmd creates the "analyze" subcommand.
func newAnalyzeCmd(dbPath *string) *cobra.Command {
	var runIDs string
	var rolloutDir string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze run traces using Claude",
		Long:  `Loads traces, sends them to Claude for analysis, saves findings to the database, and prints results.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(runIDs, rolloutDir, outputFormat, *dbPath)
		},
	}

	cmd.Flags().StringVar(&runIDs, "run-ids", "", "Comma-separated run IDs to analyze (required)")
	cmd.Flags().StringVar(&rolloutDir, "rollout-dir", defaultRolloutDir(), "Directory containing rollout JSONL files")
	cmd.Flags().StringVar(&outputFormat, "output-format", "text", "Output format: text or json")
	_ = cmd.MarkFlagRequired("run-ids")

	return cmd
}

// runAnalyze implements the analyze command logic.
func runAnalyze(runIDsStr, rolloutDir, outputFormat, dbPath string) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable is required for analyze")
	}

	ids := strings.Split(runIDsStr, ",")
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
	}

	bundles, err := loadBundles(rolloutDir, ids)
	if err != nil {
		return err
	}

	trainer := training.NewClaudeTrainer(apiKey)
	ctx := context.Background()

	// Ensure db directory exists.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	store, err := training.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	var allFindings []training.Finding

	if len(bundles) == 1 {
		report, err := trainer.Analyze(ctx, *bundles[0])
		if err != nil {
			return fmt.Errorf("analyze run %s: %w", bundles[0].RunID, err)
		}
		allFindings = report.Findings
		if err := store.SaveFindings(bundles[0].RunID, report.Findings); err != nil {
			return fmt.Errorf("save findings: %w", err)
		}
	} else {
		deref := make([]training.TraceBundle, len(bundles))
		for i, b := range bundles {
			deref[i] = *b
		}
		report, err := trainer.AnalyzeBatch(ctx, deref)
		if err != nil {
			return fmt.Errorf("analyze batch: %w", err)
		}
		allFindings = report.Findings
		for _, id := range ids {
			if err := store.SaveFindings(id, report.Findings); err != nil {
				return fmt.Errorf("save findings for %s: %w", id, err)
			}
		}
	}

	return printFindings(allFindings, outputFormat)
}

// newLoopCmd creates the "loop" subcommand.
func newLoopCmd(dbPath *string) *cobra.Command {
	var taskSet string
	var trainerModel string
	var dryRun bool
	var rolloutDir string

	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Run the full training loop: score, analyze, save",
		Long: `Iterates over all JSONL files in the rollout directory, scores each trace,
optionally analyzes with Claude, and saves results to the database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoop(taskSet, trainerModel, dryRun, rolloutDir, *dbPath)
		},
	}

	cmd.Flags().StringVar(&taskSet, "task-set", "all", "Task set filter: all, go, python, etc.")
	cmd.Flags().StringVar(&trainerModel, "trainer", "claude-opus", "Trainer model to use")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would happen without making changes")
	cmd.Flags().StringVar(&rolloutDir, "rollout-dir", defaultRolloutDir(), "Directory containing rollout JSONL files")

	return cmd
}

// runLoop implements the loop command logic.
func runLoop(taskSet, trainerModel string, dryRun bool, rolloutDir, dbPath string) error {
	files, err := findAllRolloutFiles(rolloutDir)
	if err != nil {
		return fmt.Errorf("find rollout files: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No rollout files found in", rolloutDir)
		return nil
	}

	scorer := &training.Scorer{}
	var bundles []*training.TraceBundle
	var scores []training.ScoreResult

	for _, f := range files {
		bundle, err := training.ExportFromJSONL(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", f, err)
			continue
		}
		// Filter by task set if not "all".
		if taskSet != "all" && !strings.Contains(bundle.TaskID, taskSet) {
			continue
		}
		result := scorer.Score(*bundle)
		bundles = append(bundles, bundle)
		scores = append(scores, result)
	}

	if len(bundles) == 0 {
		fmt.Println("No matching traces found for task-set:", taskSet)
		return nil
	}

	// Print scoring summary.
	fmt.Printf("=== Training Loop Summary ===\n")
	fmt.Printf("Traces found: %d\n", len(bundles))
	fmt.Printf("Task set:     %s\n", taskSet)
	fmt.Printf("Dry run:      %v\n\n", dryRun)

	for i, s := range scores {
		fmt.Printf("[%d] %s\n", i+1, s.Summary)
	}

	if dryRun {
		fmt.Println("\n(dry-run) Skipping analysis and database save.")
		return nil
	}

	// Ensure db directory exists.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	store, err := training.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	// Save traces and scores.
	for i, bundle := range bundles {
		if err := store.SaveTrace(*bundle, scores[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: save trace %s: %v\n", bundle.RunID, err)
		}
	}

	// Analyze with Claude if API key is available.
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Println("\nANTHROPIC_API_KEY not set — skipping Claude analysis.")
		fmt.Println("Traces and scores saved to database.")
		return nil
	}

	trainer := training.NewClaudeTrainer(apiKey)
	ctx := context.Background()

	deref := make([]training.TraceBundle, len(bundles))
	for i, b := range bundles {
		deref[i] = *b
	}

	report, err := trainer.AnalyzeBatch(ctx, deref)
	if err != nil {
		return fmt.Errorf("analyze batch: %w", err)
	}

	// Save findings.
	for _, bundle := range bundles {
		if err := store.SaveFindings(bundle.RunID, report.Findings); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: save findings for %s: %v\n", bundle.RunID, err)
		}
	}

	fmt.Printf("\nAnalysis complete. %d findings saved.\n", len(report.Findings))
	return printFindings(report.Findings, "text")
}

// newStatusCmd creates the "status" subcommand.
func newStatusCmd(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show training database status",
		Long:  `Opens the training database and prints summary counts for traces, findings, and applied changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(*dbPath)
		},
	}
}

// runStatus implements the status command logic.
func runStatus(dbPath string) error {
	store, err := training.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	counts, err := queryCounts(store)
	if err != nil {
		return err
	}

	fmt.Println("=== Training Database Status ===")
	fmt.Printf("Database:         %s\n", dbPath)
	fmt.Printf("Traces:           %d\n", counts.traces)
	fmt.Printf("Findings:         %d\n", counts.findings)
	fmt.Printf("Applied Changes:  %d\n", counts.appliedChanges)
	return nil
}

// newHistoryCmd creates the "history" subcommand.
func newHistoryCmd(dbPath *string) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show applied change history",
		Long:  `Queries applied changes from the database since a given date and prints them as a table.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(since, *dbPath)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Show changes since date (YYYY-MM-DD format, required)")
	_ = cmd.MarkFlagRequired("since")

	return cmd
}

// runHistory implements the history command logic.
func runHistory(since, dbPath string) error {
	// Validate date format.
	sinceTime, err := time.Parse("2006-01-02", since)
	if err != nil {
		return fmt.Errorf("invalid date format %q (expected YYYY-MM-DD): %w", since, err)
	}

	store, err := training.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	changes, err := queryHistory(store, sinceTime)
	if err != nil {
		return err
	}

	if len(changes) == 0 {
		fmt.Printf("No applied changes since %s\n", since)
		return nil
	}

	fmt.Printf("=== Applied Changes since %s ===\n", since)
	fmt.Printf("%-12s %-10s %s\n", "DATE", "COMMIT", "DESCRIPTION")
	fmt.Printf("%-12s %-10s %s\n", "----", "------", "-----------")
	for _, c := range changes {
		commitShort := c.GitCommit
		if len(commitShort) > 8 {
			commitShort = commitShort[:8]
		}
		dateShort := c.CreatedAt
		if len(dateShort) > 10 {
			dateShort = dateShort[:10]
		}
		fmt.Printf("%-12s %-10s %s\n", dateShort, commitShort, c.Description)
	}
	return nil
}

// --- helpers ---

// findRolloutFile locates the JSONL file for a given run ID within the rollout directory.
func findRolloutFile(dir, runID string) (string, error) {
	// Try direct match: <dir>/**/<runID>.jsonl
	matches, err := filepath.Glob(filepath.Join(dir, "*", runID+".jsonl"))
	if err != nil {
		return "", err
	}
	if len(matches) > 0 {
		return matches[0], nil
	}

	// Try flat: <dir>/<runID>.jsonl
	flat := filepath.Join(dir, runID+".jsonl")
	if _, err := os.Stat(flat); err == nil {
		return flat, nil
	}

	return "", fmt.Errorf("rollout file not found for run %q in %s", runID, dir)
}

// findAllRolloutFiles returns all .jsonl files under the rollout directory.
func findAllRolloutFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk rollout dir: %w", err)
	}
	return files, nil
}

// loadBundles loads TraceBundle objects for the given run IDs.
func loadBundles(rolloutDir string, ids []string) ([]*training.TraceBundle, error) {
	var bundles []*training.TraceBundle
	for _, id := range ids {
		path, err := findRolloutFile(rolloutDir, id)
		if err != nil {
			return nil, fmt.Errorf("find rollout for %s: %w", id, err)
		}
		bundle, err := training.ExportFromJSONL(path)
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", id, err)
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}

// printFindings outputs findings in the requested format.
func printFindings(findings []training.Finding, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	}

	if len(findings) == 0 {
		fmt.Println("No findings.")
		return nil
	}

	fmt.Printf("\n%-10s %-8s %-20s %s\n", "TYPE", "PRIORITY", "TARGET", "ISSUE")
	fmt.Printf("%-10s %-8s %-20s %s\n", "----", "--------", "------", "-----")
	for _, f := range findings {
		target := f.Target
		if len(target) > 20 {
			target = target[:17] + "..."
		}
		issue := f.Issue
		if len(issue) > 60 {
			issue = issue[:57] + "..."
		}
		fmt.Printf("%-10s %-8s %-20s %s\n", f.Type, f.Priority, target, issue)
	}
	return nil
}

type statusCounts struct {
	traces         int
	findings       int
	appliedChanges int
}

// queryCounts gets summary counts from the store.
func queryCounts(store *training.Store) (statusCounts, error) {
	traces, err := store.CountTraces()
	if err != nil {
		return statusCounts{}, err
	}
	findings, err := store.CountFindings()
	if err != nil {
		return statusCounts{}, err
	}
	applied, err := store.CountAppliedChanges()
	if err != nil {
		return statusCounts{}, err
	}
	return statusCounts{
		traces:         traces,
		findings:       findings,
		appliedChanges: applied,
	}, nil
}

// queryHistory gets applied changes from the store since a given time.
func queryHistory(store *training.Store, since time.Time) ([]training.AppliedChange, error) {
	return store.QueryHistory(since)
}
