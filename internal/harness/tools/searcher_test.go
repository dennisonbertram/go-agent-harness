package tools

import (
	"strings"
	"testing"
)

func TestKeywordSearcher_EmptyQuery(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{{Name: "read_file", Description: "Read a file"}}

	got := s.Search("", tools)
	if got != nil {
		t.Fatalf("expected nil for empty query, got %v", got)
	}

	// whitespace-only query should also return nil
	got = s.Search("   ", tools)
	if got != nil {
		t.Fatalf("expected nil for whitespace-only query, got %v", got)
	}
}

func TestKeywordSearcher_EmptyTools(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}

	got := s.Search("read", nil)
	if got != nil {
		t.Fatalf("expected nil for nil tools, got %v", got)
	}

	got = s.Search("read", []Definition{})
	if got != nil {
		t.Fatalf("expected nil for empty tools, got %v", got)
	}
}

func TestKeywordSearcher_ExactNameMatch(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "read_file", Description: "Read a file from disk"},
		{Name: "write_file", Description: "Write content to a file"},
		{Name: "grep", Description: "Search file contents with regex"},
	}

	results := s.Search("grep", tools)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Name != "grep" {
		t.Fatalf("expected exact name match 'grep' first, got %q", results[0].Name)
	}
	// Exact name match = 10 + description "search" doesn't match "grep" = 0
	// but the description contains no "grep" either, so just the name bonus
	if results[0].Score < 10 {
		t.Fatalf("expected exact name match score >= 10, got %f", results[0].Score)
	}
}

func TestKeywordSearcher_TagMatch(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "web_search", Description: "Search the web", Tags: []string{"search", "web", "internet"}},
		{Name: "grep", Description: "Search file contents", Tags: []string{"search", "regex", "files"}},
		{Name: "read_file", Description: "Read a file from disk", Tags: []string{"file", "read"}},
	}

	results := s.Search("search", tools)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results for 'search', got %d", len(results))
	}

	// Both web_search and grep have "search" as an exact tag match (score 8)
	// web_search also has "search" as a substring of the name (score 5) and description (score 1)
	// grep also has "search" in description (score 1)
	// So web_search should score higher
	if results[0].Name != "web_search" {
		t.Fatalf("expected web_search first (has name substring + tag + desc match), got %q", results[0].Name)
	}
}

func TestKeywordSearcher_DescriptionMatch(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "tool_a", Description: "Manages database connections and queries"},
		{Name: "tool_b", Description: "Handles HTTP routing"},
	}

	results := s.Search("database", tools)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'database', got %d", len(results))
	}
	if results[0].Name != "tool_a" {
		t.Fatalf("expected tool_a matched by description, got %q", results[0].Name)
	}
	// Description-only match = score of 1
	if results[0].Score != 1 {
		t.Fatalf("expected description-only score of 1, got %f", results[0].Score)
	}
}

func TestKeywordSearcher_MultiTermQuery(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "read_file", Description: "Read a file from disk", Tags: []string{"file", "read", "disk"}},
		{Name: "write_file", Description: "Write content to a file", Tags: []string{"file", "write"}},
		{Name: "grep", Description: "Search file contents", Tags: []string{"search", "regex"}},
	}

	// "read file" -> two terms, read_file matches both strongly
	results := s.Search("read file", tools)
	if len(results) == 0 {
		t.Fatal("expected results for 'read file'")
	}
	if results[0].Name != "read_file" {
		t.Fatalf("expected read_file first for multi-term 'read file', got %q", results[0].Name)
	}
	// read_file should accumulate scores from both terms:
	// "read": name contains "read" (5) + tag exact "read" (8) + desc contains "read" (1) = 14
	// "file": name contains "file" (5) + tag exact "file" (8) + desc contains "file" (1) = 14
	// total = 28
	if results[0].Score < 20 {
		t.Fatalf("expected multi-term accumulated score >= 20 for read_file, got %f", results[0].Score)
	}
}

func TestKeywordSearcher_MaxResultsCap(t *testing.T) {
	t.Parallel()

	// Create 20 tools that all match "tool"
	var tools []Definition
	for i := 0; i < 20; i++ {
		tools = append(tools, Definition{
			Name:        "tool_" + string(rune('a'+i)),
			Description: "A tool that does something",
		})
	}

	s := &KeywordSearcher{MaxResults: 5}
	results := s.Search("tool", tools)
	if len(results) != 5 {
		t.Fatalf("expected 5 results (MaxResults cap), got %d", len(results))
	}
}

