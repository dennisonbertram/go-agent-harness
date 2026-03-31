// regression_test.go contains regression tests for the memory package.
// These tests cover integration points and edge cases that catch future
// breakage if the core behavior is reverted or regressed.
package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTaxonomySize_RegressionFour ensures the taxonomy never silently shrinks.
// If a type is accidentally removed, this test fails immediately.
func TestTaxonomySize_RegressionFour(t *testing.T) {
	t.Parallel()
	all := AllMemoryTypes()
	if len(all) != 4 {
		t.Errorf("taxonomy size = %d, want exactly 4; if you added a type, update this test", len(all))
	}
}

// TestIndexRoundtrip_Regression verifies that SaveIndex→LoadIndex preserves
// all entries faithfully. If the serialization format changes and breaks
// parsing, this test fails.
func TestIndexRoundtrip_Regression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")

	original := &Index{
		Path: path,
		Entries: []IndexEntry{
			{Name: "alpha", Type: MemoryTypeUser, Description: "user alpha", FilePath: "alpha.md"},
			{Name: "beta", Type: MemoryTypeFeedback, Description: "feedback beta", FilePath: "beta.md"},
			{Name: "gamma", Type: MemoryTypeProject, Description: "project gamma", FilePath: "gamma.md"},
			{Name: "delta", Type: MemoryTypeReference, Description: "ref delta", FilePath: "delta.md"},
		},
	}

	cfg := DefaultMemoryConfig()
	if err := original.SaveIndex(cfg); err != nil {
		t.Fatalf("SaveIndex() error = %v", err)
	}

	loaded, err := LoadIndex(path)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}

	if len(loaded.Entries) != len(original.Entries) {
		t.Fatalf("roundtrip: got %d entries, want %d", len(loaded.Entries), len(original.Entries))
	}

	for i, orig := range original.Entries {
		got := loaded.Entries[i]
		if got.Name != orig.Name {
			t.Errorf("[%d] Name: got %q, want %q", i, got.Name, orig.Name)
		}
		if got.Type != orig.Type {
			t.Errorf("[%d] Type: got %q, want %q", i, got.Type, orig.Type)
		}
		if got.Description != orig.Description {
			t.Errorf("[%d] Description: got %q, want %q", i, got.Description, orig.Description)
		}
	}
}

// TestCapEnforcement_Regression ensures that AddEntry never allows the index
// to persist in an over-cap state after the add. If TrimToFit is accidentally
// removed from AddEntry, this test catches it.
func TestCapEnforcement_Regression(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	cfg.IndexMaxLines = 8 // header(3) + ~5 data rows → forces trimming before 5

	idx := &Index{}
	for i := range 10 {
		e := IndexEntry{
			Name:        "entry-" + string(rune('a'+i)),
			Type:        MemoryTypeProject,
			Description: "desc",
			FilePath:    "file.md",
		}
		if err := idx.AddEntry(e, cfg); err != nil {
			t.Fatalf("AddEntry(%d): %v", i, err)
		}
	}

	if idx.IsOverCap(cfg) {
		t.Errorf("after 10 AddEntry calls with cap=%d lines, index is still over cap", cfg.IndexMaxLines)
	}
}

// TestDriftProtectionText_NoBulletPoint_Regression verifies drift protection
// text is always rendered as a section header (## level) and never degrades to
// a bullet point. Eval shows 0/3 pass rate for bullets vs 3/3 for headers.
func TestDriftProtectionText_NoBulletPoint_Regression(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	text := DriftProtectionText(cfg)

	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "*") || strings.HasPrefix(text, "•") {
		t.Errorf("DriftProtectionText starts with a bullet character; must be a ## section header")
	}
	if !strings.HasPrefix(text, "## ") {
		t.Errorf("DriftProtectionText does not start with '## '; got: %q", text[:min(len(text), 20)])
	}
}

// TestTopicFile_FrontmatterSeparation_Regression ensures that the body content
// of a topic file is properly separated from the YAML frontmatter. If the
// separator logic breaks, body content bleeds into the frontmatter or is lost.
func TestTopicFile_FrontmatterSeparation_Regression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sep-test.md")

	// This body starts with dashes which could confuse a naive parser
	body := "---\nThis line starts with dashes but is part of the body.\n\n## Section\n\nMore content.\n"
	raw := "---\nname: sep-test\ntype: project\ndescription: separator test\n---\n" + body
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}

	tf, err := LoadTopicFile(path)
	if err != nil {
		t.Fatalf("LoadTopicFile() error = %v", err)
	}
	if tf.Entry.Name != "sep-test" {
		t.Errorf("Entry.Name = %q, want 'sep-test'", tf.Entry.Name)
	}
	if tf.Content != body {
		t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", tf.Content, body)
	}
}

// TestListTopicFiles_ExcludesNonMd_Regression verifies that ListTopicFiles
// never returns non-.md files. If the filter is removed, the harness might
// attempt to parse binary or config files as topic files.
func TestListTopicFiles_ExcludesNonMd_Regression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	nonMd := []string{"notes.txt", "config.toml", "data.json", ".hidden"}
	mdFiles := []string{"topic1.md", "topic2.md"}

	for _, f := range append(nonMd, mdFiles...) {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := ListTopicFiles(dir)
	if err != nil {
		t.Fatalf("ListTopicFiles() error = %v", err)
	}
	if len(files) != len(mdFiles) {
		t.Errorf("ListTopicFiles() = %d files, want %d (only .md)", len(files), len(mdFiles))
	}
	for _, f := range files {
		if filepath.Ext(f) != ".md" {
			t.Errorf("ListTopicFiles() returned non-.md file: %q", f)
		}
	}
}
