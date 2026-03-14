package harnessmcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// Dispatcher routes incoming JSON-RPC requests to the appropriate handler.
type Dispatcher struct {
	tools map[string]ToolHandler
}

// NewDispatcher creates a Dispatcher wired to the given HarnessClient and Clock.
func NewDispatcher(client *HarnessClient, clock Clock) *Dispatcher {
	d := &Dispatcher{
		tools: map[string]ToolHandler{
			"start_run":      newStartRunHandler(client),
			"get_run_status": newGetRunStatusHandler(client),
			"wait_for_run":   newWaitForRunHandler(client, clock),
			"continue_run":   newContinueRunHandler(client),
			"list_runs":      newListRunsHandler(client),
		},
	}
	return d
}

// Dispatch routes a parsed Request to the correct handler.
// It returns the response and whether a response should be sent.
// shouldRespond is false for notifications (requests with no ID).
func (d *Dispatcher) Dispatch(ctx context.Context, req Request) (Response, bool) {
	// Notifications have no ID — dispatch but do not respond.
	isNotification := req.ID == nil || string(*req.ID) == "null"

	switch req.Method {
	case "initialize":
		result, err := d.handleInitialize(req.Params)
		if err != nil {
			return errorResponse(req.ID, -32600, err.Error()), !isNotification
		}
		return successResponse(req.ID, result), !isNotification

	case "initialized", "$/cancelRequest":
		// Notification — no response.
		return Response{}, false

	case "tools/list":
		result := d.handleToolsList()
		return successResponse(req.ID, result), !isNotification

	case "tools/call":
		result, err := d.handleToolsCall(ctx, req.Params)
		if err != nil {
			return errorResponse(req.ID, -32603, err.Error()), !isNotification
		}
		return successResponse(req.ID, result), !isNotification

	default:
		if isNotification {
			return Response{}, false
		}
		return errorResponse(req.ID, -32601, fmt.Sprintf("method not found: %q", req.Method)), true
	}
}

// handleInitialize processes the initialize request and returns InitializeResult.
func (d *Dispatcher) handleInitialize(params json.RawMessage) (InitializeResult, error) {
	// We accept the initialize without strict parameter validation.
	result := InitializeResult{
		ProtocolVersion: "2025-11-25",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{},
		},
	}
	result.ServerInfo.Name = "harness-mcp"
	result.ServerInfo.Version = "1.0.0"
	return result, nil
}

// handleToolsList returns the list of available tools.
func (d *Dispatcher) handleToolsList() map[string]any {
	return map[string]any{
		"tools": toolDefs(),
	}
}

// handleToolsCall dispatches a tool call to the appropriate handler.
func (d *Dispatcher) handleToolsCall(ctx context.Context, params json.RawMessage) (ToolResult, error) {
	var p ToolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid params: %v", err)}}}, nil
	}

	handler, ok := d.tools[p.Name]
	if !ok {
		return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("unknown tool: %q", p.Name)}}}, nil
	}

	args := p.Arguments
	if args == nil {
		args = json.RawMessage("{}")
	}

	result, err := handler(ctx, args)
	if err != nil {
		return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: err.Error()}}}, nil
	}
	return result, nil
}

// successResponse builds a JSON-RPC success response.
func successResponse(id *json.RawMessage, result any) Response {
	raw, _ := json.Marshal(result)
	resp := Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(raw),
	}
	if id != nil {
		resp.ID = json.RawMessage(*id)
	} else {
		resp.ID = json.RawMessage("null")
	}
	return resp
}

// errorResponse builds a JSON-RPC error response.
func errorResponse(id *json.RawMessage, code int, message string) Response {
	resp := Response{
		JSONRPC: "2.0",
		Error:   &RPCError{Code: code, Message: message},
	}
	if id != nil {
		resp.ID = json.RawMessage(*id)
	} else {
		resp.ID = json.RawMessage("null")
	}
	return resp
}
