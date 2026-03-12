package causalgraph

import (
	"strings"
	"unicode"
)

// stopwords is a set of common English words that should be excluded from
// data-flow token matching even when they meet the minimum length threshold.
var stopwords = map[string]bool{
	"should": true, "return": true, "string": true, "result": true,
	"before": true, "because": true, "between": true, "through": true,
	"during": true, "without": true, "within": true, "against": true,
	"around": true, "another": true, "already": true, "always": true,
	"include": true, "system": true, "output": true, "number": true,
	"called": true, "create": true, "change": true, "update": true,
	"delete": true, "cannot": true, "status": true, "failed": true,
	"please": true, "simply": true, "rather": true, "really": true,
	"enough": true, "across": true, "except": true, "entire": true,
	"inside": true, "itself": true, "object": true, "method": true,
	"public": true, "private": true, "import": true, "export": true,
	"module": true, "package": true, "struct": true, "interface": true,
	"function": true, "variable": true, "boolean": true, "integer": true,
	"default": true, "defined": true,
}

// isStopword reports whether a lowercased token is a stopword.
func isStopword(token string) bool {
	return stopwords[strings.ToLower(token)]
}

// ExtractTokens returns significant tokens from text. A token is significant
// if it is at least 6 characters long (after stripping surrounding punctuation)
// and is not a common stopword. Tokens are deduplicated and returned in
// lowercase to ensure case-insensitive data-flow matching.
func ExtractTokens(text string) []string {
	fields := strings.Fields(text)
	seen := make(map[string]bool, len(fields))
	var tokens []string
	for _, f := range fields {
		// Strip surrounding punctuation.
		cleaned := strings.TrimFunc(f, func(r rune) bool {
			return unicode.IsPunct(r)
		})
		if len(cleaned) < 6 {
			continue
		}
		lower := strings.ToLower(cleaned)
		if isStopword(lower) {
			continue
		}
		if seen[lower] {
			continue
		}
		seen[lower] = true
		tokens = append(tokens, lower) // return lowercase for consistent matching
	}
	return tokens
}

// maxResultBytes caps how many bytes of a tool result are processed for
// data-flow matching to prevent O(n) token explosion on large outputs.
const maxResultBytes = 65536 // 64 KiB

// maxTokensPerResult caps how many tokens are extracted from a single result.
const maxTokensPerResult = 500

// maxTokenLen caps individual token length — longer tokens are likely noise.
const maxTokenLen = 100

// maxResultEntries caps how many result-producing calls are checked for
// data-flow matches, bounding the outer loop of FindDataFlowEdges.
const maxResultEntries = 100

// maxTargetChecks caps how many (result, target) pairs are evaluated in total
// across the entire call to FindDataFlowEdges.
const maxTargetChecks = 10_000

// FindDataFlowEdges detects when output tokens from one tool call appear in a
// later tool call's arguments. Only forward edges are created (source must come
// before target in ordering). For each (from, to) pair, only the first matched
// token is recorded to avoid redundant edges.
func FindDataFlowEdges(results map[string]string, args map[string]string, ordering []string) []Edge {
	if len(ordering) == 0 {
		return nil
	}

	// Build position index for ordering enforcement.
	pos := make(map[string]int, len(ordering))
	for i, id := range ordering {
		pos[id] = i
	}

	// Pre-extract tokens from all results.
	type tokenEntry struct {
		callID string
		tokens []string
	}
	var resultTokens []tokenEntry
	for _, id := range ordering {
		r, ok := results[id]
		if !ok || r == "" {
			continue
		}
		// Cap result size before token extraction to bound processing time.
		if len(r) > maxResultBytes {
			r = r[:maxResultBytes]
		}
		toks := ExtractTokens(r)
		// Cap token count and filter oversized tokens.
		var filtered []string
		for _, tok := range toks {
			if len(tok) > maxTokenLen {
				continue
			}
			filtered = append(filtered, tok)
			if len(filtered) >= maxTokensPerResult {
				break
			}
		}
		if len(filtered) > 0 {
			resultTokens = append(resultTokens, tokenEntry{callID: id, tokens: filtered})
		}
	}

	// Cap result entries to bound outer loop iterations.
	if len(resultTokens) > maxResultEntries {
		resultTokens = resultTokens[:maxResultEntries]
	}

	// Pre-compute lowercase args for all targets once — avoids repeated allocations
	// in the hot (result, target) loop.
	lowerArgs := make(map[string]string, len(args))
	for id, a := range args {
		if a != "" {
			lowerArgs[id] = strings.ToLower(a)
		}
	}

	// For each result's tokens, check if they appear in any later call's args.
	type edgeKey struct{ from, to string }
	seen := make(map[edgeKey]bool)
	var edges []Edge
	totalChecks := 0

	for _, rt := range resultTokens {
		fromPos := pos[rt.callID]
		for _, targetID := range ordering {
			if totalChecks >= maxTargetChecks {
				return edges // budget exhausted — return what we have
			}
			targetPos := pos[targetID]
			if targetPos <= fromPos {
				continue // only forward edges
			}
			totalChecks++
			targetArgsLower, ok := lowerArgs[targetID]
			if !ok {
				continue
			}
			key := edgeKey{rt.callID, targetID}
			if seen[key] {
				continue
			}
			for _, tok := range rt.tokens {
				// tok is already lowercase (ExtractTokens returns lowercase).
				if strings.Contains(targetArgsLower, tok) {
					seen[key] = true
					edges = append(edges, Edge{
						From:         rt.callID,
						To:           targetID,
						Type:         EdgeTypeDataFlow,
						MatchedToken: tok,
					})
					break // one edge per (from, to) pair
				}
			}
		}
	}

	return edges
}
