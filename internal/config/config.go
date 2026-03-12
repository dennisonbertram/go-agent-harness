// Package config provides a 6-layer configuration cascade for the agent harness.
//
// Layer priority (lowest to highest):
//  1. Built-in defaults (hardcoded)
//  2. User global config: ~/.harness/config.toml
//  3. Project config: .harness/config.toml in workspace root
//  4. Named profile: ~/.harness/profiles/<name>.toml
//  5. CLI/env overrides: HARNESS_* environment variables
//  6. Cloud/team constraints (future stub — not yet applied)
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// CostConfig holds per-run cost ceiling configuration.
type CostConfig struct {
	// MaxPerRunUSD is the maximum spend per run in USD. 0 means unlimited.
	MaxPerRunUSD float64 `toml:"max_per_run_usd"`
}

// MemoryConfig holds memory feature configuration.
type MemoryConfig struct {
	// Enabled controls whether observational memory is active by default.
	Enabled bool `toml:"enabled"`
}

// MCPServerConfig holds the configuration for a single external MCP server.
// The transport field controls how the harness connects to the server.
//
// Example TOML (stdio):
//
//	[mcp_servers.my-tool]
//	transport = "stdio"
//	command = "/usr/local/bin/my-mcp-server"
//	args = ["--verbose"]
//
// Example TOML (http):
//
//	[mcp_servers.remote-tool]
//	transport = "http"
//	url = "http://localhost:3001/mcp"
type MCPServerConfig struct {
	// Transport must be "stdio" or "http".
	Transport string `toml:"transport"`

	// Stdio transport: path or name of the subprocess to launch.
	Command string `toml:"command"`

	// Stdio transport: additional arguments for the subprocess.
	Args []string `toml:"args"`

	// HTTP transport: the MCP server endpoint URL.
	URL string `toml:"url"`
}

// Config is the merged, resolved configuration for a harness instance.
// It represents the final result after all config layers have been applied.
type Config struct {
	// Model is the LLM model identifier (e.g. "gpt-4.1-mini").
	Model string `toml:"model"`

	// MaxSteps is the maximum number of tool-calling steps per run. 0 = unlimited.
	MaxSteps int `toml:"max_steps"`

	// Addr is the HTTP listen address (e.g. ":8080").
	Addr string `toml:"addr"`

	// Cost holds per-run cost ceiling settings.
	Cost CostConfig `toml:"cost"`

	// Memory holds memory feature settings.
	Memory MemoryConfig `toml:"memory"`

	// MCPServers is the map of named external MCP server configurations.
	// Keys are the logical server names (used as prefixes for tool names).
	// This field is populated from the [mcp_servers.*] sections in TOML files.
	MCPServers map[string]MCPServerConfig `toml:"mcp_servers"`
}

// Resolve returns the config itself. It exists to satisfy the issue requirement
// that Config.Resolve() returns the merged config, and to serve as an extension
// point for future cloud/team constraint application (layer 6).
func (c Config) Resolve() Config {
	return c
}

// Defaults returns the built-in default configuration (layer 1).
func Defaults() Config {
	return Config{
		Model:    "gpt-4.1-mini",
		MaxSteps: 0,
		Addr:     ":8080",
		Cost: CostConfig{
			MaxPerRunUSD: 0.0,
		},
		Memory: MemoryConfig{
			Enabled: true,
		},
	}
}

// LoadOptions controls how Load() sources configuration layers.
// Fields that are empty string are treated as "not configured" and
// the corresponding layer is skipped.
type LoadOptions struct {
	// UserConfigPath is the path to the user global config file
	// (typically ~/.harness/config.toml). If empty, the layer is skipped.
	UserConfigPath string

	// ProjectConfigPath is the path to the project config file
	// (typically .harness/config.toml in workspace root). If empty, skipped.
	ProjectConfigPath string

	// ProfilesDir is the directory that contains named profile files
	// (typically ~/.harness/profiles/). Required when ProfileName is non-empty.
	ProfilesDir string

	// ProfileName is the name of the profile to load (without ".toml" suffix).
	// If empty, the profile layer is skipped. Names with path separators or
	// absolute path components are rejected to prevent path traversal.
	ProfileName string

	// Getenv is the function used to read environment variables. If nil,
	// os.Getenv is used. Inject a custom function in tests.
	Getenv func(string) string
}

// rawLayer is the TOML-decoded partial configuration from a single config file.
// Pointer fields distinguish "not set" from "set to zero value", enabling
// correct layered merging where only non-zero fields override lower layers.
// MCPServers uses a plain map because absent keys are naturally nil/missing.
type rawLayer struct {
	Model      *string                    `toml:"model"`
	MaxSteps   *int                       `toml:"max_steps"`
	Addr       *string                    `toml:"addr"`
	Cost       *rawCost                   `toml:"cost"`
	Memory     *rawMemory                 `toml:"memory"`
	MCPServers map[string]MCPServerConfig `toml:"mcp_servers"`
}

type rawCost struct {
	MaxPerRunUSD *float64 `toml:"max_per_run_usd"`
}

