package mcpserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go-agent-harness/internal/mcpserver"
)

// BT-server-options: WithServerName and WithServerVersion override defaults.
func TestWithServerNameAndVersion(t *testing.T) {
	cfg := mcpserver.DefaultConfig()
	assert.Equal(t, "go-agent-harness", cfg.Name, "default name must be go-agent-harness")
	assert.NotEmpty(t, cfg.Version, "default version must be non-empty")

	mcpserver.WithServerName("my-harness")(cfg)
	assert.Equal(t, "my-harness", cfg.Name)

	mcpserver.WithServerVersion("2.0.0")(cfg)
	assert.Equal(t, "2.0.0", cfg.Version)
}

// Regression: multiple options can be applied without overwriting each other.
func TestOptionsAreAdditive(t *testing.T) {
	cfg := mcpserver.DefaultConfig()
	mcpserver.WithServerName("name-a")(cfg)
	mcpserver.WithServerVersion("v1.2.3")(cfg)

	assert.Equal(t, "name-a", cfg.Name)
	assert.Equal(t, "v1.2.3", cfg.Version)
}