func TestKeywordSearcher_MaxResultsDefaultTen(t *testing.T) {
	t.Parallel()

	// Create 15 tools that all match "tool"
	var tools []Definition
	for i := 0; i < 15; i++ {
		tools = append(tools, Definition{
			Name:        "tool_" + string(rune('a'+i)),
			Description: "A tool that does something",
		})
	}

	// MaxResults <= 0 should default to 10
	s := &KeywordSearcher{MaxResults: 0}
	results := s.Search("tool", tools)
	if len(results) != 10 {
		t.Fatalf("expected 10 results (default cap), got %d", len(results))
	}
}

func TestKeywordSearcher_ScoreOrdering(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "unrelated", Description: "Does something with files", Tags: []string{"file"}},       // desc match only for "file": 1 + tag exact "file": 8 = 9
		{Name: "file_manager", Description: "Manage files on disk", Tags: []string{"file", "disk"}}, // name contains "file": 5 + tag exact: 8 + desc "file": 1 = 14
		{Name: "file", Description: "File operations tool", Tags: []string{"io"}},                   // exact name: 10 + desc "file": 1 = 11
	}

	results := s.Search("file", tools)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// file_manager should be first: name substring(5) + tag exact(8) + desc(1) = 14
	// file should be second: exact name(10) + desc(1) = 11
	// unrelated should be third: desc(1) + tag exact(8) = 9
	if results[0].Name != "file_manager" {
		t.Fatalf("expected file_manager first (highest score), got %q (score=%f)", results[0].Name, results[0].Score)
	}
	if results[1].Name != "file" {
		t.Fatalf("expected file second, got %q (score=%f)", results[1].Name, results[1].Score)
	}
	if results[2].Name != "unrelated" {
		t.Fatalf("expected unrelated third, got %q (score=%f)", results[2].Name, results[2].Score)
	}

	// Verify strictly descending order
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Fatalf("results not sorted descending: index %d score %f > index %d score %f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestKeywordSearcher_CaseInsensitive(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "ReadFile", Description: "Read a FILE from DISK", Tags: []string{"FILE", "Read"}},
	}

	results := s.Search("readfile", tools)
	if len(results) == 0 {
		t.Fatal("expected case-insensitive match on name")
	}
	if results[0].Name != "ReadFile" {
		t.Fatalf("expected ReadFile, got %q", results[0].Name)
	}

	results = s.Search("DISK", tools)
	if len(results) == 0 {
		t.Fatal("expected case-insensitive match on description")
	}

	results = s.Search("FILE", tools)
	if len(results) == 0 {
		t.Fatal("expected case-insensitive match on tags")
	}
}

func TestKeywordSearcher_DescriptionTruncation(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}

	longDesc := strings.Repeat("a", 300)
	tools := []Definition{
		{Name: "tool_long", Description: longDesc + " searchterm"},
	}

	results := s.Search("searchterm", tools)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Description) != 203 { // 200 chars + "..."
		t.Fatalf("expected truncated description of 203 chars, got %d", len(results[0].Description))
	}
	if !strings.HasSuffix(results[0].Description, "...") {
		t.Fatal("expected truncated description to end with '...'")
	}
}

func TestKeywordSearcher_NoMatchReturnsNil(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "read_file", Description: "Read a file", Tags: []string{"file"}},
	}

	results := s.Search("zzzznonexistent", tools)
	if results != nil {
		t.Fatalf("expected nil for no matches, got %v", results)
	}
}

func TestKeywordSearcher_TagSubstringMatch(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}
	tools := []Definition{
		{Name: "tool_a", Description: "Does something", Tags: []string{"filesystem", "operations"}},
	}

	// "file" is a substring of "filesystem" tag -> score 3
	results := s.Search("file", tools)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for tag substring match, got %d", len(results))
	}
	if results[0].Score != 3 {
		t.Fatalf("expected tag substring score of 3, got %f", results[0].Score)
	}
}

func TestScoreTool_NilTags(t *testing.T) {
	t.Parallel()
	tool := Definition{Name: "bash", Description: "Run shell commands", Tags: nil}
	score := scoreTool(tool, []string{"bash"})
	// exact name match (10) + no tag matches + description doesn't contain "bash" = 10
	if score != 10 {
		t.Fatalf("expected score 10 for exact name match with nil tags, got %f", score)
	}
}
