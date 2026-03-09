package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// mockTracker implements ActivationTrackerInterface for testing.
type mockTracker struct {
	mu        sync.Mutex
	activated map[string]map[string]bool // runID -> toolName -> true
}

func newMockTracker() *mockTracker {
	return &mockTracker{activated: make(map[string]map[string]bool)}
}

func (m *mockTracker) Activate(runID string, toolNames ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activated[runID] == nil {
		m.activated[runID] = make(map[string]bool)
	}
	for _, name := range toolNames {
		m.activated[runID][name] = true
	}
}

func (m *mockTracker) IsActive(runID string, toolName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activated[runID][toolName]
}

func (m *mockTracker) activatedTools(runID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for name := range m.activated[runID] {
		names = append(names, name)
	}
	return names
}

// testDeferredDefs creates a set of deferred tool definitions for testing.
func testDeferredDefs() []Definition {
	return []Definition{
		{
			Name:        "cron_create",
			Description: "Create a recurring scheduled cron job",
			Tier:        TierDeferred,
			Tags:        []string{"scheduling", "cron", "automation"},
		},
		{
			Name:        "cron_list",
			Description: "List all scheduled cron jobs",
			Tier:        TierDeferred,
			Tags:        []string{"scheduling", "cron"},
		},
		{
			Name:        "web_search",
			Description: "Search the web for information",
			Tier:        TierDeferred,
			Tags:        []string{"web", "search", "browsing"},
		},
		{
			Name:        "lsp_diagnostics",
			Description: "Get LSP diagnostics for code analysis",
			Tier:        TierDeferred,
			Tags:        []string{"code", "lsp", "analysis"},
		},
		{
			Name:        "sourcegraph_search",
			Description: "Search code using Sourcegraph",
			Tier:        TierDeferred,
			Tags:        []string{"code", "search", "sourcegraph"},
		},
	}
}

func findToolCtx(runID string) context.Context {
	return context.WithValue(context.Background(), ContextKeyRunID, runID)
}

func callFindTool(t *testing.T, tool Tool, ctx context.Context, query string) map[string]any {
	t.Helper()
	args, err := json.Marshal(findToolArgs{Query: query})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	result, err := tool.Handler(ctx, json.RawMessage(args))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return parsed
}

func TestFindTool_KeywordSearch(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-keyword-1")
	result := callFindTool(t, tool, ctx, "cron scheduling")

	// Should find cron tools
	msg, ok := result["message"].(string)
	if !ok || !strings.Contains(msg, "activated") {
		t.Fatalf("expected activation message, got %v", result["message"])
	}

	activated, ok := result["activated"].([]any)
	if !ok || len(activated) == 0 {
		t.Fatalf("expected activated tools, got %v", result["activated"])
	}

	// Verify cron_create was found (exact tag match for "cron" and "scheduling")
	foundCronCreate := false
	for _, name := range activated {
		if name == "cron_create" {
			foundCronCreate = true
		}
	}
	if !foundCronCreate {
		t.Fatalf("expected cron_create in activated tools, got %v", activated)
	}

	// Verify tracker was updated
	if !tracker.IsActive("run-keyword-1", "cron_create") {
		t.Fatal("expected cron_create to be active in tracker")
	}
}

func TestFindTool_DirectSelect(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-select-1")
	result := callFindTool(t, tool, ctx, "select:web_search")

	msg, ok := result["message"].(string)
	if !ok || !strings.Contains(msg, "web_search") || !strings.Contains(msg, "activated") {
		t.Fatalf("expected activation message for web_search, got %v", result["message"])
	}

	activated, ok := result["activated"].([]any)
	if !ok || len(activated) != 1 {
		t.Fatalf("expected exactly 1 activated tool, got %v", result["activated"])
	}
	if activated[0] != "web_search" {
		t.Fatalf("expected web_search, got %v", activated[0])
	}

	if !tracker.IsActive("run-select-1", "web_search") {
		t.Fatal("expected web_search to be active in tracker")
	}
}

