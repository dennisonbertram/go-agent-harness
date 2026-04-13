package mcpserver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go-agent-harness/internal/mcpserver"
)

func TestStdioServerAccessorsExposeInnerServerAndMetadata(t *testing.T) {
	t.Parallel()

	srv, err := mcpserver.NewStdioServer(
		nil,
		mcpserver.WithServerName("test-harness"),
		mcpserver.WithServerVersion("1.2.3"),
	)
	require.NoError(t, err)
	require.NotNil(t, srv)
	require.NotNil(t, srv.InnerMCPServer())

	info := srv.ServerInfo()
	require.Equal(t, "test-harness", info.Name)
	require.Equal(t, "1.2.3", info.Version)
}
