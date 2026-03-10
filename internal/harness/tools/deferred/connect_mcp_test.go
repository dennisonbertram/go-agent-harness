package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// ---------- mock implementations for connect_mcp tests ----------

// mockDynamicRegistrar implements DynamicToolRegistrar for testing.
// It is thread-safe via an embedded mutex.
type mockDynamicRegistrar struct {
	mu          sync.Mutex
	registered  map[string][]string // serverName -> tool names
	returnErr   error
	returnNames []string // override return value
}

func newMockRegistrar() *mockDynamicRegistrar {
	return &mockDynamicRegistrar{registered: make(map[string][]string)}
}

func (m *mockDynamicRegistrar) RegisterMCPTools(serverName string, toolDefs []tools.MCPToolDefinition, caller tools.MCPRegistry) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.returnErr != nil {
		return nil, m.returnErr
	}
	if _, exists := m.registered[serverName]; exists {
		return nil, fmt.Errorf("MCP server %q is already connected", serverName)
	}
	var names []string
	if m.returnNames != nil {
		names = m.returnNames
	} else {
		for _, td := range toolDefs {
			names = append(names, "mcp_"+sanitizeToolNamePart(serverName)+"_"+sanitizeToolNamePart(td.Name))
		}
	}
	m.registered[serverName] = names
	return names, nil
}

// mockMCPConnector implements MCPConnector for testing.
type mockMCPConnector struct {
	returnRegistry tools.MCPRegistry
	returnErr      error
}

func (m *mockMCPConnector) Connect(_ context.Context, serverURL, serverName string) (tools.MCPRegistry, error) {
	if m.returnErr != nil {
		return nil, m.returnErr
	}
	return m.returnRegistry, nil
}

// mockScopedMCPRegistry implements MCPRegistry and returns tools for a single server.
type mockScopedMCPRegistry struct {
	serverName string
	toolDefs   []tools.MCPToolDefinition
	listErr    error
}

func (m *mockScopedMCPRegistry) ListResources(_ context.Context, server string) ([]tools.MCPResource, error) {
	return nil, nil
}
func (m *mockScopedMCPRegistry) ReadResource(_ context.Context, server, uri string) (string, error) {
	return "", nil
}
func (m *mockScopedMCPRegistry) ListTools(_ context.Context) (map[string][]tools.MCPToolDefinition, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return map[string][]tools.MCPToolDefinition{
		m.serverName: m.toolDefs,
	}, nil
}
func (m *mockScopedMCPRegistry) CallTool(_ context.Context, server, tool string, args json.RawMessage) (string, error) {
	return `{"result":"ok"}`, nil
}

// ---------- tests ----------

// TestConnectMCPTool_Definition verifies the connect_mcp tool definition.
func TestConnectMCPTool_Definition(t *testing.T) {
	reg := newMockRegistrar()
	connector := &mockMCPConnector{}
	tool := ConnectMCPTool(reg, connector)

	assertToolDef(t, tool, "connect_mcp", tools.TierDeferred)
	assertHasTags(t, tool, "mcp", "connect")
}

// TestConnectMCPTool_Handler_MissingURL verifies connect_mcp returns an error when url is empty.
func TestConnectMCPTool_Handler_MissingURL(t *testing.T) {
	reg := newMockRegistrar()
	connector := &mockMCPConnector{}
	tool := ConnectMCPTool(reg, connector)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("expected 'url is required' in error, got %q", err.Error())
	}
}

// TestConnectMCPTool_Handler_InvalidScheme verifies connect_mcp rejects non-http/https URLs.
func TestConnectMCPTool_Handler_InvalidScheme(t *testing.T) {
	reg := newMockRegistrar()
	connector := &mockMCPConnector{}
	tool := ConnectMCPTool(reg, connector)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"url":"ftp://example.com/mcp"}`))
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("expected 'unsupported scheme' in error, got %q", err.Error())
	}
}

// TestConnectMCPTool_Handler_InvalidURL verifies connect_mcp returns an error for malformed URLs.
func TestConnectMCPTool_Handler_InvalidURL(t *testing.T) {
	reg := newMockRegistrar()
	connector := &mockMCPConnector{}
	tool := ConnectMCPTool(reg, connector)

	// A URL with an invalid percent-encoding sequence.
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"url":"http://%ZZ/bad"}`))
	if err == nil {
		t.Fatal("expected error for malformed URL")
	}
}

