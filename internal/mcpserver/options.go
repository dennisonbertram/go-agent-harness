package mcpserver

// Config holds configuration for the MCP server.
type Config struct {
	// Name is the server name advertised in MCP server info.
	Name string
	// Version is the server version advertised in MCP server info.
	Version string
}

// DefaultConfig returns a Config pre-populated with defaults.
func DefaultConfig() *Config {
	return &Config{
		Name:    "go-agent-harness",
		Version: "0.1.0",
	}
}

// Option is a functional option for configuring the MCP server.
type Option func(*Config)

// WithServerName overrides the MCP server name.
func WithServerName(name string) Option {
	return func(c *Config) {
		c.Name = name
	}
}

// WithServerVersion overrides the MCP server version string.
func WithServerVersion(version string) Option {
	return func(c *Config) {
		c.Version = version
	}
}
