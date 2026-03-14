package harnessmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestT8_MalformedJSON verifies that malformed JSON produces a parse error response.
func TestT8_MalformedJSON(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	d := NewDispatcher(client, RealClock{})

	in := strings.NewReader("{not valid json}\n")
	var out bytes.Buffer

	transport := NewStdioTransport(in, &out, d)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Should have written a parse error response.
	line, err := bufio.NewReader(&out).ReadString('\n')
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("got error code %d, want -32700 (Parse error)", resp.Error.Code)
	}
}

// TestT9_Notification_NoOutput verifies that a notification (no ID) produces no output.
func TestT9_Notification_NoOutput(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	d := NewDispatcher(client, RealClock{})

	// A notification has no "id" field.
	notification := `{"jsonrpc":"2.0","method":"initialized"}` + "\n"
	in := strings.NewReader(notification)
	var out bytes.Buffer

	transport := NewStdioTransport(in, &out, d)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for notification, got %q", out.String())
	}
}

// TestT12_ConcurrentRequests tests 10 concurrent requests with race detector.
// All 10 responses should be written with matching IDs.
func TestT12_ConcurrentRequests(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	d := NewDispatcher(client, RealClock{})

	// Build 10 tools/list requests with unique IDs.
	const n = 10
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/list"}`, i)
		sb.WriteString(msg)
		sb.WriteString("\n")
	}

	in := strings.NewReader(sb.String())
	var out safeBuffer

	transport := NewStdioTransport(in, &out, d)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Parse all responses.
	responseIDs := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal response %q: %v", line, err)
		}
		responseIDs[string(resp.ID)] = true
	}

	if len(responseIDs) != n {
		t.Errorf("got %d unique response IDs, want %d", len(responseIDs), n)
	}
	for i := 1; i <= n; i++ {
		idStr := fmt.Sprintf("%d", i)
		if !responseIDs[idStr] {
			t.Errorf("missing response for ID %s", idStr)
		}
	}
}

// TestMissingMethod verifies that requests without a method return -32600.
func TestMissingMethod(t *testing.T) {
	client := NewHarnessClient("http://localhost:9999")
	d := NewDispatcher(client, RealClock{})

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1}` + "\n")
	var out bytes.Buffer

	transport := NewStdioTransport(in, &out, d)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var resp Response
	if err := json.NewDecoder(&out).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("got code %d, want -32600", resp.Error.Code)
	}
}

// safeBuffer is a thread-safe bytes.Buffer for concurrent writes in tests.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
