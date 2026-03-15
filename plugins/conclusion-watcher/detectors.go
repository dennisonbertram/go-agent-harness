package conclusionwatcher

import (
	"encoding/json"
	"regexp"
	"strings"
)

// hedgePhrases are the case-insensitive substrings that trigger HedgeAssertion.
var hedgePhrases = []string{
	"must be", "clearly", "obviously", "definitely",
	"i assume", "probably is", "should be", "it appears that",
}

// filePathRe matches file paths with known extensions.
var filePathRe = regexp.MustCompile(
	`(?i)\b[\w./\-]+\.(?:go|py|ts|js|jsx|tsx|yaml|yml|json|toml|md|sh|txt|env|cfg|conf)\b`,
)

// completionTerms trigger PrematureCompletion when found in response content.
var completionTerms = []string{
	"done", "fixed", "complete", "completed",
	"resolved", "implemented", "finished", "all set",
}

// destructiveBashPatterns identify destructive bash commands.
var destructiveBashPatterns = []string{
	"rm ", "mv ", "cp ", "chmod ", "chown ",
	"sed -i", "awk ", "truncate",
	"tee ", "> /", ">> /",
}

// architecturePhrases trigger ArchitectureAssumption.
var architecturePhrases = []string{
	"the design is", "the flow is", "this is a bug",
	"the architecture requires", "the intended flow is",
	"this is how it works", "the system does",
}

// DetectHedgeAssertion fires when the response content contains hedge-assertion
// language: "must be", "clearly", "obviously", "definitely", "I assume",
// "probably is", "should be", "it appears that".
// Returns non-nil when any phrase is found. Case-insensitive. Confidence: 1.0.
func DetectHedgeAssertion(runID string, step int, content string) *DetectionResult {
	if content == "" {
		return nil
	}
	lower := strings.ToLower(content)
	for _, phrase := range hedgePhrases {
		if strings.Contains(lower, phrase) {
			return &DetectionResult{
				Pattern:    PatternHedgeAssertion,
				Confidence: 1.0,
				Evidence:   phrase,
				Step:       step,
				RunID:      runID,
			}
		}
	}
	return nil
}

// DetectUnverifiedFileClaim fires when the content mentions a file path AND
// uses an assertion keyword, but the ledger has no record of that file
// being read. Returns nil when ledger is nil (safe fallback).
func DetectUnverifiedFileClaim(runID string, step int, content string, ledger *ObservationLedger) *DetectionResult {
	if content == "" || ledger == nil {
		return nil
	}

	// Check if content has an assertion keyword.
	lower := strings.ToLower(content)
	hasAssertion := false
	for _, phrase := range hedgePhrases {
		if strings.Contains(lower, phrase) {
			hasAssertion = true
			break
		}
	}
	if !hasAssertion {
		return nil
	}

	// Find all file path matches.
	matches := filePathRe.FindAllString(content, -1)
	for _, match := range matches {
		if !ledger.HasSeenFile(match) {
			return &DetectionResult{
				Pattern:    PatternUnverifiedFileClaim,
				Confidence: 0.8,
				Evidence:   match,
				Step:       step,
				RunID:      runID,
			}
		}
	}
	return nil
}

// DetectPrematureCompletion fires when the content contains completion language
// but the recent tool history (last 3 steps) contains no test-or-verification tool.
// Returns nil when ledger is nil.
func DetectPrematureCompletion(runID string, step int, content string, ledger *ObservationLedger) *DetectionResult {
	if content == "" {
		return nil
	}

	lower := strings.ToLower(content)
	hasCompletion := false
	var evidence string
	for _, term := range completionTerms {
		if strings.Contains(lower, term) {
			hasCompletion = true
			evidence = term
			break
		}
	}
	if !hasCompletion {
		return nil
	}

	if ledger == nil {
		return &DetectionResult{
			Pattern:    PatternPrematureCompletion,
			Confidence: 0.9,
			Evidence:   evidence,
			Step:       step,
			RunID:      runID,
		}
	}

	// Look back 3 steps for verification tool.
	if ledger.hasVerificationInLastN(3) {
		return nil
	}

	return &DetectionResult{
		Pattern:    PatternPrematureCompletion,
		Confidence: 0.9,
		Evidence:   evidence,
		Step:       step,
		RunID:      runID,
	}
}

// isBashDestructive checks whether bash args JSON contains a destructive command.
func isBashDestructive(args []byte) bool {
	if len(args) == 0 {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		// Malformed args: fail closed (treat as non-destructive to avoid false positives).
		return false
	}
	cmd, _ := m["command"].(string)
	if cmd == "" {
		return false
	}
	lower := cmd
	for _, pat := range destructiveBashPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// DetectSkippedDiagnostic fires when the proposed tool call is a mutating tool
// (write_file, edit_file, or bash with a destructive command pattern) AND
// neither the current step nor the immediately preceding step included a
// diagnostic tool call.
// Returns nil when ledger is nil or args are nil (safe fallback).
func DetectSkippedDiagnostic(runID string, step int, toolName string, args []byte, ledger *ObservationLedger) *DetectionResult {
	if ledger == nil {
		return nil
	}

	var isMutating bool
	var confidence float64
	var evidence string

	switch toolName {
	case "write_file", "edit_file", "patch_file", "delete_file":
		isMutating = true
		confidence = 1.0
		evidence = toolName + " without prior diagnostic"
	case "bash":
		if isBashDestructive(args) {
			isMutating = true
			confidence = 0.85
			// Extract command for evidence.
			var m map[string]any
			if err := json.Unmarshal(args, &m); err == nil {
				if cmd, ok := m["command"].(string); ok {
					evidence = "destructive bash: " + cmd
				}
			}
			if evidence == "" {
				evidence = "destructive bash command"
			}
		}
	}

	if !isMutating {
		return nil
	}

	if ledger.LastStepHadDiagnostic(step) {
		return nil
	}

	return &DetectionResult{
		Pattern:    PatternSkippedDiagnostic,
		Confidence: confidence,
		Evidence:   evidence,
		Step:       step,
		RunID:      runID,
	}
}

// DetectArchitectureAssumption fires when the content contains architecture
// assertion phrases AND no exploration tool has been called in the recent
// history (last 3 steps).
// Returns nil when ledger is nil.
func DetectArchitectureAssumption(runID string, step int, content string, ledger *ObservationLedger) *DetectionResult {
	if content == "" {
		return nil
	}

	lower := strings.ToLower(content)
	hasPhrase := false
	var evidence string
	for _, phrase := range architecturePhrases {
		if strings.Contains(lower, phrase) {
			hasPhrase = true
			evidence = phrase
			break
		}
	}
	if !hasPhrase {
		return nil
	}

	if ledger == nil {
		return &DetectionResult{
			Pattern:    PatternArchitectureAssumption,
			Confidence: 0.75,
			Evidence:   evidence,
			Step:       step,
			RunID:      runID,
		}
	}

	if ledger.hasExplorationInLastN(3) {
		return nil
	}

	return &DetectionResult{
		Pattern:    PatternArchitectureAssumption,
		Confidence: 0.75,
		Evidence:   evidence,
		Step:       step,
		RunID:      runID,
	}
}
