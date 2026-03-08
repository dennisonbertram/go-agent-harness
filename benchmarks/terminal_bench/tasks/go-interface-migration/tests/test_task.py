from __future__ import annotations

import subprocess
from pathlib import Path


def test_build_succeeds() -> None:
    """go build must succeed (the whole point is fixing the build)."""
    result = subprocess.run(
        ["go", "build", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result.returncode == 0, (
        f"build failed:\nstdout: {result.stdout}\nstderr: {result.stderr}"
    )


def test_all_go_tests_pass() -> None:
    """go test must succeed."""
    result = subprocess.run(
        ["go", "test", "-v", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=120,
    )
    assert result.returncode == 0, (
        f"tests failed:\nstdout: {result.stdout}\nstderr: {result.stderr}"
    )


def test_list_method_exists() -> None:
    """file_storage.go must define a List method."""
    contents = Path("/app/file_storage.go").read_text()
    assert "func (fs *FileStorage) List(" in contents or \
           "func (f *FileStorage) List(" in contents, \
        "file_storage.go must define List method"


def test_delete_method_exists() -> None:
    """file_storage.go must define a Delete method."""
    contents = Path("/app/file_storage.go").read_text()
    assert "func (fs *FileStorage) Delete(" in contents or \
           "func (f *FileStorage) Delete(" in contents, \
        "file_storage.go must define Delete method"


def test_list_uses_walk_or_readdir() -> None:
    """List should use filepath.Walk, filepath.WalkDir, or os.ReadDir."""
    contents = Path("/app/file_storage.go").read_text()
    uses_walk = (
        "filepath.Walk" in contents
        or "filepath.WalkDir" in contents
        or "os.ReadDir" in contents
    )
    assert uses_walk, (
        "List should use filepath.Walk, filepath.WalkDir, or os.ReadDir"
    )


def test_storage_interface_unchanged() -> None:
    """storage.go must not be modified."""
    contents = Path("/app/storage.go").read_text()
    assert "type Storage interface" in contents
    assert "Get(key string) ([]byte, error)" in contents
    assert "Put(key string, data []byte) error" in contents
    assert "Exists(key string) (bool, error)" in contents
    assert "List(prefix string) ([]string, error)" in contents
    assert "Delete(key string) error" in contents


def test_consumer_unchanged() -> None:
    """consumer.go must not be modified."""
    contents = Path("/app/consumer.go").read_text()
    assert "type Inventory struct" in contents
    assert "func (inv *Inventory) ListItems" in contents
    assert "func (inv *Inventory) RemoveItem" in contents
