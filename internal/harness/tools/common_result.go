package tools

import (
	"encoding/json"
	"fmt"
)

// MarshalToolResult JSON-marshals v and returns the string representation.
// Exported for use by tools/core and tools/deferred sub-packages.
func MarshalToolResult(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal tool result: %w", err)
	}
	return string(data), nil
}
