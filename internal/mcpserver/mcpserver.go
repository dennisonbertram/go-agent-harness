// Package mcpserver exposes the agent harness as an MCP server over HTTP.
//
// It implements the Model Context Protocol (MCP) JSON-RPC 2.0 protocol,
// serving requests at the /mcp endpoint. The server exposes three tools:
//
//   - start_run: submits a new agent run and returns its run ID
//   - get_run_status: retrieves current status and output for a run
//   - list_runs: lists all known runs
//
// Usage:
//
//	runner := &myRunner{...}
//	s := mcpserver.NewServer(runner)
//	http.ListenAndServe(":8081", s.Handler())
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// RunnerInterface is the subset of the harness runner that the MCP server needs.
//
// This interface is intentionally narrow to decouple mcpserver from the full
// harness package, avoiding import cycles and simplifying testing.
type RunnerInterface interface {
	// StartRun submits a new run with the given prompt and returns its run ID.
	StartRun(prompt string) (string, error)

	// GetRunStatus returns the current status of a run by ID.
	GetRunStatus(runID string) (RunStatus, error)

	// ListRuns returns all known runs.
	ListRuns() ([]RunStatus, error)
}

// RunStatus holds the observable state of a single run.
type RunStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Server is an MCP HTTP server that exposes the harness runner as MCP tools.
// It is safe for concurrent use.
type Server struct {
	runner RunnerInterface
}

// NewServer creates a new MCP server backed by the given runner.
func NewServer(runner RunnerInterface) *Server {
	return &Server{runner: runner}
}

// Handler returns an http.Handler that serves the /mcp endpoint.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)
	return mux
}

// Shutdown performs any cleanup. Currently a no-op; provided as an extension
// point and to satisfy shutdown conventions.
func (s *Server) Shutdown(_ context.Context) error {
	return nil
}

// --- JSON-RPC 2.0 types ---

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      *json.RawMessage `json:"id"` // pointer so we can distinguish missing vs null
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP error codes (subset of JSON-RPC 2.0 standard codes).
const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

// handleMCP is the main HTTP handler for all MCP requests.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, nullID(), errParseError, "parse error: "+err.Error())
		return
	}

	// Notifications (no ID field) are silently acknowledged.
	if req.ID == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	id := *req.ID

	switch req.Method {
	case "initialize":
		s.handleInitialize(w, id, req.Params)
	case "tools/list":
		s.handleToolsList(w, id)
	case "tools/call":
		s.handleToolsCall(w, id, req.Params)
	default:
		writeError(w, id, errMethodNotFound, fmt.Sprintf("method not found: %q", req.Method))
	}
}

// handleInitialize responds to the MCP initialize handshake.
func (s *Server) handleInitialize(w http.ResponseWriter, id json.RawMessage, _ json.RawMessage) {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "go-agent-harness",
			"version": "1.0",
		},
	}
	writeResult(w, id, result)
}

// handleToolsList responds to tools/list.
func (s *Server) handleToolsList(w http.ResponseWriter, id json.RawMessage) {
	tools := []map[string]any{
		{
			"name":        "start_run",
			"description": "Submit a new agent run with the given prompt. Returns the run ID that can be used to poll status.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{
						"type":        "string",
						"description": "The prompt to send to the agent.",
					},
				},
				"required": []string{"prompt"},
			},
		},
		{
			"name":        "get_run_status",
			"description": "Get the current status and output of a run by its run ID.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"run_id": map[string]any{
						"type":        "string",
						"description": "The run ID returned by start_run.",
					},
				},
				"required": []string{"run_id"},
			},
		},
		{
			"name":        "list_runs",
			"description": "List all known runs with their current statuses.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
	writeResult(w, id, map[string]any{"tools": tools})
}

