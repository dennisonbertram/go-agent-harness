package networks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadDefinitions(dir string) ([]Definition, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var defs []Definition
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var def Definition
		if err := yaml.Unmarshal(raw, &def); err != nil {
			return nil, fmt.Errorf("parse network %s: %w", name, err)
		}
		if strings.TrimSpace(def.Name) == "" {
			return nil, fmt.Errorf("network %s: name is required", name)
		}
		defs = append(defs, def)
	}
	return defs, nil
}
