package slashcomplete

import (
	"sort"
	"strings"
)

// Score computes a fuzzy match score for a query against a target string.
// Higher score = better match. Returns 0 if no match at all.
// Scoring rules:
//   - Exact prefix match scores highest (100 + remaining chars bonus)
//   - Contains match scores medium (50 + position bonus: earlier = higher)
//   - Subsequence match scores low (25 + coverage ratio)
//   - No match: 0
func Score(query, target string) int {
	if query == "" {
		return 1 // any non-empty target beats nothing
	}

	q := strings.ToLower(query)
	t := strings.ToLower(target)

	if t == "" {
		return 0
	}

	// Exact prefix match: highest tier
	if strings.HasPrefix(t, q) {
		// Bonus for shorter remaining tail (shorter target = more precise match)
		remaining := len(t) - len(q)
		return 100 + (50 - remaining)
	}

	// Contains match: medium tier — bonus for earlier position
	idx := strings.Index(t, q)
	if idx >= 0 {
		// Earlier position = higher score; position 0 would be prefix (handled above)
		posBonus := len(t) - idx
		return 50 + posBonus
	}

	// Subsequence match: low tier — all query runes appear in order in target
	if isSubsequence(q, t) {
		// Coverage ratio: how much of target is "covered" by the match
		coverage := len(q) * 10 / len(t)
		return 25 + coverage
	}

	return 0
}

// isSubsequence returns true if all runes of q appear in t in order.
func isSubsequence(q, t string) bool {
	qr := []rune(q)
	tr := []rune(t)
	qi := 0
	for ti := 0; ti < len(tr) && qi < len(qr); ti++ {
		if tr[ti] == qr[qi] {
			qi++
		}
	}
	return qi == len(qr)
}

// FuzzyFilter returns suggestions ranked by fuzzy Score against query.
// Suggestions with Score==0 are excluded.
// Stable sort: equal scores maintain insertion order.
func FuzzyFilter(suggestions []Suggestion, query string) []Suggestion {
	if query == "" {
		cp := make([]Suggestion, len(suggestions))
		copy(cp, suggestions)
		return cp
	}

	type scored struct {
		s     Suggestion
		score int
		index int
	}

	var results []scored
	for i, s := range suggestions {
		sc := Score(query, s.Name)
		if sc > 0 {
			results = append(results, scored{s: s, score: sc, index: i})
		}
	}

	// Stable sort by descending score; equal scores keep insertion order (stable).
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	out := make([]Suggestion, len(results))
	for i, r := range results {
		out[i] = r.s
	}
	return out
}
