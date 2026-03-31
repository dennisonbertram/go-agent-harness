// StdioServer exposes the harness tool catalog over the Model Context Protocol
// (MCP) via a stdio (stdin/stdout) transport using the mark3labs/mcp-go library.
//
// This is distinct from the existing HTTP-based Server (mcpserver.go) which
// exposes harness management tools (start_run, get_run_status, etc.) over HTTP.
// StdioServer is invoked when harnessd is started with the --mcp flag.
package mcpserver

import (
	"context"
	"errors"
	"io"
	"os"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"go-agent-harness/internal/harness/tools"
)

// StdioServer is an MCP server that communicates over stdin/stdout.
type StdioServer struct {
	mcpServer *mcpgo.MCPServer
	cfg       *Config
	numTools  int
}

// NewStdioServer creates a new MCP stdio server with the given tool catalog and options.
// All tools in the catalog (both TierCore and TierDeferred) are registered.
func NewStdioServer(catalog []tools.Tool, opts ...Option) (*StdioServer, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	mcpSrv := mcpgo.NewMCPServer(
		cfg.Name,
		cfg.Version,
		mcpgo.WithToolCapabilities(true),
	)

	bridged := BridgeTools(catalog)
	for _, st := range bridged {
		mcpSrv.AddTool(st.Tool, st.Handler)
	}

	return &StdioServer{
		mcpServer: mcpSrv,
		cfg:       cfg,
		numTools:  len(bridged),
	}, nil
}

// Start begins serving the MCP protocol over stdin/stdout. It blocks until the
// context is cancelled or stdin is closed. Signal handling (SIGINT/SIGTERM)
// must be managed by the caller; cancel the context to trigger shutdown.
//
// context.Canceled and io.EOF are treated as normal termination — they indicate
// that the MCP client disconnected or the process received a shutdown signal.
// Both are filtered to nil so callers see a clean exit rather than an error.
func (s *StdioServer) Start(ctx context.Context) error {
	stdioSrv := mcpgo.NewStdioServer(s.mcpServer)
	err := stdioSrv.Listen(ctx, stdioReaderFunc(), stdioWriterFunc())
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// ToolCount returns the number of harness tools registered on this server.
func (s *StdioServer) ToolCount() int {
	return s.numTools
}

// InnerMCPServer returns the underlying mcp-go MCPServer for advanced use
// (e.g., attaching to an HTTP transport in a later issue).
func (s *StdioServer) InnerMCPServer() *mcpgo.MCPServer {
	return s.mcpServer
}

// ServerInfo returns the MCP server info (name + version).
func (s *StdioServer) ServerInfo() mcplib.Implementation {
	return mcplib.Implementation{
		Name:    s.cfg.Name,
		Version: s.cfg.Version,
	}
}

// stdioReaderFunc and stdioWriterFunc are variables so tests can swap them.
var stdioReaderFunc = func() io.Reader { return os.Stdin }
var stdioWriterFunc = func() io.Writer { return os.Stdout }
