package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// FileVersionFromBytes returns a short hex hash of the given content.
// Exported for use by tools/core and tools/deferred sub-packages.
func FileVersionFromBytes(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:8])
}

// ReadFileVersion reads a file and returns its version hash.
// Exported for use by tools/core and tools/deferred sub-packages.
func ReadFileVersion(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read version file: %w", err)
	}
	return FileVersionFromBytes(content), nil
}
