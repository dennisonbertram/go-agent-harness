package causalgraph

import (
	"sort"
	"testing"
)

func TestExtractTokens_Basic(t *testing.T) {
	t.Parallel()
	tokens := ExtractTokens("hello world important_value foobar baz")
	// "hello" = 5 chars (< 6, excluded), "world" = 5 chars (< 6, excluded)
	// "important_value" = 15 chars (included), "foobar" = 6 chars (included)
	// "baz" = 3 chars (< 6, excluded)
	sort.Strings(tokens)
	expected := []string{"foobar", "important_value"}
	if len(tokens) != len(expected) {
		t.Fatalf("ExtractTokens got %v, want %v", tokens, expected)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("tokens[%d] = %q, want %q", i, tok, expected[i])
		}
	}
}

func TestExtractTokens_Empty(t *testing.T) {
	t.Parallel()
	tokens := ExtractTokens("")
	if len(tokens) != 0 {
		t.Errorf("expected empty, got %v", tokens)
	}
}

func TestExtractTokens_ShortTokensOnly(t *testing.T) {
	t.Parallel()
	tokens := ExtractTokens("a bb ccc dddd eeeee")
	if len(tokens) != 0 {
		t.Errorf("expected empty (all tokens < 6 chars), got %v", tokens)
	}
}

func TestExtractTokens_Stopwords(t *testing.T) {
	t.Parallel()
	// Common stopwords should be excluded even if >= 6 chars
	tokens := ExtractTokens("should return string result")
	// "should" = 6, "return" = 6, "string" = 6, "result" = 6
	// "should" and "return" and "string" and "result" are stopwords
	for _, tok := range tokens {
		if isStopword(tok) {
			t.Errorf("stopword %q should have been excluded", tok)
		}
	}
}

func TestExtractTokens_Deduplication(t *testing.T) {
	t.Parallel()
	tokens := ExtractTokens("foobar foobar foobar")
	if len(tokens) != 1 {
		t.Errorf("expected 1 unique token, got %d: %v", len(tokens), tokens)
	}
}

func TestExtractTokens_Punctuation(t *testing.T) {
	t.Parallel()
	tokens := ExtractTokens("important_value, another-token. foobar!")
	// After splitting on whitespace, should strip punctuation
	found := make(map[string]bool)
	for _, tok := range tokens {
		found[tok] = true
	}
	if !found["important_value"] {
		t.Error("expected important_value in tokens")
	}
	if !found["foobar"] {
		t.Error("expected foobar in tokens")
	}
}

func TestFindDataFlowEdges_Basic(t *testing.T) {
	t.Parallel()
	results := map[string]string{
		"call-1": "the important_value is here",
	}
	args := map[string]string{
		"call-2": `{"content":"use important_value now"}`,
	}
	ordering := []string{"call-1", "call-2"}

	edges := FindDataFlowEdges(results, args, ordering)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d: %+v", len(edges), edges)
	}
	e := edges[0]
	if e.From != "call-1" || e.To != "call-2" {
		t.Errorf("edge = %s -> %s, want call-1 -> call-2", e.From, e.To)
	}
	if e.Type != EdgeTypeDataFlow {
		t.Errorf("edge type = %q, want %q", e.Type, EdgeTypeDataFlow)
	}
	// HIGH-4 fix: MatchedToken now stores a fingerprint (sha256[:16]+len),
	// not the raw token, to prevent exfiltrating secrets via forensic outputs.
	wantToken := tokenFingerprint("important_value")
	if e.MatchedToken != wantToken {
		t.Errorf("matched token = %q, want %q", e.MatchedToken, wantToken)
	}
}

func TestFindDataFlowEdges_Empty(t *testing.T) {
	t.Parallel()
	edges := FindDataFlowEdges(nil, nil, nil)
	if len(edges) != 0 {
		t.Errorf("expected empty, got %v", edges)
	}
}

func TestFindDataFlowEdges_NoMatch(t *testing.T) {
	t.Parallel()
	results := map[string]string{
		"call-1": "unique_output_value",
	}
	args := map[string]string{
		"call-2": `{"path":"something_else_entirely"}`,
	}
	ordering := []string{"call-1", "call-2"}

	edges := FindDataFlowEdges(results, args, ordering)
	if len(edges) != 0 {
		t.Errorf("expected no edges, got %d: %+v", len(edges), edges)
	}
}

func TestFindDataFlowEdges_OnlyForwardEdges(t *testing.T) {
	t.Parallel()
	// call-2's result contains a token that appears in call-1's args
	// but call-2 comes AFTER call-1, so no reverse edge should be created
	results := map[string]string{
		"call-1": "alpha_result",
		"call-2": "call_one_args_value",
	}
	args := map[string]string{
		"call-1": `{"key":"call_one_args_value"}`,
		"call-2": `{"key":"nothing_special"}`,
	}
	ordering := []string{"call-1", "call-2"}

	edges := FindDataFlowEdges(results, args, ordering)
	// call-2's result "call_one_args_value" should NOT match back to call-1's args
	for _, e := range edges {
		if e.From == "call-2" && e.To == "call-1" {
			t.Error("should not create backward edges (call-2 -> call-1)")
		}
	}
}

func TestFindDataFlowEdges_MultipleTokenMatches_OnlyFirstPerPair(t *testing.T) {
	t.Parallel()
	results := map[string]string{
		"call-1": "token_alpha token_beta token_gamma",
	}
	args := map[string]string{
		"call-2": `{"a":"token_alpha","b":"token_beta"}`,
	}
	ordering := []string{"call-1", "call-2"}

	edges := FindDataFlowEdges(results, args, ordering)
	// Should get exactly 1 edge from call-1 to call-2 (deduplicated per pair)
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge (deduplicated per pair), got %d: %+v", len(edges), edges)
	}
}

func TestFindDataFlowEdges_ChainedDataFlow(t *testing.T) {
	t.Parallel()
	results := map[string]string{
		"call-1": "intermediate_value",
		"call-2": "final_output",
	}
	args := map[string]string{
		"call-1": `{}`,
		"call-2": `{"input":"intermediate_value"}`,
		"call-3": `{"input":"final_output"}`,
	}
	ordering := []string{"call-1", "call-2", "call-3"}

	edges := FindDataFlowEdges(results, args, ordering)
	// Should have call-1->call-2 (intermediate_value) and call-2->call-3 (final_output)
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d: %+v", len(edges), edges)
	}
}
