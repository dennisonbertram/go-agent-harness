package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTopicFile_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "go-patterns.md")

	content := `---
name: go-patterns
description: Common Go patterns used in this repo
type: project
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-15T00:00:00Z
---

## Patterns

Use table-driven tests for all unit tests.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tf, err := LoadTopicFile(path)
	if err != nil {
		t.Fatalf("LoadTopicFile() error = %v", err)
	}
	if tf.Entry.Name != "go-patterns" {
		t.Errorf("Entry.Name = %q, want %q", tf.Entry.Name, "go-patterns")
	}
	if tf.Entry.Type != MemoryTypeProject {
		t.Errorf("Entry.Type = %q, want %q", tf.Entry.Type, MemoryTypeProject)
	}
	if tf.Entry.Description != "Common Go patterns used in this repo" {
		t.Errorf("Entry.Description = %q, want %q", tf.Entry.Description, "Common Go patterns used in this repo")
	}
	if !tf.Entry.CreatedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Entry.CreatedAt = %v, want 2026-01-01T00:00:00Z", tf.Entry.CreatedAt)
	}
	if tf.Content == "" {
		t.Error("Content is empty, want non-empty body content")
	}
}

func TestLoadTopicFile_NoFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.md")

	content := "# Plain File\n\nJust some content without frontmatter.\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tf, err := LoadTopicFile(path)
	if err != nil {
		t.Fatalf("LoadTopicFile() error = %v", err)
	}
	// Without frontmatter the content should be the full file content
	if tf.Content == "" {
		t.Error("Content is empty for plain file, want full content")
	}
	// Entry should not panic; name and type will be zero values
	_ = tf.Entry
}

func TestSaveTopicFile_WritesCorrectly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-topic.md")

	original := &TopicFile{
		Entry: MemoryEntry{
			Name:        "test-topic",
			Description: "A test topic for roundtrip",
			Type:        MemoryTypeFeedback,
			FilePath:    path,
			CreatedAt:   time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		},
		Content: "## Test Content\n\nThis is the body of the topic file.\n",
	}

	if err := SaveTopicFile(original); err != nil {
		t.Fatalf("SaveTopicFile() error = %v", err)
	}

	loaded, err := LoadTopicFile(path)
	if err != nil {
		t.Fatalf("LoadTopicFile() after save error = %v", err)
	}

	if loaded.Entry.Name != original.Entry.Name {
		t.Errorf("roundtrip Name: got %q, want %q", loaded.Entry.Name, original.Entry.Name)
	}
	if loaded.Entry.Type != original.Entry.Type {
		t.Errorf("roundtrip Type: got %q, want %q", loaded.Entry.Type, original.Entry.Type)
	}
	if loaded.Entry.Description != original.Entry.Description {
		t.Errorf("roundtrip Description: got %q, want %q", loaded.Entry.Description, original.Entry.Description)
	}
	if loaded.Content != original.Content {
		t.Errorf("roundtrip Content: got %q, want %q", loaded.Content, original.Content)
	}
}

func TestListTopicFiles_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files, err := ListTopicFiles(dir)
	if err != nil {
		t.Fatalf("ListTopicFiles() error = %v", err)
	}
	if len(files) != 0 {
		t.Errorf("ListTopicFiles() = %d files, want 0", len(files))
	}
}

func TestListTopicFiles_WithFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create some .md files
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Create a non-md file that should NOT be listed
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ListTopicFiles(dir)
	if err != nil {
		t.Fatalf("ListTopicFiles() error = %v", err)
	}
	if len(files) != 3 {
		t.Errorf("ListTopicFiles() = %d files, want 3", len(files))
	}
	// All returned paths should end in .md
	for _, f := range files {
		if filepath.Ext(f) != ".md" {
			t.Errorf("ListTopicFiles() returned non-.md file: %q", f)
		}
	}
}
