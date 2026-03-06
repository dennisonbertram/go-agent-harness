package pricing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func LoadCatalog(path string) (*Catalog, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, fmt.Errorf("pricing catalog path is required")
	}

	raw, err := os.ReadFile(trimmedPath)
	if err != nil {
		return nil, fmt.Errorf("read pricing catalog: %w", err)
	}

	var catalog Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, fmt.Errorf("decode pricing catalog: %w", err)
	}
	if len(catalog.Providers) == 0 {
		return nil, fmt.Errorf("pricing catalog has no providers")
	}
	return &catalog, nil
}
