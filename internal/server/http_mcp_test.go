package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go-agent-harness/internal/harness"
)

// fakeMCPConnector is a test-double MCPConnector.
type fakeMCPConnector struct {
	mu      sync.Mutex
	connect func(ctx context.Context, url, name string) ([]string, error)
}

func (f *fakeMCPConnector) Connect(ctx context.Context, serverURL, serverName string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.connect != nil {
		return f.connect(ctx, serverURL, serverName)
	}
	return []string{"tool_a", "tool_b"}, nil
}

func newMCPTestServer(t *testing.T, connector MCPConnector) *httptest.Server {
	t.Helper()
	registry := harness.NewRegistry()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner, MCPConnector: connector})
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestMCPListEmpty(t *testing.T) {
	t.Parallel()
	ts := newMCPTestServer(t, &fakeMCPConnector{})

	resp, err := http.Get(ts.URL + "/v1/mcp/servers")
	if err != nil {
		t.Fatalf("GET /v1/mcp/servers: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Servers []connectedMCPServer `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Servers) != 0 {
		t.Errorf("expected empty servers list, got %d", len(body.Servers))
	}
}

func TestMCPConnectAndList(t *testing.T) {
	t.Parallel()
	ts := newMCPTestServer(t, &fakeMCPConnector{})

	// Connect a server.
	payload := `{"url":"http://example.com/mcp","name":"my-server"}`
	resp, err := http.Post(ts.URL+"/v1/mcp/servers", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /v1/mcp/servers: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created connectedMCPServer
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Name != "my-server" {
		t.Errorf("name = %q, want my-server", created.Name)
	}
	if created.URL != "http://example.com/mcp" {
		t.Errorf("url = %q", created.URL)
	}
	if len(created.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(created.Tools))
	}

	// List should now include the server.
	resp2, err := http.Get(ts.URL + "/v1/mcp/servers")
	if err != nil {
		t.Fatalf("GET /v1/mcp/servers: %v", err)
	}
	defer resp2.Body.Close()

	var body struct {
		Servers []connectedMCPServer `json:"servers"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(body.Servers))
	}
	if body.Servers[0].Name != "my-server" {
		t.Errorf("servers[0].name = %q", body.Servers[0].Name)
	}
}

func TestMCPConnectMissingURL(t *testing.T) {
	t.Parallel()
	ts := newMCPTestServer(t, &fakeMCPConnector{})

	payload := `{"name":"no-url"}`
	resp, err := http.Post(ts.URL+"/v1/mcp/servers", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /v1/mcp/servers: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestMCPConnectInvalidJSON(t *testing.T) {
	t.Parallel()
	ts := newMCPTestServer(t, &fakeMCPConnector{})

	resp, err := http.Post(ts.URL+"/v1/mcp/servers", "application/json", bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestMCPConnectError(t *testing.T) {
	t.Parallel()
	connector := &fakeMCPConnector{
		connect: func(_ context.Context, _, _ string) ([]string, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	ts := newMCPTestServer(t, connector)

	payload := `{"url":"http://unreachable.local/mcp","name":"bad"}`
	resp, err := http.Post(ts.URL+"/v1/mcp/servers", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", resp.StatusCode)
	}
}

func TestMCPNilConnector(t *testing.T) {
	t.Parallel()
	// No connector configured.
	registry := harness.NewRegistry()
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
	handler := NewWithOptions(ServerOptions{Runner: runner})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// GET should still work (empty list).
	resp, err := http.Get(ts.URL + "/v1/mcp/servers")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// POST should return 501.
	payload := `{"url":"http://example.com/mcp"}`
	resp2, err := http.Post(ts.URL+"/v1/mcp/servers", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", resp2.StatusCode)
	}
}

func TestMCPMethodNotAllowed(t *testing.T) {
	t.Parallel()
	ts := newMCPTestServer(t, &fakeMCPConnector{})

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/v1/mcp/servers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestMCPConnectDerivedName(t *testing.T) {
	t.Parallel()
	var capturedName string
	connector := &fakeMCPConnector{
		connect: func(_ context.Context, _, name string) ([]string, error) {
			capturedName = name
			return []string{"tool_x"}, nil
		},
	}
	ts := newMCPTestServer(t, connector)

	// No name provided — should derive from URL.
	payload := `{"url":"http://myserver.example.com:8080/mcp"}`
	resp, err := http.Post(ts.URL+"/v1/mcp/servers", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if capturedName == "" {
		t.Error("expected derived name to be non-empty")
	}
}

func TestMCPConcurrentAccess(t *testing.T) {
	t.Parallel()
	ts := newMCPTestServer(t, &fakeMCPConnector{})

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			payload := fmt.Sprintf(`{"url":"http://server%d.local/mcp","name":"server%d"}`, i, i)
			resp, err := http.Post(ts.URL+"/v1/mcp/servers", "application/json", bytes.NewBufferString(payload))
			if err != nil {
				return
			}
			resp.Body.Close()
		}(i)
	}
	wg.Wait()

	// All connections should be listed.
	resp, err := http.Get(ts.URL + "/v1/mcp/servers")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Servers []connectedMCPServer `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Servers) != goroutines {
		t.Errorf("expected %d servers, got %d", goroutines, len(body.Servers))
	}
}
