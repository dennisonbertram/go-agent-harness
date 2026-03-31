package mcpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/mcpserver"
)

// BT-server-create: StdioServer creation with options succeeds.
func TestNewStdioServerCreatesSuccessfully(t *testing.T) {
	fakeCatalog := []tools.Tool{
		{
			Definition: tools.Definition{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters:  map[string]any{"type": "object"},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return "ok", nil
			},
		},
	}

	srv, err := mcpserver.NewStdioServer(fakeCatalog,
		mcpserver.WithServerName("test-harness"),
		mcpserver.WithServerVersion("1.0.0"),
	)

	require.NoError(t, err, "NewStdioServer() must not return an error")
	require.NotNil(t, srv, "NewStdioServer() must return a non-nil server")
}

// Regression: server created with empty catalog does not error.
func TestNewStdioServerWithEmptyCatalogDoesNotError(t *testing.T) {
	srv, err := mcpserver.NewStdioServer(nil)
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

// BT-Fix1a-unit: StdioServer.Start returns nil when context is cancelled
// (simulating SIGINT during a blocking read). This verifies the fix that
// filters context.Canceled to nil so callers see clean shutdown.
func TestStdioServerStartReturnsNilOnContextCancel(t *testing.T) {
	// Use a blocking pipe — the read goroutine inside mcp-go will block,
	// and context cancellation should cause Start to return nil (not context.Canceled).
	pr, _ := io.Pipe()
	// NOTE: we intentionally do NOT close pw, so the reader blocks.

	restore := mcpserver.SetStdioIO(
		func() io.Reader { return pr },
		func() io.Writer { return io.Discard },
	)
	t.Cleanup(func() {
		pr.Close()
		restore()
	})

	srv, err := mcpserver.NewStdioServer(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	// Cancel context to simulate SIGINT
	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("StdioServer.Start must return nil on context cancel, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StdioServer.Start did not return within 5 seconds on context cancel")
	}
}

// BT-Fix1b-unit: StdioServer.Start returns nil when stdin returns io.EOF.
// This verifies the underlying Start method filters EOF to nil so that callers
// (including runMCPStdio) do not need to duplicate the check.
func TestStdioServerStartReturnsNilOnEOF(t *testing.T) {
	pr, pw := io.Pipe()
	pw.Close() // immediate EOF

	restore := mcpserver.SetStdioIO(
		func() io.Reader { return pr },
		func() io.Writer { return io.Discard },
	)
	t.Cleanup(restore)

	srv, err := mcpserver.NewStdioServer(nil)
	require.NoError(t, err)

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("StdioServer.Start must return nil on EOF, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StdioServer.Start did not return within 5 seconds on EOF")
	}
}

// Regression: server created with large catalog works correctly.
func TestNewStdioServerRegistersAllToolsFromCatalog(t *testing.T) {
	fakeCatalog := make([]tools.Tool, 10)
	for i := range fakeCatalog {
		fakeCatalog[i] = tools.Tool{
			Definition: tools.Definition{
				Name:        tools.Definition{}.Name,
				Description: "desc",
				Parameters:  map[string]any{"type": "object"},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return "result", nil
			},
		}
		fakeCatalog[i].Definition.Name = "tool_" + string(rune('a'+i))
	}

	srv, err := mcpserver.NewStdioServer(fakeCatalog)
	require.NoError(t, err)
	require.NotNil(t, srv)

	// The server should have 10 tools registered.
	assert.Equal(t, 10, srv.ToolCount())
}
