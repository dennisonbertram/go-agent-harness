package systemprompt

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// TestDefaultPromptCacheConfig verifies the defaults are populated correctly.
func TestDefaultPromptCacheConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()

	if !cfg.Enabled {
		t.Error("DefaultPromptCacheConfig: Enabled should be true by default")
	}
	if cfg.BoundaryMarker == "" {
		t.Error("DefaultPromptCacheConfig: BoundaryMarker should not be empty")
	}
	if len(cfg.StaticSections) == 0 {
		t.Error("DefaultPromptCacheConfig: StaticSections should not be empty")
	}
	if len(cfg.DynamicSections) == 0 {
		t.Error("DefaultPromptCacheConfig: DynamicSections should not be empty")
	}
	// Verify the expected default static sections are present.
	wantStatic := []string{"identity", "rules", "tool_behavior", "tone_style"}
	for _, want := range wantStatic {
		found := false
		for _, s := range cfg.StaticSections {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultPromptCacheConfig: StaticSections missing %q", want)
		}
	}
	// Verify the expected default dynamic sections are present.
	wantDynamic := []string{"memory", "environment", "tool_catalog", "plugins"}
	for _, want := range wantDynamic {
		found := false
		for _, s := range cfg.DynamicSections {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultPromptCacheConfig: DynamicSections missing %q", want)
		}
	}
}

// TestSplitPromptAtBoundary_WithMarker verifies a prompt splits correctly when the marker is present.
func TestSplitPromptAtBoundary_WithMarker(t *testing.T) {
	t.Parallel()
	marker := "---DYNAMIC-BOUNDARY---"
	prompt := "static content\n" + marker + "\ndynamic content"

	static, dynamic := SplitPromptAtBoundary(prompt, marker)

	if static != "static content\n" {
		t.Errorf("SplitPromptAtBoundary: static = %q, want %q", static, "static content\n")
	}
	if dynamic != "\ndynamic content" {
		t.Errorf("SplitPromptAtBoundary: dynamic = %q, want %q", dynamic, "\ndynamic content")
	}
}

// TestSplitPromptAtBoundary_NoMarker verifies entire prompt goes to static when marker is absent.
func TestSplitPromptAtBoundary_NoMarker(t *testing.T) {
	t.Parallel()
	marker := "---DYNAMIC-BOUNDARY---"
	prompt := "only static content here"

	static, dynamic := SplitPromptAtBoundary(prompt, marker)

	if static != prompt {
		t.Errorf("SplitPromptAtBoundary: static = %q, want %q", static, prompt)
	}
	if dynamic != "" {
		t.Errorf("SplitPromptAtBoundary: dynamic should be empty, got %q", dynamic)
	}
}

// TestSplitPromptAtBoundary_MarkerAtStart verifies empty static when marker is at the very beginning.
func TestSplitPromptAtBoundary_MarkerAtStart(t *testing.T) {
	t.Parallel()
	marker := "---DYNAMIC-BOUNDARY---"
	prompt := marker + "\ndynamic only"

	static, dynamic := SplitPromptAtBoundary(prompt, marker)

	if static != "" {
		t.Errorf("SplitPromptAtBoundary: static should be empty, got %q", static)
	}
	if dynamic != "\ndynamic only" {
		t.Errorf("SplitPromptAtBoundary: dynamic = %q, want %q", dynamic, "\ndynamic only")
	}
}

// TestSplitPromptAtBoundary_MarkerAtEnd verifies all content goes to static when marker is at the end.
func TestSplitPromptAtBoundary_MarkerAtEnd(t *testing.T) {
	t.Parallel()
	marker := "---DYNAMIC-BOUNDARY---"
	prompt := "all static content\n" + marker

	static, dynamic := SplitPromptAtBoundary(prompt, marker)

	if static != "all static content\n" {
		t.Errorf("SplitPromptAtBoundary: static = %q, want %q", static, "all static content\n")
	}
	if dynamic != "" {
		t.Errorf("SplitPromptAtBoundary: dynamic should be empty when marker is at end, got %q", dynamic)
	}
}

// TestJoinWithBoundary_CombinesParts verifies static + marker + dynamic output.
func TestJoinWithBoundary_CombinesParts(t *testing.T) {
	t.Parallel()
	marker := "---DYNAMIC-BOUNDARY---"
	static := "static part"
	dynamic := "dynamic part"

	result := JoinWithBoundary(static, dynamic, marker)

	expected := static + marker + dynamic
	if result != expected {
		t.Errorf("JoinWithBoundary: got %q, want %q", result, expected)
	}
	if !strings.Contains(result, marker) {
		t.Error("JoinWithBoundary: result must contain the boundary marker")
	}
}

