// Package mcpserver exposes social-agent database operations as MCP tools
// over an HTTP (streamable-HTTP) transport using github.com/mark3labs/mcp-go.
package mcpserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	mcplib "github.com/mark3labs/mcp-go/server"

	"go-agent-harness/apps/socialagent/db"
)

// UserStore defines the database interface the MCP server needs.
type UserStore interface {
	SearchProfiles(ctx context.Context, query string, limit int) ([]db.UserProfile, error)
	GetProfile(ctx context.Context, userID string) (*db.UserProfile, error)
	GetUserByDisplayName(ctx context.Context, displayName string) (*db.User, error)
	GetUserByID(ctx context.Context, userID string) (*db.User, error)
	GetRecentActivity(ctx context.Context, limit int, excludeUserID string) ([]db.ActivityEntry, error)
	SaveInsight(ctx context.Context, userID, insight, source string) error
	GetInsights(ctx context.Context, userID string) ([]db.UserInsight, error)
	GetAllProfiles(ctx context.Context, excludeUserID string, limit int) ([]db.UserProfile, error)
}

// Server wraps an MCPServer and a UserStore, exposing social tools over MCP.
type Server struct {
	mcpServer *mcplib.MCPServer
	store     UserStore
}

// New creates a new MCP Server backed by the given UserStore and registers all
// social tools.
func New(store UserStore) *Server {
	s := &Server{store: store}
	s.mcpServer = mcplib.NewMCPServer(
		"socialagent-tools",
		"1.0.0",
		mcplib.WithToolCapabilities(false),
	)
	s.registerTools()
	return s
}

// Handler returns an http.Handler that serves the MCP protocol over the
// streamable-HTTP transport. Mount it at any path you like.
func (s *Server) Handler() http.Handler {
	return mcplib.NewStreamableHTTPServer(s.mcpServer,
		mcplib.WithStateLess(true),
	)
}

// CallTool dispatches a tool call directly to the registered handler.
// This is used by tests to invoke tools without going through HTTP.
func (s *Server) CallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	st := s.mcpServer.GetTool(req.Params.Name)
	if st == nil {
		return nil, fmt.Errorf("tool %q not registered", req.Params.Name)
	}
	return st.Handler(ctx, req)
}