func TestFindTool_DirectSelectNotFound(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-notfound-1")
	result := callFindTool(t, tool, ctx, "select:nonexistent_tool")

	errMsg, ok := result["error"].(string)
	if !ok || !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' error, got %v", result["error"])
	}

	hint, ok := result["hint"].(string)
	if !ok || hint == "" {
		t.Fatalf("expected hint in response, got %v", result["hint"])
	}

	// Verify nothing was activated
	tools := tracker.activatedTools("run-notfound-1")
	if len(tools) != 0 {
		t.Fatalf("expected no tools activated, got %v", tools)
	}
}

func TestFindTool_EmptyQuery(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-empty-1")
	result := callFindTool(t, tool, ctx, "")

	errMsg, ok := result["error"].(string)
	if !ok || errMsg != "query is required" {
		t.Fatalf("expected 'query is required' error, got %v", result["error"])
	}
}

func TestFindTool_NoResults(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-noresults-1")
	result := callFindTool(t, tool, ctx, "zzzzzzunmatched")

	msg, ok := result["message"].(string)
	if !ok || !strings.Contains(msg, "No matching tools found") {
		t.Fatalf("expected 'No matching tools found' message, got %v", result["message"])
	}

	query, ok := result["query"].(string)
	if !ok || query != "zzzzzzunmatched" {
		t.Fatalf("expected query echoed back, got %v", result["query"])
	}

	// Verify nothing was activated
	tools := tracker.activatedTools("run-noresults-1")
	if len(tools) != 0 {
		t.Fatalf("expected no tools activated, got %v", tools)
	}
}

func TestFindTool_SelectEmptyName(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-emptyselect-1")
	result := callFindTool(t, tool, ctx, "select:")

	errMsg, ok := result["error"].(string)
	if !ok || !strings.Contains(errMsg, "tool name is required") {
		t.Fatalf("expected 'tool name is required' error, got %v", result["error"])
	}
}

