package main

import (
	"os"
	"path/filepath"
)

// FileStorage implements Storage using the local filesystem.
type FileStorage struct {
	baseDir string
}

// NewFileStorage creates a FileStorage rooted at the given directory.
func NewFileStorage(baseDir string) (*FileStorage, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}
	return &FileStorage{baseDir: baseDir}, nil
}

// Get reads the file at key and returns its contents.
func (fs *FileStorage) Get(key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(fs.baseDir, key))
}

// Put writes data to the file at key, creating directories as needed.
func (fs *FileStorage) Put(key string, data []byte) error {
	full := filepath.Join(fs.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

// Exists reports whether the file at key exists.
func (fs *FileStorage) Exists(key string) (bool, error) {
	_, err := os.Stat(filepath.Join(fs.baseDir, key))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// TODO: List and Delete are not yet implemented.
// The Storage interface requires them, so this file won't compile.
