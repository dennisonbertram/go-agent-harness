package tools

import (
	"sort"
	"strings"
)

// ToolSearcher searches tool definitions and returns scored results.
type ToolSearcher interface {
	Search(query string, tools []Definition) []SearchResult
}

// SearchResult represents a tool matched by a search query.
type SearchResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Score       float64  `json:"score"`
}

// KeywordSearcher scores tools by keyword overlap across name, description, and tags.
type KeywordSearcher struct {
	MaxResults int
}

// Search performs keyword-based tool search. It lowercases the query, splits
// into terms, and scores each tool by substring matches in name/description/tags.
// Exact name or tag matches get a bonus. Results are sorted by score descending
// and capped at MaxResults.
func (s *KeywordSearcher) Search(query string, tools []Definition) []SearchResult {
	if query == "" || len(tools) == 0 {
		return nil
	}

	query = strings.ToLower(query)
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil
	}

	var results []SearchResult

	for _, tool := range tools {
		score := scoreTool(tool, terms)
		if score > 0 {
			// Truncate description for search results
			desc := tool.Description
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			results = append(results, SearchResult{
				Name:        tool.Name,
				Description: desc,
				Tags:        tool.Tags,
				Score:       score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	max := s.MaxResults
	if max <= 0 {
		max = 10
	}
	if len(results) > max {
		results = results[:max]
	}

	return results
}

// scoreTool computes a relevance score for a tool against query terms.
func scoreTool(tool Definition, terms []string) float64 {
	var score float64

	nameLower := strings.ToLower(tool.Name)
	descLower := strings.ToLower(tool.Description)

	tagsLower := make([]string, len(tool.Tags))
	for i, t := range tool.Tags {
		tagsLower[i] = strings.ToLower(t)
	}

	for _, term := range terms {
		// Skip short stop words (1-2 characters, e.g. "a", "in", "to", "of").
		// Short terms cause spurious substring matches in names and tags — for
		// example "in" matches inside the tag "inspect" (+3 tag substring) and
		// "contents" (+3 tag substring), generating 6+ spurious bonus points
		// that corrupt ranking between otherwise unrelated tools.
		if len(term) <= 2 {
			continue
		}

		// Name match (highest value)
		if nameLower == term {
			score += 10 // exact name match bonus
		} else if strings.Contains(nameLower, term) {
			score += 5
		}

		// Tag match
		for _, tag := range tagsLower {
			if tag == term {
				score += 8 // exact tag match bonus
			} else if strings.Contains(tag, term) {
				score += 3
			}
		}

		// Description match (lowest value)
		if strings.Contains(descLower, term) {
			score += 1
		}
	}

	return score
}