func TestFindTool_NoRunContext(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	// Use context without runID
	ctx := context.Background()
	args, err := json.Marshal(findToolArgs{Query: "cron"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	_, err = tool.Handler(ctx, json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error when no run context")
	}
	if !strings.Contains(err.Error(), "run context") {
		t.Fatalf("expected 'run context' in error, got %v", err)
	}
}

func TestFindTool_InvalidJSON(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-badjson-1")
	_, err := tool.Handler(ctx, json.RawMessage(`{invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid find_tool arguments") {
		t.Fatalf("expected 'invalid find_tool arguments' in error, got %v", err)
	}
}

func TestFindTool_DefinitionFields(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	if tool.Definition.Name != "find_tool" {
		t.Fatalf("expected name 'find_tool', got %q", tool.Definition.Name)
	}
	if tool.Definition.Tier != TierCore {
		t.Fatalf("expected TierCore, got %q", tool.Definition.Tier)
	}
	if tool.Definition.Description == "" {
		t.Fatal("expected non-empty description")
	}
	if tool.Handler == nil {
		t.Fatal("expected non-nil handler")
	}

	// Description should include deferred tool names from the catalog
	desc := tool.Definition.Description
	for _, def := range defs {
		if !strings.Contains(desc, def.Name) {
			t.Errorf("expected description to contain tool name %q", def.Name)
		}
	}
}

func TestBuildFindToolDescription_IncludesTools(t *testing.T) {
	t.Parallel()

	defs := testDeferredDefs()
	desc := buildFindToolDescription(defs)

	// Should contain the base description text
	if !strings.Contains(desc, "Search for and activate additional tools") {
		t.Fatal("expected base description text in output")
	}

	// Should contain the catalog header
	if !strings.Contains(desc, "Available tools (use select:<name> to activate):") {
		t.Fatal("expected catalog header in output")
	}

	// Should contain each tool name and a summary from its description
	for _, def := range defs {
		if !strings.Contains(desc, "- "+def.Name+":") {
			t.Errorf("expected catalog entry for %q", def.Name)
		}
		// The summary should include at least part of the description
		summary := firstSentence(def.Description, 100)
		if !strings.Contains(desc, summary) {
			t.Errorf("expected summary %q for tool %q in description", summary, def.Name)
		}
	}
}

func TestBuildFindToolDescription_NoDefs(t *testing.T) {
	t.Parallel()

	desc := buildFindToolDescription(nil)

	// With no deferred defs, should just be the base description
	if strings.Contains(desc, "Available tools") {
		t.Fatal("expected no catalog section when no deferred defs")
	}
	if !strings.Contains(desc, "Search for and activate additional tools") {
		t.Fatal("expected base description text")
	}
}

func TestFirstSentence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "simple sentence",
			input:  "Create a recurring scheduled cron job. It supports complex expressions.",
			maxLen: 100,
			want:   "Create a recurring scheduled cron job.",
		},
		{
			name:   "no period",
			input:  "Search the web for information",
			maxLen: 100,
			want:   "Search the web for information",
		},
		{
			name:   "period at end of string",
			input:  "List all scheduled cron jobs.",
			maxLen: 100,
			want:   "List all scheduled cron jobs.",
		},
		{
			name:   "truncated at maxLen",
			input:  "This is a very long description that goes on and on without a period ending anywhere soon enough",
			maxLen: 30,
			want:   "This is a very long descrip...",
		},
		{
			name:   "sentence longer than maxLen",
			input:  "This is a very long first sentence that should be truncated. Second sentence.",
			maxLen: 30,
			want:   "This is a very long first s...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 100,
			want:   "",
		},
		{
			name:   "whitespace only",
			input:  "   ",
			maxLen: 100,
			want:   "",
		},
		{
			name:   "period followed by space stops at sentence",
			input:  "Use e.g. this tool for work",
			maxLen: 100,
			want:   "Use e.g.",
		},
		{
			name:   "period followed by space mid-string",
			input:  "First sentence here. Second sentence here.",
			maxLen: 100,
			want:   "First sentence here.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := firstSentence(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("firstSentence(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFindTool_WhitespaceOnlyQuery(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-whitespace-1")
	result := callFindTool(t, tool, ctx, "   ")

	errMsg, ok := result["error"].(string)
	if !ok || errMsg != "query is required" {
		t.Fatalf("expected 'query is required' error for whitespace query, got %v", result["error"])
	}
}

func TestFindTool_SelectWithWhitespace(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-selectws-1")
	// Extra whitespace around tool name should be trimmed
	result := callFindTool(t, tool, ctx, "select:  lsp_diagnostics  ")

	msg, ok := result["message"].(string)
	if !ok || !strings.Contains(msg, "lsp_diagnostics") {
		t.Fatalf("expected activation message for lsp_diagnostics, got %v", result["message"])
	}

	if !tracker.IsActive("run-selectws-1", "lsp_diagnostics") {
		t.Fatal("expected lsp_diagnostics to be active in tracker")
	}
}

func TestFindTool_MultipleSearchResults(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	ctx := findToolCtx("run-multi-1")
	// "code" should match lsp_diagnostics and sourcegraph_search
	result := callFindTool(t, tool, ctx, "code")

	activated, ok := result["activated"].([]any)
	if !ok || len(activated) < 2 {
		t.Fatalf("expected at least 2 activated tools for 'code' query, got %v", result["activated"])
	}

	// Both code-related tools should be activated
	if !tracker.IsActive("run-multi-1", "lsp_diagnostics") {
		t.Fatal("expected lsp_diagnostics to be active")
	}
	if !tracker.IsActive("run-multi-1", "sourcegraph_search") {
		t.Fatal("expected sourcegraph_search to be active")
	}
}

func TestFindTool_RunMetadataContext(t *testing.T) {
	t.Parallel()

	tracker := newMockTracker()
	defs := testDeferredDefs()
	searcher := &KeywordSearcher{MaxResults: 10}
	tool := FindToolTool(searcher, defs, tracker)

	// Use RunMetadata instead of plain ContextKeyRunID
	meta := RunMetadata{RunID: "run-meta-1", TenantID: "t1"}
	ctx := context.WithValue(context.Background(), ContextKeyRunMetadata, meta)
	result := callFindTool(t, tool, ctx, "select:cron_create")

	msg, ok := result["message"].(string)
	if !ok || !strings.Contains(msg, "cron_create") {
		t.Fatalf("expected activation message, got %v", result["message"])
	}

	if !tracker.IsActive("run-meta-1", "cron_create") {
		t.Fatal("expected cron_create to be active via RunMetadata context")
	}
}
