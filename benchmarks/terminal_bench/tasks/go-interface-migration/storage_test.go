package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupTestStorage(t *testing.T) *FileStorage {
	t.Helper()
	dir := t.TempDir()
	fs, err := NewFileStorage(dir)
	if err != nil {
		t.Fatalf("NewFileStorage: %v", err)
	}
	return fs
}

func TestPutAndGet(t *testing.T) {
	fs := setupTestStorage(t)
	if err := fs.Put("hello.txt", []byte("world")); err != nil {
		t.Fatal(err)
	}
	data, err := fs.Get("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "world" {
		t.Fatalf("expected 'world', got %q", string(data))
	}
}

func TestExists(t *testing.T) {
	fs := setupTestStorage(t)
	ok, err := fs.Exists("nope.txt")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false for nonexistent key")
	}
	fs.Put("yep.txt", []byte("here"))
	ok, err = fs.Exists("yep.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true for existing key")
	}
}

func TestListEmpty(t *testing.T) {
	fs := setupTestStorage(t)
	keys, err := fs.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(keys))
	}
}

func TestListWithPrefix(t *testing.T) {
	fs := setupTestStorage(t)
	fs.Put("docs/readme.md", []byte("r"))
	fs.Put("docs/guide.md", []byte("g"))
	fs.Put("src/main.go", []byte("m"))
	fs.Put("src/util.go", []byte("u"))

	keys, err := fs.List("docs/")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"docs/guide.md", "docs/readme.md"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, keys)
	}
	sort.Strings(keys)
	for i := range expected {
		if keys[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, keys)
		}
	}
}

func TestListAllFiles(t *testing.T) {
	fs := setupTestStorage(t)
	fs.Put("a.txt", []byte("a"))
	fs.Put("b.txt", []byte("b"))
	fs.Put("sub/c.txt", []byte("c"))

	keys, err := fs.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}
}

func TestListSorted(t *testing.T) {
	fs := setupTestStorage(t)
	fs.Put("z.txt", []byte("z"))
	fs.Put("a.txt", []byte("a"))
	fs.Put("m.txt", []byte("m"))

	keys, err := fs.List("")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Fatalf("keys not sorted: %v", keys)
		}
	}
}

func TestDeleteExisting(t *testing.T) {
	fs := setupTestStorage(t)
	fs.Put("delete-me.txt", []byte("bye"))

	err := fs.Delete("delete-me.txt")
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	ok, _ := fs.Exists("delete-me.txt")
	if ok {
		t.Fatal("file still exists after Delete")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	fs := setupTestStorage(t)
	err := fs.Delete("ghost.txt")
	if err == nil {
		t.Fatal("expected error when deleting nonexistent file")
	}
}

func TestDeleteInSubdir(t *testing.T) {
	fs := setupTestStorage(t)
	fs.Put("sub/deep/file.txt", []byte("deep"))

	err := fs.Delete("sub/deep/file.txt")
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	full := filepath.Join(fs.baseDir, "sub/deep/file.txt")
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		t.Fatal("file still on disk after Delete")
	}
}

func TestInventoryIntegration(t *testing.T) {
	fs := setupTestStorage(t)
	inv := NewInventory(fs)

	inv.AddItem("tools/hammer", []byte("heavy"))
	inv.AddItem("tools/saw", []byte("sharp"))
	inv.AddItem("food/apple", []byte("sweet"))

	items, err := inv.ListItems("tools/")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 tool items, got %d", len(items))
	}

	err = inv.RemoveItem("tools/hammer")
	if err != nil {
		t.Fatal(err)
	}

	has, _ := inv.HasItem("tools/hammer")
	if has {
		t.Fatal("hammer should be removed")
	}
}