type rawMemory struct {
	Enabled *bool `toml:"enabled"`
}

// Load builds the merged Config by walking through all layers in priority
// order. Each successive layer overrides only the fields it explicitly sets.
func Load(opts LoadOptions) (Config, error) {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	// Start from built-in defaults (layer 1).
	cfg := Defaults()

	// Layer 2: user global config.
	if opts.UserConfigPath != "" {
		layer, err := loadTOMLFile(opts.UserConfigPath)
		if err != nil {
			return Config{}, fmt.Errorf("user config %s: %w", opts.UserConfigPath, err)
		}
		applyLayer(&cfg, layer)
	}

	// Layer 3: project config.
	if opts.ProjectConfigPath != "" {
		layer, err := loadTOMLFile(opts.ProjectConfigPath)
		if err != nil {
			return Config{}, fmt.Errorf("project config %s: %w", opts.ProjectConfigPath, err)
		}
		applyLayer(&cfg, layer)
	}

	// Layer 4: named profile.
	if opts.ProfileName != "" {
		if err := validateProfileName(opts.ProfileName); err != nil {
			return Config{}, err
		}
		profilePath := filepath.Join(opts.ProfilesDir, opts.ProfileName+".toml")
		// A named profile that doesn't exist is always an error — the user
		// explicitly requested it, so a missing file is not silently skipped.
		if _, statErr := os.Stat(profilePath); os.IsNotExist(statErr) {
			return Config{}, fmt.Errorf("profile %q not found at %s", opts.ProfileName, profilePath)
		}
		layer, err := loadTOMLFile(profilePath)
		if err != nil {
			return Config{}, fmt.Errorf("profile %s: %w", profilePath, err)
		}
		applyLayer(&cfg, layer)
	}

	// Layer 5: HARNESS_* environment variables.
	applyEnvLayer(&cfg, getenv)

	// Layer 6 (cloud/team constraints): stub — not yet implemented.

	return cfg, nil
}

// loadTOMLFile loads a single TOML config file into a rawLayer.
// Returns the zero-value rawLayer (no overrides) if the file does not exist.
// Returns an error for any other I/O or parse failure.
func loadTOMLFile(path string) (rawLayer, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rawLayer{}, nil // layer is simply absent — not an error
		}
		return rawLayer{}, err
	}
	defer f.Close()

	var layer rawLayer
	if _, err := toml.NewDecoder(f).Decode(&layer); err != nil {
		return rawLayer{}, err
	}
	return layer, nil
}

// applyLayer merges non-nil fields from layer into cfg.
// MCPServers are merged additively: entries in layer are added to (or
// override) any existing entries in cfg. Servers absent from layer are
// preserved unchanged from lower layers.
func applyLayer(cfg *Config, layer rawLayer) {
	if layer.Model != nil {
		cfg.Model = *layer.Model
	}
	if layer.MaxSteps != nil {
		cfg.MaxSteps = *layer.MaxSteps
	}
	if layer.Addr != nil {
		cfg.Addr = *layer.Addr
	}
	if layer.Cost != nil {
		if layer.Cost.MaxPerRunUSD != nil {
			cfg.Cost.MaxPerRunUSD = *layer.Cost.MaxPerRunUSD
		}
	}
	if layer.Memory != nil {
		if layer.Memory.Enabled != nil {
			cfg.Memory.Enabled = *layer.Memory.Enabled
		}
	}
	if len(layer.MCPServers) > 0 {
		if cfg.MCPServers == nil {
			cfg.MCPServers = make(map[string]MCPServerConfig)
		}
		for name, srv := range layer.MCPServers {
			cfg.MCPServers[name] = srv
		}
	}
}

// applyEnvLayer applies HARNESS_* environment variables as layer 5 overrides.
// Invalid values (e.g. non-numeric HARNESS_MAX_STEPS) are silently ignored,
// preserving the previous layer's value.
func applyEnvLayer(cfg *Config, getenv func(string) string) {
	if v := strings.TrimSpace(getenv("HARNESS_MODEL")); v != "" {
		cfg.Model = v
	}
	if v := strings.TrimSpace(getenv("HARNESS_ADDR")); v != "" {
		cfg.Addr = v
	}
	if v := strings.TrimSpace(getenv("HARNESS_MAX_STEPS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSteps = n
		}
		// invalid value: silently skip, preserve previous layer's value
	}
	if v := strings.TrimSpace(getenv("HARNESS_MAX_COST_PER_RUN_USD")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Cost.MaxPerRunUSD = f
		}
	}
}

// validateProfileName ensures the profile name contains no path separators or
// absolute path components that could cause path traversal attacks.
func validateProfileName(name string) error {
	// Reject anything with a path separator.
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid profile name %q: must not contain path separators", name)
	}
	// Reject absolute paths that start with / (already caught above but be explicit).
	if filepath.IsAbs(name) {
		return fmt.Errorf("invalid profile name %q: must not be an absolute path", name)
	}
	// Reject names that contain "..".
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid profile name %q: must not contain '..'", name)
	}
	if name == "" {
		return fmt.Errorf("profile name must not be empty")
	}
	return nil
}