// handleToolsCall dispatches tools/call requests to the appropriate tool handler.
func (s *Server) handleToolsCall(w http.ResponseWriter, id json.RawMessage, params json.RawMessage) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		writeError(w, id, errInvalidParams, "invalid params: "+err.Error())
		return
	}

	switch p.Name {
	case "start_run":
		s.toolStartRun(w, id, p.Arguments)
	case "get_run_status":
		s.toolGetRunStatus(w, id, p.Arguments)
	case "list_runs":
		s.toolListRuns(w, id)
	default:
		// Return an error as a tool result (isError: true), not a JSON-RPC error,
		// since unknown tool is a tool-level error per MCP spec.
		writeToolError(w, id, fmt.Sprintf("unknown tool: %q", p.Name))
	}
}

// toolStartRun handles the start_run tool.
func (s *Server) toolStartRun(w http.ResponseWriter, id json.RawMessage, args json.RawMessage) {
	var a struct {
		Prompt string `json:"prompt"`
	}
	if args != nil {
		_ = json.Unmarshal(args, &a)
	}
	if strings.TrimSpace(a.Prompt) == "" {
		writeToolError(w, id, "prompt is required")
		return
	}

	runID, err := s.runner.StartRun(a.Prompt)
	if err != nil {
		writeToolError(w, id, fmt.Sprintf("start_run failed: %s", err.Error()))
		return
	}
	writeToolText(w, id, fmt.Sprintf("Run started. run_id=%s status=running", runID))
}

// toolGetRunStatus handles the get_run_status tool.
func (s *Server) toolGetRunStatus(w http.ResponseWriter, id json.RawMessage, args json.RawMessage) {
	var a struct {
		RunID string `json:"run_id"`
	}
	if args != nil {
		_ = json.Unmarshal(args, &a)
	}
	if strings.TrimSpace(a.RunID) == "" {
		writeToolError(w, id, "run_id is required")
		return
	}

	status, err := s.runner.GetRunStatus(a.RunID)
	if err != nil {
		writeToolError(w, id, fmt.Sprintf("run %q not found: %s", a.RunID, err.Error()))
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "run_id=%s status=%s", status.ID, status.Status)
	if status.Output != "" {
		fmt.Fprintf(&sb, "\noutput=%s", status.Output)
	}
	if status.Error != "" {
		fmt.Fprintf(&sb, "\nerror=%s", status.Error)
	}
	writeToolText(w, id, sb.String())
}

// toolListRuns handles the list_runs tool.
func (s *Server) toolListRuns(w http.ResponseWriter, id json.RawMessage) {
	runs, err := s.runner.ListRuns()
	if err != nil {
		writeToolError(w, id, fmt.Sprintf("list_runs failed: %s", err.Error()))
		return
	}
	if len(runs) == 0 {
		writeToolText(w, id, "No runs found.")
		return
	}

	var sb strings.Builder
	for i, r := range runs {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "run_id=%s status=%s", r.ID, r.Status)
		if r.Output != "" {
			fmt.Fprintf(&sb, " output=%s", truncate(r.Output, 80))
		}
		if r.Error != "" {
			fmt.Fprintf(&sb, " error=%s", truncate(r.Error, 80))
		}
	}
	writeToolText(w, id, sb.String())
}

// --- response helpers ---

// writeResult writes a successful JSON-RPC response.
func writeResult(w http.ResponseWriter, id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		writeError(w, id, errInternal, "internal error: marshal result: "+err.Error())
		return
	}
	writeRaw(w, rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  raw,
	})
}

// writeError writes a JSON-RPC error response.
func writeError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	writeRaw(w, rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	})
}

// writeToolText writes a successful tools/call result with a text content item.
func writeToolText(w http.ResponseWriter, id json.RawMessage, text string) {
	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"isError": false,
	}
	writeResult(w, id, result)
}

// writeToolError writes a tools/call result indicating a tool-level error.
func writeToolError(w http.ResponseWriter, id json.RawMessage, msg string) {
	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": "Error: " + msg},
		},
		"isError": true,
	}
	writeResult(w, id, result)
}

// writeRaw encodes and writes a JSON-RPC response.
func writeRaw(w http.ResponseWriter, resp rpcResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"internal error"}}`,
			http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// nullID returns the JSON null value, used as the id for parse-error responses
// where we could not determine the request ID.
func nullID() json.RawMessage {
	return json.RawMessage("null")
}

// truncate shortens s to at most n runes, appending "..." if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
