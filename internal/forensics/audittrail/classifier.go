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
// Package audittrail provides an append-only, hash-chained audit log for
// compliance and accountability in the agent harness.
package audittrail

import (
	"strings"
	"unicode"
)

// exactStateModifyingTools lists tools that are unconditionally state-modifying
// regardless of their name's keyword content. Extend this list when new
// state-modifying tools with names that don't contain keyword tokens are added.
//
// HIGH-4 fix: extended with additional common state-modifying tool names.
// Name-heuristic classification is best-effort; operators should maintain
// this explicit list as the primary classification mechanism.
var exactStateModifyingTools = map[string]bool{
	"bash":           true,
	"file_write":     true,
	"file_delete":    true,
	"git_commit":     true,
	"git_push":       true,
	"git_reset":      true,
	"git_revert":     true,
	"git_merge":      true,
	"git_rebase":     true,
	"kubectl_apply":  true,
	"kubectl_delete": true,
	"kubectl_patch":  true,
	"rm":             true,
	"mv":             true,
	"cp":             true,
	"chmod":          true,
	"chown":          true,
	"mkdir":          true,
	"mkfile":         true,
	"touch":          true,
}

// stateModifyingKeywords lists tokens that, when appearing as a word boundary
// token in a tool name (underscore-, hyphen-, or camelCase-delimited), mark
// the tool as state-modifying.
//
// HIGH-4 fix: extended keyword list to reduce bypass surface for tools with
// non-obvious names (e.g., "applypatch", "persist_record", "put_object").
// Token matching is performed after normalizing to lowercase.
var stateModifyingKeywords = map[string]bool{
	"write":     true,
	"delete":    true,
	"create":    true,
	"modify":    true,
	"update":    true,
	"patch":     true,
	"put":       true,
	"remove":    true,
	"insert":    true,
	"append":    true,
	"exec":      true,
	"execute":   true,
	"commit":    true,
	"apply":     true,
	"deploy":    true,
	"install":   true,
	"uninstall": true,
	"destroy":   true,
	"overwrite": true,
	"rename":    true,
	"move":      true,
	"wipe":      true,
	"flush":     true,
	"publish":   true,
	"push":      true,
	"post":      true,
	"submit":    true,
	"persist":   true,
	"store":     true,
	"save":      true,
	"upload":    true,
	"send":      true,
	"format":    true,
	"init":      true,
	"rollback":  true,
}

// splitToolNameTokens splits a tool name into lower-case tokens by:
//  1. Splitting on underscore and hyphen delimiters.
//  2. Further splitting each part on camelCase boundaries
//     (e.g., "commitChanges" → ["commit", "Changes"]).
//
// All tokens are returned in lower-case.
func splitToolNameTokens(name string) []string {
	// Step 1: split on underscore and hyphen.
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})

	var tokens []string
	for _, part := range parts {
		// Step 2: split each part on camelCase boundaries.
		tokens = append(tokens, splitCamelCase(part)...)
	}
	return tokens
}

// splitCamelCase splits a camelCase or PascalCase string into lower-case
// words at uppercase→lowercase boundaries (e.g., "commitChanges" →
// ["commit", "changes"], "applyPatch" → ["apply", "patch"]).
func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	start := 0
	runes := []rune(s)
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) && !unicode.IsUpper(runes[i-1]) {
			words = append(words, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	words = append(words, strings.ToLower(string(runes[start:])))
	return words
}

// IsStateModifying reports whether a tool call with the given name performs
// state-modifying operations (writes, deletes, creates, or modifies data).
//
// Classification rules (applied in order):
//  1. Exact match against the known state-modifying tool name set.
//  2. Keyword match: tokens extracted by splitting on "_", "-", and camelCase
//     boundaries are matched against the keyword set (case-insensitive).
//
// LIMITATION: name-heuristic classification is best-effort. Tools with
// state-modifying semantics but non-keyword names (e.g., "applypatch" as a
// single token, exotic verbs) may be misclassified. Add such tools to
// exactStateModifyingTools for reliable classification.
//
// Examples:
//   - "bash"              → true  (exact match)
//   - "file_write"        → true  (exact + keyword "write")
//   - "write_config"      → true  (keyword "write")
//   - "applyPatch"        → true  (camelCase token "apply" matches keyword)
//   - "commitChanges"     → true  (camelCase token "commit" matches keyword)
//   - "put-object"        → true  (hyphen token "put" matches keyword)
//   - "persist_record"    → true  (keyword "persist")
//   - "file_read"         → false (no match)
//   - "grep"              → false (no match)
//   - "writer"            → false ("writer" ≠ "write" token)
func IsStateModifying(toolName string) bool {
	if toolName == "" {
		return false
	}

	// Rule 1: exact match against known tools.
	if exactStateModifyingTools[toolName] {
		return true
	}

	// Rule 2: keyword match on normalized tokens (underscore, hyphen, camelCase).
	for _, token := range splitToolNameTokens(toolName) {
		if stateModifyingKeywords[token] {
			return true
		}
	}

	return false
}