// TestConnectMCPTool_Handler_InvalidServerName verifies connect_mcp rejects server names with invalid characters.
func TestConnectMCPTool_Handler_InvalidServerName(t *testing.T) {
	reg := newMockRegistrar()
	connector := &mockMCPConnector{}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{
		"url":         "http://localhost:3000/mcp",
		"server_name": "bad name!",
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for invalid server_name")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Errorf("expected 'invalid character' in error, got %q", err.Error())
	}
}

// TestConnectMCPTool_Handler_ConnectorError verifies connect_mcp returns an error when the connector fails.
func TestConnectMCPTool_Handler_ConnectorError(t *testing.T) {
	reg := newMockRegistrar()
	connector := &mockMCPConnector{returnErr: fmt.Errorf("connection refused")}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{"url": "http://localhost:9999/mcp"})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error when connector fails")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected 'connection refused' in error, got %q", err.Error())
	}
}

// TestConnectMCPTool_Handler_ListToolsError verifies connect_mcp returns an error when ListTools fails.
func TestConnectMCPTool_Handler_ListToolsError(t *testing.T) {
	reg := newMockRegistrar()
	mcpReg := &mockScopedMCPRegistry{
		serverName: "test-server",
		listErr:    fmt.Errorf("server unavailable"),
	}
	connector := &mockMCPConnector{returnRegistry: mcpReg}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{
		"url":         "http://localhost:3000/mcp",
		"server_name": "test-server",
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error when ListTools fails")
	}
	if !strings.Contains(err.Error(), "server unavailable") {
		t.Errorf("expected 'server unavailable' in error, got %q", err.Error())
	}
}

// TestConnectMCPTool_Handler_NoTools verifies connect_mcp succeeds with a message when no tools are found.
func TestConnectMCPTool_Handler_NoTools(t *testing.T) {
	reg := newMockRegistrar()
	mcpReg := &mockScopedMCPRegistry{
		serverName: "empty-server",
		toolDefs:   []tools.MCPToolDefinition{},
	}
	connector := &mockMCPConnector{returnRegistry: mcpReg}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{
		"url":         "http://localhost:3000/mcp",
		"server_name": "empty-server",
	})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if count, ok := out["count"].(float64); !ok || count != 0 {
		t.Errorf("expected count=0, got %v", out["count"])
	}
}

// TestConnectMCPTool_Handler_Success verifies connect_mcp registers tools successfully.
func TestConnectMCPTool_Handler_Success(t *testing.T) {
	reg := newMockRegistrar()
	mcpReg := &mockScopedMCPRegistry{
		serverName: "my-server",
		toolDefs: []tools.MCPToolDefinition{
			{
				Name:        "search",
				Description: "Search something",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
			{
				Name:        "fetch",
				Description: "Fetch something",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
	}
	connector := &mockMCPConnector{returnRegistry: mcpReg}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{
		"url":         "http://localhost:3000/mcp",
		"server_name": "my-server",
	})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if count, ok := out["count"].(float64); !ok || count != 2 {
		t.Errorf("expected count=2, got %v", out["count"])
	}
	if out["server_name"] != "my-server" {
		t.Errorf("expected server_name='my-server', got %v", out["server_name"])
	}
	// Verify server was registered.
	reg.mu.Lock()
	_, exists := reg.registered["my-server"]
	reg.mu.Unlock()
	if !exists {
		t.Error("expected server to be registered in the registrar")
	}
}

// TestConnectMCPTool_Handler_DuplicateServer verifies connect_mcp returns an error for duplicate server names.
func TestConnectMCPTool_Handler_DuplicateServer(t *testing.T) {
	reg := newMockRegistrar()
	mcpReg := &mockScopedMCPRegistry{
		serverName: "dup-server",
		toolDefs: []tools.MCPToolDefinition{
			{Name: "tool1", Description: "t1", Parameters: map[string]any{}},
		},
	}
	connector := &mockMCPConnector{returnRegistry: mcpReg}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{
		"url":         "http://localhost:3000/mcp",
		"server_name": "dup-server",
	})

	// First call should succeed.
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("first connect unexpected error: %v", err)
	}

	// Second call with the same server name should fail.
	_, err = tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error for duplicate server name")
	}
	if !strings.Contains(err.Error(), "already connected") {
		t.Errorf("expected 'already connected' in error, got %q", err.Error())
	}
}

