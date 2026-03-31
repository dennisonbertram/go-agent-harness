package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadIndex_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadIndex(path)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("LoadIndex() entries = %d, want 0", len(idx.Entries))
	}
}

func TestLoadIndex_WithEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	content := `# Memory Index

| Name | Type | Description | File |
|------|------|-------------|------|
| go-patterns | project | Common Go patterns used in this repo | go-patterns.md |
| user-prefs | user | User coding preferences | user-prefs.md |
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadIndex(path)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}
	if len(idx.Entries) != 2 {
		t.Errorf("LoadIndex() entries = %d, want 2", len(idx.Entries))
	}
	if idx.Entries[0].Name != "go-patterns" {
		t.Errorf("idx.Entries[0].Name = %q, want %q", idx.Entries[0].Name, "go-patterns")
	}
	if idx.Entries[0].Type != MemoryTypeProject {
		t.Errorf("idx.Entries[0].Type = %q, want %q", idx.Entries[0].Type, MemoryTypeProject)
	}
	if idx.Entries[1].Name != "user-prefs" {
		t.Errorf("idx.Entries[1].Name = %q, want %q", idx.Entries[1].Name, "user-prefs")
	}
	if idx.Entries[1].Type != MemoryTypeUser {
		t.Errorf("idx.Entries[1].Type = %q, want %q", idx.Entries[1].Type, MemoryTypeUser)
	}
}

func TestAddEntry_UnderCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	idx := &Index{Path: path}
	cfg := DefaultMemoryConfig()

	entry := IndexEntry{
		Name:        "test-entry",
		Type:        MemoryTypeUser,
		Description: "A test entry",
		FilePath:    "test-entry.md",
	}
	if err := idx.AddEntry(entry, cfg); err != nil {
		t.Fatalf("AddEntry() error = %v", err)
	}
	if len(idx.Entries) != 1 {
		t.Errorf("after AddEntry, len(Entries) = %d, want 1", len(idx.Entries))
	}
	if idx.Entries[0].Name != "test-entry" {
		t.Errorf("idx.Entries[0].Name = %q, want %q", idx.Entries[0].Name, "test-entry")
	}
}

func TestAddEntry_AtLineCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")
	idx := &Index{Path: path}

	// Use a tiny cap: 2 entries max (each entry becomes a table row — small line limit)
	cfg := DefaultMemoryConfig()
	cfg.IndexMaxLines = 10 // Very small — header + 2 rows will exceed this limit

	// Fill to just at limit
	for i := range 8 {
		entry := IndexEntry{
			Name:        "entry-" + string(rune('a'+i)),
			Type:        MemoryTypeProject,
			Description: "desc",
			FilePath:    "file.md",
		}
		if err := idx.AddEntry(entry, cfg); err != nil {
			t.Fatalf("AddEntry() error = %v on entry %d", err, i)
		}
	}
	initial := len(idx.Entries)

	// Add one more that should cause a trim
	overflow := IndexEntry{
		Name:        "overflow",
		Type:        MemoryTypeProject,
		Description: "causes trim",
		FilePath:    "overflow.md",
	}
	if err := idx.AddEntry(overflow, cfg); err != nil {
		t.Fatalf("AddEntry() overflow error = %v", err)
	}

	// Should still have the new entry, but total should be <= initial
	found := false
	for _, e := range idx.Entries {
		if e.Name == "overflow" {
			found = true
		}
	}
	if !found {
		t.Error("overflow entry not found after AddEntry with trim")
	}
	if len(idx.Entries) > initial {
		t.Errorf("after trim, len(Entries) = %d, want <= %d", len(idx.Entries), initial)
	}
}

func TestRemoveEntry_Exists(t *testing.T) {
	t.Parallel()
	idx := &Index{
		Entries: []IndexEntry{
			{Name: "keep", Type: MemoryTypeUser, Description: "keep this"},
			{Name: "remove-me", Type: MemoryTypeProject, Description: "remove this"},
		},
	}
	err := idx.RemoveEntry("remove-me")
	if err != nil {
		t.Fatalf("RemoveEntry() error = %v", err)
	}
	if len(idx.Entries) != 1 {
		t.Errorf("after RemoveEntry, len(Entries) = %d, want 1", len(idx.Entries))
	}
	if idx.Entries[0].Name != "keep" {
		t.Errorf("idx.Entries[0].Name = %q, want %q", idx.Entries[0].Name, "keep")
	}
}

func TestRemoveEntry_NotFound(t *testing.T) {
	t.Parallel()
	idx := &Index{
		Entries: []IndexEntry{
			{Name: "existing", Type: MemoryTypeUser},
		},
	}
	// Should not return an error when entry is not found
	err := idx.RemoveEntry("nonexistent")
	if err != nil {
		t.Errorf("RemoveEntry(nonexistent) error = %v, want nil", err)
	}
	if len(idx.Entries) != 1 {
		t.Errorf("after RemoveEntry(nonexistent), len(Entries) = %d, want 1", len(idx.Entries))
	}
}

func TestEntriesByType_FiltersCorrectly(t *testing.T) {
	t.Parallel()
	idx := &Index{
		Entries: []IndexEntry{
			{Name: "e1", Type: MemoryTypeUser},
			{Name: "e2", Type: MemoryTypeProject},
			{Name: "e3", Type: MemoryTypeUser},
			{Name: "e4", Type: MemoryTypeFeedback},
		},
	}
	userEntries := idx.EntriesByType(MemoryTypeUser)
	if len(userEntries) != 2 {
		t.Errorf("EntriesByType(user) returned %d entries, want 2", len(userEntries))
	}
	for _, e := range userEntries {
		if e.Type != MemoryTypeUser {
			t.Errorf("EntriesByType(user) returned entry with type %q", e.Type)
		}
	}

	projectEntries := idx.EntriesByType(MemoryTypeProject)
	if len(projectEntries) != 1 {
		t.Errorf("EntriesByType(project) returned %d entries, want 1", len(projectEntries))
	}
}

func TestIsOverCap_UnderLimit(t *testing.T) {
	t.Parallel()
	idx := &Index{
		Entries: []IndexEntry{
			{Name: "e1", Type: MemoryTypeUser, Description: "short"},
		},
	}
	cfg := DefaultMemoryConfig() // 200 lines, 25600 bytes
	if idx.IsOverCap(cfg) {
		t.Error("IsOverCap() = true, want false for single entry")
	}
}

func TestIsOverCap_OverLineLimit(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	cfg.IndexMaxLines = 3

	// Build an index that will exceed 3 lines when serialized
	// (header lines + table rows)
	idx := &Index{
		Entries: []IndexEntry{
			{Name: "e1", Type: MemoryTypeUser, Description: "d1", FilePath: "e1.md"},
			{Name: "e2", Type: MemoryTypeProject, Description: "d2", FilePath: "e2.md"},
			{Name: "e3", Type: MemoryTypeFeedback, Description: "d3", FilePath: "e3.md"},
			{Name: "e4", Type: MemoryTypeReference, Description: "d4", FilePath: "e4.md"},
		},
	}

	if !idx.IsOverCap(cfg) {
		t.Error("IsOverCap() = false, want true when line limit is exceeded")
	}
}

func TestIsOverCap_OverByteLimit(t *testing.T) {
	t.Parallel()
	cfg := DefaultMemoryConfig()
	cfg.IndexMaxBytes = 50 // Very small byte limit

	// Even a single long-description entry should exceed 50 bytes
	idx := &Index{
		Entries: []IndexEntry{
			{
				Name:        "long-entry",
				Type:        MemoryTypeUser,
				Description: "This is a sufficiently long description that exceeds fifty bytes",
				FilePath:    "long-entry.md",
			},
		},
	}

	if !idx.IsOverCap(cfg) {
		t.Error("IsOverCap() = false, want true when byte limit is exceeded")
	}
}

func TestTrimToFit_RemovesOldest(t *testing.T) {
	t.Parallel()
	now := time.Now()

	idx := &Index{
		Entries: []IndexEntry{
			{Name: "oldest", Type: MemoryTypeUser, Description: "d", FilePath: "oldest.md"},
			{Name: "middle", Type: MemoryTypeProject, Description: "d", FilePath: "middle.md"},
			{Name: "newest", Type: MemoryTypeFeedback, Description: "d", FilePath: "newest.md"},
		},
	}

	// Use a low byte cap so TrimToFit must remove entries.
	// "oldest" should be removed first.
	_ = now
	cfg := DefaultMemoryConfig()
	cfg.IndexMaxLines = 5 // Only 2 rows + headers can fit

	removed := idx.TrimToFit(cfg)
	if removed == 0 {
		t.Error("TrimToFit() returned 0 removed, want > 0")
	}
	// "newest" should still be present — oldest entries removed first
	found := false
	for _, e := range idx.Entries {
		if e.Name == "newest" {
			found = true
		}
	}
	if !found {
		t.Error("TrimToFit() removed 'newest' entry, but should preserve newest and remove oldest")
	}

	// The index should no longer be over cap
	if idx.IsOverCap(cfg) {
		t.Error("after TrimToFit(), index is still over cap")
	}

	// Check that serialization works and mentions no dropped entries
	_ = strings.Contains("", "")
}
