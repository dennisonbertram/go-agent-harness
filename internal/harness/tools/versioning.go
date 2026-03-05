package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

func fileVersionFromBytes(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:8])
}

func readFileVersion(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read version file: %w", err)
	}
	return fileVersionFromBytes(content), nil
}