// TestJoinWithBoundary_EmptyDynamic verifies output when dynamic part is empty.
func TestJoinWithBoundary_EmptyDynamic(t *testing.T) {
	t.Parallel()
	marker := "---DYNAMIC-BOUNDARY---"
	static := "static only"

	result := JoinWithBoundary(static, "", marker)

	// Marker must still be present even with empty dynamic section.
	if !strings.Contains(result, marker) {
		t.Errorf("JoinWithBoundary: result should contain marker even with empty dynamic, got %q", result)
	}
	if !strings.Contains(result, static) {
		t.Errorf("JoinWithBoundary: result should contain static content, got %q", result)
	}
}

// TestHashStaticContent_Deterministic verifies same input produces same hash.
func TestHashStaticContent_Deterministic(t *testing.T) {
	t.Parallel()
	content := "some static prompt content"

	h1 := HashStaticContent(content)
	h2 := HashStaticContent(content)

	if h1 != h2 {
		t.Errorf("HashStaticContent: not deterministic: %q != %q", h1, h2)
	}
	if h1 == "" {
		t.Error("HashStaticContent: returned empty hash")
	}
}

// TestHashStaticContent_DifferentInputs verifies different inputs produce different hashes.
func TestHashStaticContent_DifferentInputs(t *testing.T) {
	t.Parallel()
	h1 := HashStaticContent("content A")
	h2 := HashStaticContent("content B")

	if h1 == h2 {
		t.Errorf("HashStaticContent: different inputs produced same hash %q", h1)
	}
}

// TestHashStaticContent_EmptyString verifies empty string doesn't panic and returns a valid hash.
func TestHashStaticContent_EmptyString(t *testing.T) {
	t.Parallel()
	// Must not panic.
	h := HashStaticContent("")

	if h == "" {
		t.Error("HashStaticContent: empty string should return a valid (non-empty) hash")
	}
}

// TestClassifySection_Static verifies known static section names are classified as "static".
func TestClassifySection_Static(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()

	got := ClassifySection("identity", cfg)
	if got != "static" {
		t.Errorf("ClassifySection(identity): got %q, want %q", got, "static")
	}

	got = ClassifySection("rules", cfg)
	if got != "static" {
		t.Errorf("ClassifySection(rules): got %q, want %q", got, "static")
	}
}

// TestClassifySection_Dynamic verifies known dynamic section names are classified as "dynamic".
func TestClassifySection_Dynamic(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()

	got := ClassifySection("memory", cfg)
	if got != "dynamic" {
		t.Errorf("ClassifySection(memory): got %q, want %q", got, "dynamic")
	}

	got = ClassifySection("environment", cfg)
	if got != "dynamic" {
		t.Errorf("ClassifySection(environment): got %q, want %q", got, "dynamic")
	}
}

// TestClassifySection_Unknown verifies unrecognized section names return "unknown".
func TestClassifySection_Unknown(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()

	got := ClassifySection("custom_section", cfg)
	if got != "unknown" {
		t.Errorf("ClassifySection(custom_section): got %q, want %q", got, "unknown")
	}
}

// TestBuildCachedPrompt_OrdersCorrectly verifies static sections appear before dynamic sections in output.
func TestBuildCachedPrompt_OrdersCorrectly(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()

	sections := map[string]string{
		"memory":       "DYNAMIC_MEMORY",
		"identity":     "STATIC_IDENTITY",
		"environment":  "DYNAMIC_ENV",
		"rules":        "STATIC_RULES",
	}

	result := BuildCachedPrompt(sections, cfg)

	staticIdx := strings.Index(result.FullPrompt, "STATIC_IDENTITY")
	dynamicIdx := strings.Index(result.FullPrompt, "DYNAMIC_MEMORY")

	if staticIdx == -1 {
		t.Fatal("BuildCachedPrompt: static content (STATIC_IDENTITY) not found in FullPrompt")
	}
	if dynamicIdx == -1 {
		t.Fatal("BuildCachedPrompt: dynamic content (DYNAMIC_MEMORY) not found in FullPrompt")
	}
	if staticIdx >= dynamicIdx {
		t.Errorf("BuildCachedPrompt: static content should appear before dynamic content (staticIdx=%d, dynamicIdx=%d)", staticIdx, dynamicIdx)
	}
}