// TestConnectMCPTool_Handler_DeriveServerName verifies server name is derived from URL when not provided.
func TestConnectMCPTool_Handler_DeriveServerName(t *testing.T) {
	reg := newMockRegistrar()
	mcpReg := &mockScopedMCPRegistry{
		serverName: "localhost",
		toolDefs: []tools.MCPToolDefinition{
			{Name: "ping", Description: "Ping", Parameters: map[string]any{}},
		},
	}
	connector := &mockMCPConnector{returnRegistry: mcpReg}
	tool := ConnectMCPTool(reg, connector)

	// No server_name provided; should derive from URL.
	args, _ := json.Marshal(map[string]string{
		"url": "http://localhost:3000/mcp",
	})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["server_name"] == "" {
		t.Error("expected non-empty server_name in result")
	}
}

// TestConnectMCPTool_Handler_InvalidJSON verifies connect_mcp returns error for malformed JSON.
func TestConnectMCPTool_Handler_InvalidJSON(t *testing.T) {
	reg := newMockRegistrar()
	connector := &mockMCPConnector{}
	tool := ConnectMCPTool(reg, connector)

	_, err := tool.Handler(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestConnectMCPTool_Handler_RegistrationError verifies connect_mcp returns error when RegisterMCPTools fails.
func TestConnectMCPTool_Handler_RegistrationError(t *testing.T) {
	reg := &mockDynamicRegistrar{
		registered: make(map[string][]string),
		returnErr:  fmt.Errorf("registry full"),
	}
	mcpReg := &mockScopedMCPRegistry{
		serverName: "fail-server",
		toolDefs: []tools.MCPToolDefinition{
			{Name: "tool1", Description: "t1", Parameters: map[string]any{}},
		},
	}
	connector := &mockMCPConnector{returnRegistry: mcpReg}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{
		"url":         "http://localhost:3000/mcp",
		"server_name": "fail-server",
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err == nil {
		t.Fatal("expected error when registration fails")
	}
	if !strings.Contains(err.Error(), "registry full") {
		t.Errorf("expected 'registry full' in error, got %q", err.Error())
	}
}

// TestDeriveServerName verifies the URL-to-server-name derivation.
func TestDeriveServerName(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{"http://localhost:3000/mcp", "localhost"},
		{"https://api.example.com/mcp", "api_example_com"},
		{"http://my-server.internal/path", "my_server_internal"},
	}
	for _, tt := range tests {
		t.Run(tt.rawURL, func(t *testing.T) {
			parsed, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatalf("parse url: %v", err)
			}
			got := deriveServerName(parsed)
			if got != tt.want {
				t.Errorf("deriveServerName(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

// TestValidateServerName verifies server name validation.
func TestValidateServerName(t *testing.T) {
	valid := []string{"my-server", "my_server", "server123", "MyServer"}
	for _, name := range valid {
		if err := validateServerName(name); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}

	invalid := []string{"", "bad name", "bad!name", "bad.name"}
	for _, name := range invalid {
		if err := validateServerName(name); err == nil {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

// TestConnectMCPTool_Handler_HTTPS verifies connect_mcp accepts https URLs.
func TestConnectMCPTool_Handler_HTTPS(t *testing.T) {
	reg := newMockRegistrar()
	mcpReg := &mockScopedMCPRegistry{
		serverName: "secure-server",
		toolDefs: []tools.MCPToolDefinition{
			{Name: "secure_tool", Description: "A secure tool", Parameters: map[string]any{}},
		},
	}
	connector := &mockMCPConnector{returnRegistry: mcpReg}
	tool := ConnectMCPTool(reg, connector)

	args, _ := json.Marshal(map[string]string{
		"url":         "https://secure-server.example.com/mcp",
		"server_name": "secure-server",
	})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error for https URL: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
}

// TestConnectMCPTool_Concurrency verifies connect_mcp is safe under concurrent use with thread-safe mock.
func TestConnectMCPTool_Concurrency(t *testing.T) {
	reg := newMockRegistrar() // thread-safe mock
	var wg sync.WaitGroup
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			serverName := fmt.Sprintf("server-%d", i)
			mcpReg := &mockScopedMCPRegistry{
				serverName: serverName,
				toolDefs: []tools.MCPToolDefinition{
					{Name: "tool", Description: "t", Parameters: map[string]any{}},
				},
			}
			connector := &mockMCPConnector{returnRegistry: mcpReg}
			tool := ConnectMCPTool(reg, connector)

			args, _ := json.Marshal(map[string]string{
				"url":         "http://localhost:3000/mcp",
				"server_name": serverName,
			})
			_, err := tool.Handler(context.Background(), json.RawMessage(args))
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Errorf("concurrent connect_mcp failed: %v", err)
		}
	}
}
