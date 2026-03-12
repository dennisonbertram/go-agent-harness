// Package audittrail provides an append-only, hash-chained audit log for
// compliance and accountability in the agent harness.
//
// The audit log captures only security-relevant events:
//   - run.started — records run provenance (initiator, model, prompt).
//   - audit.action — records state-modifying tool calls.
//   - run.completed / run.failed — records run terminal state.
//
// Each entry is hashed with a chain linking it to the previous entry,
// providing tamper-evidence: any modification to a past entry breaks all
// subsequent hashes.
package audittrail

import "strings"

// exactStateModifyingTools lists tools that are unconditionally state-modifying
// regardless of their name's keyword content.
var exactStateModifyingTools = map[string]bool{
	"bash":       true,
	"file_write": true,
	"file_delete": true,
	"git_commit": true,
	"git_push":   true,
}

// stateModifyingKeywords lists substrings that, when appearing as a
// underscore-separated token in a tool name, mark it as state-modifying.
// Matching is done on the set of tokens produced by splitting on "_".
var stateModifyingKeywords = []string{
	"write",
	"delete",
	"create",
	"modify",
}

// IsStateModifying reports whether a tool call with the given name performs
// state-modifying operations (writes, deletes, creates, or modifies data).
//
// Classification rules (applied in order):
//  1. Exact match against known state-modifying tool names (bash, file_write,
//     file_delete, git_commit, git_push).
//  2. Keyword match: if any underscore-separated token in the tool name exactly
//     equals one of "write", "delete", "create", or "modify", the tool is
//     considered state-modifying.
//
// Examples:
//   - "bash"           → true  (exact match)
//   - "file_write"     → true  (exact match)
//   - "git_push"       → true  (exact match)
//   - "write_config"   → true  (keyword "write" as first token)
//   - "create_file"    → true  (keyword "create" as first token)
//   - "file_read"      → false (no match)
//   - "grep"           → false (no match)
//   - "writer"         → false ("writer" ≠ "write" token)
func IsStateModifying(toolName string) bool {
	if toolName == "" {
		return false
	}

	// Rule 1: exact match against known tools.
	if exactStateModifyingTools[toolName] {
		return true
	}

	// Rule 2: keyword match on underscore-separated tokens.
	tokens := strings.Split(toolName, "_")
	for _, token := range tokens {
		for _, kw := range stateModifyingKeywords {
			if token == kw {
				return true
			}
		}
	}

	return false
}
