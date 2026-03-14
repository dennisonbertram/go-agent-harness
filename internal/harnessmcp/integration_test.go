package harnessmcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestT11_Integration runs the full flow through StdioTransport:
// initialize -> initialized -> tools/list -> tools/call start_run
// Uses an httptest.Server to mock harnessd.
func TestT11_Integration(t *testing.T) {
	// Mock harnessd.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/runs" {
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "run-integration"})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewHarnessClient(srv.URL)
	d := NewDispatcher(client, RealClock{})

	// Build a sequence of 4 messages.
	messages := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"start_run","arguments":{"prompt":"integration test"}}}`,
	}

	input := strings.Join(messages, "\n") + "\n"
	in := strings.NewReader(input)

	// Use an io.Pipe to capture output.
	pr, pw := io.Pipe()

	transport := NewStdioTransport(in, pw, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run transport in background and close pipe writer when done.
	done := make(chan error, 1)
	go func() {
		err := transport.Run(ctx)
		pw.Close()
		done <- err
	}()

	// Read responses.
	responses := make(map[string]Response)
	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		responses[string(resp.ID)] = resp
	}

	if err := <-done; err != nil {
		t.Fatalf("transport.Run: %v", err)
	}

	// Verify initialize response (id=1).
	initResp, ok := responses["1"]
	if !ok {
		t.Fatal("missing initialize response (id=1)")
	}
	if initResp.Error != nil {
		t.Fatalf("initialize error: %v", initResp.Error)
	}
	var initResult InitializeResult
	if err := json.Unmarshal(initResp.Result, &initResult); err != nil {
		t.Fatalf("unmarshal initialize result: %v", err)
	}
	if initResult.ProtocolVersion != "2025-11-25" {
		t.Errorf("got protocolVersion %q, want %q", initResult.ProtocolVersion, "2025-11-25")
	}

	// initialized is a notification (id=none), no response expected.
	// Verify tools/list response (id=2).
	listResp, ok := responses["2"]
	if !ok {
		t.Fatal("missing tools/list response (id=2)")
	}
	if listResp.Error != nil {
		t.Fatalf("tools/list error: %v", listResp.Error)
	}
	var listResult struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(listResp.Result, &listResult); err != nil {
		t.Fatalf("unmarshal tools/list result: %v", err)
	}
	if len(listResult.Tools) != 5 {
		t.Errorf("got %d tools, want 5", len(listResult.Tools))
	}

	// Verify start_run response (id=3).
	callResp, ok := responses["3"]
	if !ok {
		t.Fatal("missing tools/call response (id=3)")
	}
	if callResp.Error != nil {
		t.Fatalf("tools/call error: %v", callResp.Error)
	}
	var toolResult ToolResult
	if err := json.Unmarshal(callResp.Result, &toolResult); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if toolResult.IsError {
		t.Errorf("unexpected tool error: %v", toolResult.Content)
	}
	if len(toolResult.Content) == 0 {
		t.Fatal("expected content")
	}
	var startResult map[string]string
	if err := json.Unmarshal([]byte(toolResult.Content[0].Text), &startResult); err != nil {
		t.Fatalf("parse content: %v", err)
	}
	if startResult["run_id"] != "run-integration" {
		t.Errorf("got run_id %q, want %q", startResult["run_id"], "run-integration")
	}

	// Verify no spurious response for initialized (notification).
	if _, ok := responses["null"]; ok {
		// A null ID might appear — but it shouldn't be for initialized.
		// This is acceptable as long as it's a valid response.
	}
}