// TestBuildCachedPrompt_ContainsBoundary verifies the boundary marker is present in output.
func TestBuildCachedPrompt_ContainsBoundary(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()

	sections := map[string]string{
		"identity": "STATIC_CONTENT",
		"memory":   "DYNAMIC_CONTENT",
	}

	result := BuildCachedPrompt(sections, cfg)

	if !strings.Contains(result.FullPrompt, cfg.BoundaryMarker) {
		t.Errorf("BuildCachedPrompt: FullPrompt should contain boundary marker %q, got:\n%s", cfg.BoundaryMarker, result.FullPrompt)
	}
	// BoundaryIdx must be a valid position.
	if result.BoundaryIdx < 0 {
		t.Errorf("BuildCachedPrompt: BoundaryIdx should be >= 0, got %d", result.BoundaryIdx)
	}
}

// TestBuildCachedPrompt_Hash verifies StaticHash is non-empty and deterministic.
func TestBuildCachedPrompt_Hash(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()

	sections := map[string]string{
		"identity": "STATIC_CONTENT",
		"memory":   "DYNAMIC_CONTENT",
	}

	r1 := BuildCachedPrompt(sections, cfg)
	r2 := BuildCachedPrompt(sections, cfg)

	if r1.StaticHash == "" {
		t.Error("BuildCachedPrompt: StaticHash should not be empty")
	}
	if r1.StaticHash != r2.StaticHash {
		t.Errorf("BuildCachedPrompt: StaticHash not deterministic: %q != %q", r1.StaticHash, r2.StaticHash)
	}
	// StaticPart must match what was used to produce the hash.
	if r1.StaticPart == "" {
		t.Error("BuildCachedPrompt: StaticPart should not be empty when static sections are present")
	}
}

// TestBuildCachedPrompt_Disabled verifies no boundary marker appears when cache is disabled.
func TestBuildCachedPrompt_Disabled(t *testing.T) {
	t.Parallel()
	cfg := DefaultPromptCacheConfig()
	cfg.Enabled = false

	sections := map[string]string{
		"identity": "STATIC_CONTENT",
		"memory":   "DYNAMIC_CONTENT",
	}

	result := BuildCachedPrompt(sections, cfg)

	if strings.Contains(result.FullPrompt, cfg.BoundaryMarker) {
		t.Errorf("BuildCachedPrompt: marker should not appear when cache disabled, got:\n%s", result.FullPrompt)
	}
	// When disabled, StaticPart and DynamicPart should both be empty (no split performed).
	if result.StaticPart != "" || result.DynamicPart != "" {
		t.Errorf("BuildCachedPrompt: StaticPart=%q DynamicPart=%q should both be empty when disabled",
			result.StaticPart, result.DynamicPart)
	}
}

// rawPromptCacheLayer is a local helper struct for TOML parsing tests.
type rawPromptCacheLayer struct {
	PromptCache *rawPromptCacheConfig `toml:"prompt_cache"`
}

type rawPromptCacheConfig struct {
	Enabled         *bool     `toml:"enabled"`
	BoundaryMarker  *string   `toml:"boundary_marker"`
	StaticSections  []string  `toml:"static_sections"`
	DynamicSections []string  `toml:"dynamic_sections"`
}

// TestPromptCacheConfig_FromTOML verifies a [prompt_cache] TOML block parses correctly into PromptCacheConfig.
func TestPromptCacheConfig_FromTOML(t *testing.T) {
	t.Parallel()
	input := `
[prompt_cache]
enabled = true
boundary_marker = "---DYNAMIC-BOUNDARY---"
static_sections = ["identity", "rules", "tool_behavior", "tone_style"]
dynamic_sections = ["memory", "environment", "tool_catalog", "plugins"]
`
	var layer rawPromptCacheLayer
	if _, err := toml.Decode(input, &layer); err != nil {
		t.Fatalf("toml.Decode: %v", err)
	}
	if layer.PromptCache == nil {
		t.Fatal("TOML decode: [prompt_cache] section was nil")
	}
	pc := layer.PromptCache
	if pc.Enabled == nil || !*pc.Enabled {
		t.Error("prompt_cache.enabled should be true")
	}
	if pc.BoundaryMarker == nil || *pc.BoundaryMarker != "---DYNAMIC-BOUNDARY---" {
		t.Errorf("prompt_cache.boundary_marker: got %v", pc.BoundaryMarker)
	}
	if len(pc.StaticSections) != 4 {
		t.Errorf("prompt_cache.static_sections: got %d items, want 4", len(pc.StaticSections))
	}
	if len(pc.DynamicSections) != 4 {
		t.Errorf("prompt_cache.dynamic_sections: got %d items, want 4", len(pc.DynamicSections))
	}
}
