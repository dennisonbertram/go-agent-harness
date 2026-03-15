package conclusionwatcher

import (
	"path/filepath"
	"strings"
	"sync"
)

// ObservationLedger tracks which file paths and tool names have been
// observed in the current run. Thread-safe.
type ObservationLedger struct {
	mu            sync.RWMutex
	observedFiles map[string]struct{} // normalized file paths seen via read/grep/glob/ls
	toolHistory   []toolEntry         // ordered list of tool calls this run
}

type toolEntry struct {
	step     int
	toolName string
	args     string
}

// DiagnosticTools is the set of tool names considered "diagnostic"
// (non-mutating, exploratory).
var DiagnosticTools = map[string]bool{
	"read_file": true,
	"grep":      true,
	"glob":      true,
	"bash":      true, // treated as diagnostic only when non-destructive
	"list_dir":  true,
	"git_log":   true,
	"git_diff":  true,
	"git_show":  true,
	"search":    true,
}

// ExplorationTools is the subset of DiagnosticTools that indicates
// explicit codebase exploration.
var ExplorationTools = map[string]bool{
	"read_file": true,
	"grep":      true,
	"glob":      true,
	"list_dir":  true,
	"git_log":   true,
	"git_diff":  true,
	"git_show":  true,
}

// MutatingTools is the set of tool names considered mutating / destructive.
var MutatingTools = map[string]bool{
	"write_file":  true,
	"edit_file":   true,
	"bash":        true, // bash can be either; context-dependent heuristic in detector
	"delete_file": true,
	"move_file":   true,
	"patch_file":  true,
}

// NewObservationLedger creates a new empty ledger.
func NewObservationLedger() *ObservationLedger {
	return &ObservationLedger{
		observedFiles: make(map[string]struct{}),
	}
}

// normalizePath strips leading "./" and runs filepath.Clean, then converts
// to forward slashes. Does NOT call filepath.Abs to keep tests deterministic.
func normalizePath(path string) string {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	cleaned = strings.TrimPrefix(cleaned, "./")
	return cleaned
}

// RecordFileSeen records that a file path was accessed by a tool.
// Both the normalized path and the base name are stored so that path-prefix
// mismatches (e.g. /workspace/foo.go vs foo.go) can still match.
func (l *ObservationLedger) RecordFileSeen(path string) {
	if path == "" {
		return
	}
	normalized := normalizePath(path)
	base := filepath.Base(normalized)
	l.mu.Lock()
	l.observedFiles[normalized] = struct{}{}
	l.observedFiles[base] = struct{}{}
	l.mu.Unlock()
}

// HasSeenFile reports whether path has been recorded.
func (l *ObservationLedger) HasSeenFile(path string) bool {
	if path == "" {
		return false
	}
	normalized := normalizePath(path)
	base := filepath.Base(normalized)
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.observedFiles[normalized]
	if ok {
		return true
	}
	_, ok = l.observedFiles[base]
	return ok
}

// RecordTool appends a tool call to the ordered history.
// args is the raw JSON arguments string (may be empty).
func (l *ObservationLedger) RecordTool(step int, toolName, args string) {
	l.mu.Lock()
	l.toolHistory = append(l.toolHistory, toolEntry{
		step:     step,
		toolName: toolName,
		args:     args,
	})
	l.mu.Unlock()
}

// RecentTools returns the tool names used in the last n steps (in order,
// most recent last). n <= 0 returns the full history.
func (l *ObservationLedger) RecentTools(n int) []string {
	l.mu.RLock()
	history := make([]toolEntry, len(l.toolHistory))
	copy(history, l.toolHistory)
	l.mu.RUnlock()

	if n <= 0 || n >= len(history) {
		names := make([]string, len(history))
		for i, e := range history {
			names[i] = e.toolName
		}
		return names
	}
	recent := history[len(history)-n:]
	names := make([]string, len(recent))
	for i, e := range recent {
		names[i] = e.toolName
	}
	return names
}

// recentEntries returns the last n toolEntry records (most recent last).
// Caller must hold no lock (it takes the read lock internally).
func (l *ObservationLedger) recentEntries(n int) []toolEntry {
	l.mu.RLock()
	history := make([]toolEntry, len(l.toolHistory))
	copy(history, l.toolHistory)
	l.mu.RUnlock()

	if n <= 0 || n >= len(history) {
		return history
	}
	return history[len(history)-n:]
}

// LastStepHadDiagnostic reports whether any diagnostic tool was called
// during step or step-1 (the "current or previous step" window).
func (l *ObservationLedger) LastStepHadDiagnostic(currentStep int) bool {
	l.mu.RLock()
	history := make([]toolEntry, len(l.toolHistory))
	copy(history, l.toolHistory)
	l.mu.RUnlock()

	for _, e := range history {
		if (e.step == currentStep || e.step == currentStep-1) && DiagnosticTools[e.toolName] {
			return true
		}
	}
	return false
}

// LastStepHadExploration reports whether any exploration tool was called
// during step or step-1.
func (l *ObservationLedger) LastStepHadExploration(currentStep int) bool {
	l.mu.RLock()
	history := make([]toolEntry, len(l.toolHistory))
	copy(history, l.toolHistory)
	l.mu.RUnlock()

	for _, e := range history {
		if (e.step == currentStep || e.step == currentStep-1) && ExplorationTools[e.toolName] {
			return true
		}
	}
	return false
}

// hasExplorationInLastN checks if any exploration tool was used in the last n tool entries.
func (l *ObservationLedger) hasExplorationInLastN(n int) bool {
	entries := l.recentEntries(n)
	for _, e := range entries {
		if ExplorationTools[e.toolName] {
			return true
		}
	}
	return false
}

// hasVerificationInLastN checks if any verification tool was used in the last n entries.
// bash counts only if args contain test keywords.
func (l *ObservationLedger) hasVerificationInLastN(n int) bool {
	verificationTools := map[string]bool{
		"run_tests": true,
		"go_test":   true,
		"check":     true,
		"verify":    true,
	}
	entries := l.recentEntries(n)
	for _, e := range entries {
		if verificationTools[e.toolName] {
			return true
		}
		if e.toolName == "bash" && bashHasTestKeyword(e.args) {
			return true
		}
	}
	return false
}

// bashHasTestKeyword reports whether the bash args JSON contains a test-related keyword.
func bashHasTestKeyword(args string) bool {
	if args == "" {
		return false
	}
	lower := strings.ToLower(args)
	testKeywords := []string{"go test", "pytest", "npm test", "cargo test", " test ", "-run", "check", "verify"}
	for _, kw := range testKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// Reset clears the ledger. Call between runs if a single watcher is
// reused across runs (not the typical pattern; New() is preferred per-run).
func (l *ObservationLedger) Reset() {
	l.mu.Lock()
	l.observedFiles = make(map[string]struct{})
	l.toolHistory = nil
	l.mu.Unlock()
}
