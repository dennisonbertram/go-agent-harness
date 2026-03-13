package causalgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

// tokenFingerprint returns a safe, non-reversible representation of a data-flow
// token. Storing the raw token in Edge.MatchedToken risks exfiltrating secrets
// or PII extracted from prior tool results, bypassing the redaction pipeline.
//
// HIGH-4 fix: raw tokens (API keys, JWT fragments, passwords) of ≥6 chars from
// tool results can be copied into Edge.MatchedToken and serialized to JSON
// forensic outputs. tokenFingerprint stores sha256(token)[:16hex]+length,
// sufficient to correlate "same token appeared in this pair" without exposure.
func tokenFingerprint(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return fmt.Sprintf("[sha256:%s][len=%d]", hex.EncodeToString(h[:])[:16], len(tok))
}

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

// maxScanBytes is the total byte-scanning budget for all strings.Contains
// calls in FindDataFlowEdges. Each call scans up to len(targetArgsLower) bytes,
// and with maxTargetChecks × maxTokensPerResult × maxArgBytes the naive worst
// case is enormous; this hard cap bounds total CPU regardless.
const maxScanBytes = 64 * 1024 * 1024 // 64 MiB total scan budget

// maxArgBytes caps how many bytes of a single tool call's arguments string are
// lowercased and searched for data-flow tokens. Without this cap, a single
// large arguments field could cause O(maxTargetChecks × maxArgBytes) work.
const maxArgBytes = 65536 // 64 KiB

// maxArgEntries caps how many distinct call argument entries are precomputed
// into lowerArgs. Without this cap, a rollout with thousands of tool calls
// (each near maxArgBytes) could cause large transient memory allocations
// during the lowerArgs precomputation phase, before any matching-loop budget
// kicks in.
const maxArgEntries = 1000

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

	// Pre-compute lowercase args from ordering (not from ranging the args map)
	// to ensure deterministic iteration order. Go map iteration is randomized;
	// ranging over args would make forensics output nondeterministic across runs
	// and allow attackers to pad with dummy entries so "interesting" args are
	// excluded when the cap is hit. Iterating ordering is deterministic and
	// follows causal sequence. Cap at maxArgEntries to bound upfront memory.
	lowerArgs := make(map[string]string, min(len(ordering), maxArgEntries))
	argCount := 0
	for _, id := range ordering {
		if argCount >= maxArgEntries {
			break
		}
		a, ok := args[id]
		if !ok || a == "" {
			continue
		}
		if len(a) > maxArgBytes {
			a = a[:maxArgBytes]
		}
		lowerArgs[id] = strings.ToLower(a)
		argCount++
	}

	// For each result's tokens, check if they appear in any later call's args.
	// Only ordering[fromPos+1:] is scanned to avoid wasting budget on backward
	// entries that are unconditionally skipped anyway.
	type edgeKey struct{ from, to string }
	seen := make(map[edgeKey]bool)
	var edges []Edge
	totalChecks := 0
	totalScanBytes := 0

	for _, rt := range resultTokens {
		fromPos := pos[rt.callID]
		for _, targetID := range ordering[fromPos+1:] {
			if totalChecks >= maxTargetChecks {
				return edges // pair-count budget exhausted
			}
			if totalScanBytes >= maxScanBytes {
				return edges // byte-scan budget exhausted
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
				totalScanBytes += len(targetArgsLower)
				if strings.Contains(targetArgsLower, tok) {
					seen[key] = true
					edges = append(edges, Edge{
						From:         rt.callID,
						To:           targetID,
						Type:         EdgeTypeDataFlow,
						MatchedToken: tokenFingerprint(tok), // HIGH-4 fix: fingerprint, not raw value
					})
					break // one edge per (from, to) pair
				}
			}
		}
	}

	return edges
}
