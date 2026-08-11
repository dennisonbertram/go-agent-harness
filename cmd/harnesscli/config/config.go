package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds persistent CLI configuration.
type Config struct {
	StarredModels  []string          `json:"starred_models,omitempty"`
	Gateway        string            `json:"gateway,omitempty"` // "" = direct, "openrouter" = OpenRouter
	APIKeys        map[string]string `json:"api_keys,omitempty"`
	HistoryEntries []string          `json:"history_entries,omitempty"` // newest-first command history
	Theme          string            `json:"theme,omitempty"`           // selected color theme name (epic #810)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "harnesscli", "config.json"), nil
}

// Load reads config from ~/.config/harnesscli/config.json.
// Returns empty Config if the file doesn't exist yet.
func Load() (*Config, error) {
	p, err := configPath()
	if err != nil {
		return &Config{}, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return &Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{}, err
	}
	return &cfg, nil
}

// Exists reports whether a config file is present.
//
// Load returns an empty Config with a nil error when the file is missing, so a
// caller cannot tell "no config yet" from "read fine and it was empty". Writing
// that empty value back destroys whatever was stored (issue #1300); Exists gives
// callers the signal they need to refuse.
func Exists() bool {
	p, err := configPath()
	if err != nil {
		return false
	}
	_, statErr := os.Stat(p)
	return statErr == nil
}

// Save atomically writes cfg to ~/.config/harnesscli/config.json.
//
// The write goes to a temporary file in the same directory and is then renamed
// over the target, so a concurrent reader sees either the old file or the new
// one and never a truncated one. The previous implementation used os.WriteFile,
// which truncates in place: a reader catching that window got a parse error, and
// a save site that ignored the error wrote the resulting empty config back over
// the stored API key (issue #1300).
//
// The temp file is created 0600 rather than being chmod-ed afterwards, so the
// plaintext API key it holds is never briefly world-readable.
func Save(cfg *Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Same directory, so the rename is on one filesystem and therefore atomic.
	tmp, err := os.CreateTemp(dir, ".config-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// No-op once the rename succeeds; cleans up on any early return.
		_ = os.Remove(tmpName)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	// Flush to disk before the rename so a crash cannot leave the new name
	// pointing at an empty file.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, p)
}
