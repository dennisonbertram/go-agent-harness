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

func TestKeywordSearcher_StopWordFilter(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 10}

	// Tools whose names/tags contain short substrings that would spuriously match
	// stop words like "a" or "in" via substring scoring.
	tools := []Definition{
		{Name: "read", Description: "Read a file", Tags: []string{"read", "inspect", "contents"}},
		{Name: "grep", Description: "Search file contents using regex patterns", Tags: []string{"search", "grep", "regex"}},
	}

	// Query "search file contents for a pattern":
	// "a" (1 char) and "in" (2 chars) should be filtered. Without filtering:
	//   - "a" would match inside "read" name (+5) and "read" tag (+3) → +8 spurious points
	//   - "in" would match inside "inspect" tag (+3) and "contents" tag (+3) → +6 spurious points
	// With filtering, these stop words contribute nothing and grep should win
	// because "search", "contents", and "pattern" match grep's tags/description.
	results := s.Search("search file contents for a pattern", tools)
	if len(results) == 0 {
		t.Fatal("expected results for stop-word query")
	}
	if results[0].Name != "grep" {
		t.Errorf("stop-word filter failed: 'a' and 'in' should be filtered; grep should win over read, got %q (score=%.1f)",
			results[0].Name, results[0].Score)
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

// realToolDefs returns realistic tool definitions that mirror the actual tools,
// including the tags added to each Definition struct. Used by disambiguation tests.
func realToolDefs() []Definition {
	return []Definition{
		{
			Name:        "read",
			Description: "Read the contents of a file from the workspace. Returns the file text, a version hash for conflict detection, and whether the output was truncated. Use this tool when you need to inspect or review the contents of a specific, known file.",
			Tags:        []string{"read", "file", "view", "inspect", "contents"},
		},
		{
			Name:        "write",
			Description: "Write content to a workspace file. Creates the file and any parent directories if they do not exist. Use this tool to create NEW files or to completely replace the contents of an existing file. If you only need to change part of a file, prefer the edit tool instead.",
			Tags:        []string{"write", "create", "file", "replace", "new"},
		},
		{
			Name:        "edit",
			Description: "Edit (modify) a workspace file by replacing text. Use this tool to make targeted changes to existing files — replace specific strings or code blocks with new content. Requires the file to already exist; for creating new files, use the write tool instead.",
			Tags:        []string{"edit", "modify", "change", "replace", "patch", "update"},
		},
		{
			Name:        "apply_patch",
			Description: "Apply a structured patch to one or more workspace files. Use this tool for bulk, multi-file, or complex edits — NOT for single small edits (use the edit tool instead) and NOT for creating new files from scratch (use the write tool instead).",
			Tags:        []string{"patch", "multi-file", "bulk", "batch", "multiple", "files"},
		},
		{
			Name:        "grep",
			Description: "Grep (search) file contents using text or regex patterns. Recursively searches all files under the given path. Use this tool when you need to find occurrences of a string, identifier, error message, or pattern across the codebase.",
			Tags:        []string{"search", "grep", "regex", "text", "find", "contents"},
		},
		{
			Name:        "glob",
			Description: "Find files by filename pattern using glob wildcard syntax. Use this to locate files when you know part of the filename, extension, or directory name but not the exact path. Glob matches file and directory NAMES only — it does NOT search file contents.",
			Tags:        []string{"glob", "find", "files", "pattern", "names", "wildcard"},
		},
		{
			Name:        "ls",
			Description: "List files and directories in the workspace. This is the primary tool for viewing directory contents and exploring the directory tree — use it instead of running shell commands like ls or dir via bash.",
			Tags:        []string{"list", "directory", "ls", "files", "tree"},
		},
		{
			Name:        "bash",
			Description: "Run a shell command in the workspace. Use this for executing build commands, running scripts, installing packages, managing processes, and performing system operations that require shell access.",
			Tags:        []string{"bash", "shell", "command", "execute", "run", "script"},
		},
		{
			Name:        "job_output",
			Description: "Read the output of a background bash job. Use this after launching a command with bash's run_in_background=true to check its progress or retrieve results.",
			Tags:        []string{"job", "output", "background", "process", "result"},
		},
		{
			Name:        "job_kill",
			Description: "Terminate a running background bash job. Use this to stop a long-running process that was started with bash's run_in_background=true.",
			Tags:        []string{"job", "kill", "stop", "cancel", "process"},
		},
		{
			Name:        "git_status",
			Description: "Show the git status of the workspace repository. Reports staged, unstaged, modified, untracked, and deleted files. Use this tool to check whether the working tree is clean or dirty, which files have been added to the index, and which are new or pending commit.",
			Tags:        []string{"git", "status", "repository", "staged", "modified"},
		},
		{
			Name:        "git_diff",
			Description: "Show changes in the workspace git repository as a unified diff, including line-level additions, deletions, and context. Use this tool instead of running git diff via bash.",
			Tags:        []string{"git", "diff", "changes", "delta", "patch"},
		},
		{
			Name:        "fetch",
			Description: "Fetch the contents of a remote URL (HTTP or HTTPS) and return the response body as text. This tool is READ-ONLY — it does NOT save content to disk. If you need to save a downloaded file to the workspace, use the download tool instead.",
			Tags:        []string{"fetch", "http", "url", "request", "api"},
		},
		{
			Name:        "web_fetch",
			Description: "Fetch and browse the content of a single web page by URL. This is a simple wrapper that retrieves the text content of a web page. Use this tool when you need the raw page content of a specific URL.",
			Tags:        []string{"web", "fetch", "page", "url", "browse"},
		},
		{
			Name:        "web_search",
			Description: "Search the internet using a text query and return a list of results. Use this tool when you need to find information on the internet or web.",
			Tags:        []string{"search", "web", "internet", "query", "results"},
		},
		{
			Name:        "download",
			Description: "Download a file from an HTTP/HTTPS URL and save it to a workspace path. Use this tool whenever you need to save remote content to a file.",
			Tags:        []string{"download", "save", "file", "http", "url"},
		},
		{
			Name:        "cron_create",
			Description: "Create a RECURRING scheduled cron job. Cron jobs run repeatedly on a fixed schedule (e.g. every 5 minutes, every hour, daily at midnight). They are NOT one-shot timers.",
			Tags:        []string{"cron", "schedule", "recurring", "automation", "job"},
		},
		{
			Name:        "cron_list",
			Description: "List all cron jobs with their status and schedule.",
			Tags:        []string{"cron", "schedule", "list", "automation", "job"},
		},
		{
			Name:        "lsp_diagnostics",
			Description: "Run Go language-server diagnostics on a file or the entire workspace using gopls check. Returns compiler errors, type errors, unused imports, and other static-analysis findings.",
			Tags:        []string{"lsp", "diagnostics", "errors", "compiler", "code"},
		},
		{
			Name:        "sourcegraph",
			Description: "Search code across repositories using a Sourcegraph instance. Use this tool when you need to search code beyond the current workspace — for example, searching across an organization's repositories.",
			Tags:        []string{"search", "code", "repositories", "cross-repo", "sourcegraph"},
		},
	}
}

// TestKeywordSearcher_RealToolDisambiguation verifies that the searcher correctly
// picks the right tool when given realistic queries against the actual tool set.
// Each subtest documents a disambiguation that could fail without proper tags.
func TestKeywordSearcher_RealToolDisambiguation(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 20}
	defs := realToolDefs()

	cases := []struct {
		query   string
		want    string
		comment string
	}{
		{
			// grep has "search", "text", "regex", "contents" tags; single-char "a" is filtered as stop word
			query:   "search file contents for a pattern",
			want:    "grep",
			comment: "grep must beat read/glob: 'search'+'contents'+'pattern' match grep tags; 'a' is filtered as stop word",
		},
		{
			// glob has "find", "files", "pattern" tags; grep has "find" but not "files" or "pattern" as tags
			query:   "find files matching *.go",
			want:    "glob",
			comment: "glob must beat grep because 'files' and 'pattern' are glob-specific tags",
		},
		{
			// ls has "list" and "directory" tags; glob has neither
			query:   "list directory",
			want:    "ls",
			comment: "ls must win on 'list directory' — those are exact ls tags",
		},
		{
			// edit has "modify" tag; write and read do not
			query:   "modify existing code in a file",
			want:    "edit",
			comment: "edit must win on 'modify' — it is an exact tag match unique to edit",
		},
		{
			// write has "create", "new", "replace" tags; edit has "replace" but not "create"/"new"
			query:   "replace entire file content",
			want:    "write",
			comment: "write must win because 'replace' tag + 'file' tag give it higher score than edit",
		},
		{
			// apply_patch has "multiple" and "files" tags; glob also has "files" but not "multiple"
			query:   "apply changes to multiple files at once",
			want:    "apply_patch",
			comment: "apply_patch must win: name contains 'apply', 'multiple'+'files' tags unique to it",
		},
		{
			// git_diff has "changes" tag; git_status does not
			query:   "show git changes",
			want:    "git_diff",
			comment: "git_diff must win on 'changes' tag which git_status lacks",
		},
		{
			// git_status has "status" and "repository" tags; git_diff has neither
			query:   "show repository status",
			want:    "git_status",
			comment: "git_status must win on 'status'+'repository' tags unique to it",
		},
		{
			// fetch: name exact match (10) + fetch tag (8) + url tag (8) beats web_fetch and download
			query:   "fetch a URL",
			want:    "fetch",
			comment: "fetch must rank first: exact name match + fetch+url tags",
		},
		{
			// download has "save" tag unique to it; fetch/web_fetch do not
			query:   "save a URL to disk",
			want:    "download",
			comment: "download must win on 'save' tag which fetch and web_fetch lack",
		},
		{
			// web_search has "internet" tag; grep has "search" but not "internet"
			query:   "search the internet",
			want:    "web_search",
			comment: "web_search must win: 'internet' tag + name substring 'search'",
		},
		{
			// bash has "shell", "command", "run", "execute" tags all matching
			query:   "run a shell command",
			want:    "bash",
			comment: "bash must win: 'shell'+'command'+'run' are all exact bash tags",
		},
		{
			// cron_create has "schedule", "recurring", "job" tags; cron_list has "schedule"+"job" but not "recurring"
			query:   "schedule a recurring job",
			want:    "cron_create",
			comment: "cron_create must beat cron_list because 'recurring' tag is unique to cron_create",
		},
		{
			// lsp_diagnostics has "errors", "compiler", "code" tags matching
			query:   "check code for errors",
			want:    "lsp_diagnostics",
			comment: "lsp_diagnostics must win: 'errors'+'code' tags match query terms uniquely",
		},
		{
			// sourcegraph has "repositories" and "search" tags; grep has "search" but not "repositories"
			query:   "search across code repositories",
			want:    "sourcegraph",
			comment: "sourcegraph must win: 'repositories'+'code'+'search' tags all match",
		},
		{
			// read: exact name match (10) + "read"+"file" tags; write/edit do not have "read" tag
			query:   "read a file",
			want:    "read",
			comment: "read must win: exact name match + read+file tags unique to it",
		},
		{
			// write has "create" and "new" tags; edit does not
			query:   "create a new file",
			want:    "write",
			comment: "write must win: 'create'+'new'+'file' are all exact write tags",
		},
		{
			// job_output has "output", "background" tags; job_kill has neither
			query:   "background job output",
			want:    "job_output",
			comment: "job_output must win: 'background'+'output' are exact tags unique to it",
		},
		{
			// job_kill has "kill", "stop", "cancel" tags; job_output has none of those
			query:   "kill a running process",
			want:    "job_kill",
			comment: "job_kill must win: 'kill' is an exact tag unique to it",
		},
		{
			// web_search has "search"+"web" name prefix + "internet"+"query"+"results" tags
			query:   "search web for information",
			want:    "web_search",
			comment: "web_search must win: name contains 'search'+'web' + 'search' tag",
		},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()
			results := s.Search(tc.query, defs)
			if len(results) == 0 {
				t.Fatalf("query %q returned no results; expected %q first", tc.query, tc.want)
			}
			if results[0].Name != tc.want {
				// Print top 3 for debugging
				top := results
				if len(top) > 3 {
					top = top[:3]
				}
				t.Errorf("query %q: want %q first, got %q (score=%.1f)\n  comment: %s\n  top results: %v",
					tc.query, tc.want, results[0].Name, results[0].Score, tc.comment, top)
			}
		})
	}
}

