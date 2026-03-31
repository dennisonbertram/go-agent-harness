package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LoadPlugins reads all *.json files from dir, parses them as PluginDef, and
// validates each one. It returns valid plugins and any errors encountered.
// Errors include the filename for debugging. If the directory does not exist,
// both return values are nil (not an error condition).
func LoadPlugins(dir string) ([]PluginDef, []error) {
	_, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []error{fmt.Errorf("stat %s: %w", dir, err)}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("read dir %s: %w", dir, err)}
	}

	var plugins []PluginDef
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}

		fullPath := filepath.Join(dir, name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: read error: %w", name, err))
			continue
		}

		var def PluginDef
		if err := json.Unmarshal(data, &def); err != nil {
			errs = append(errs, fmt.Errorf("%s: JSON parse error: %w", name, err))
			continue
		}

		if err := def.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("%s: validation error: %w", name, err))
			continue
		}

		plugins = append(plugins, def)
	}

	return plugins, errs
}