// TestKeywordSearcher_RealToolNegative verifies that the wrong tools do NOT
// appear at the top when a query clearly belongs to a different tool.
func TestKeywordSearcher_RealToolNegative(t *testing.T) {
	t.Parallel()
	s := &KeywordSearcher{MaxResults: 20}
	defs := realToolDefs()

	t.Run("find files should not return grep first", func(t *testing.T) {
		t.Parallel()
		// glob must beat grep because "files" is a glob tag and grep doesn't have it
		results := s.Search("find files", defs)
		if len(results) == 0 {
			t.Fatal("expected results for 'find files'")
		}
		if results[0].Name == "grep" {
			t.Errorf("'find files' should NOT return grep first; glob should win. Got: %v", results[0])
		}
		if results[0].Name != "glob" {
			t.Errorf("'find files' should return glob first, got %q (score=%.1f)", results[0].Name, results[0].Score)
		}
	})

	t.Run("search text should not return glob first", func(t *testing.T) {
		t.Parallel()
		// grep must beat glob because "text" is a grep tag; glob has no "search" or "text" tag
		results := s.Search("search text", defs)
		if len(results) == 0 {
			t.Fatal("expected results for 'search text'")
		}
		if results[0].Name == "glob" {
			t.Errorf("'search text' should NOT return glob first; grep should win. Got: %v", results[0])
		}
		if results[0].Name != "grep" {
			t.Errorf("'search text' should return grep first, got %q (score=%.1f)", results[0].Name, results[0].Score)
		}
	})
}
