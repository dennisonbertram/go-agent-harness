// Package errorchain_test tests the error chain tracing package.
package errorchain_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/forensics/errorchain"
)

// ---------------------------------------------------------------------------
// ErrorClass tests
// ---------------------------------------------------------------------------

func TestErrorClassConstants(t *testing.T) {
	t.Parallel()

	classes := []errorchain.ErrorClass{
		errorchain.ClassToolExecution,
		errorchain.ClassHallucination,
		errorchain.ClassProvider,
		errorchain.ClassResource,
	}

	seen := make(map[errorchain.ErrorClass]bool)
	for _, c := range classes {
		if c == "" {
			t.Errorf("ErrorClass constant is empty string")
		}
		if seen[c] {
			t.Errorf("duplicate ErrorClass value: %q", c)
		}
		seen[c] = true
	}
}

// ---------------------------------------------------------------------------
// ChainedError tests
// ---------------------------------------------------------------------------

func TestNewChainedError_BasicFields(t *testing.T) {
	t.Parallel()

	cause := errors.New("root cause")
	ce := errorchain.NewChainedError(errorchain.ClassProvider, "provider failed", cause)

	if ce == nil {
		t.Fatal("NewChainedError returned nil")
	}
	if ce.Class != errorchain.ClassProvider {
		t.Errorf("Class = %q, want %q", ce.Class, errorchain.ClassProvider)
	}
	if ce.Cause != cause {
		t.Errorf("Cause mismatch: got %v, want %v", ce.Cause, cause)
	}
	if !strings.Contains(ce.Error(), "provider failed") {
		t.Errorf("Error() = %q, want it to contain %q", ce.Error(), "provider failed")
	}
}

func TestNewChainedError_NilCause(t *testing.T) {
	t.Parallel()

	ce := errorchain.NewChainedError(errorchain.ClassToolExecution, "tool blew up", nil)
	if ce.Cause != nil {
		t.Errorf("expected nil Cause, got %v", ce.Cause)
	}
	if ce.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestChainedError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := errors.New("inner")
	outer := errorchain.NewChainedError(errorchain.ClassProvider, "outer", inner)

	if !errors.Is(outer, inner) {
		t.Error("errors.Is(outer, inner) should be true via Unwrap")
	}
}

func TestChainedError_ErrorMessage(t *testing.T) {
	t.Parallel()

	cause := errors.New("disk full")
	ce := errorchain.NewChainedError(errorchain.ClassResource, "resource exhausted", cause)

	msg := ce.Error()
	if !strings.Contains(msg, "resource exhausted") {
		t.Errorf("Error() = %q, want it to contain message", msg)
	}
}

func TestChainedError_ClassAll(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		class errorchain.ErrorClass
		msg   string
	}{
		{errorchain.ClassToolExecution, "tool execution failed"},
		{errorchain.ClassHallucination, "hallucination detected"},
		{errorchain.ClassProvider, "provider error"},
		{errorchain.ClassResource, "resource error"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.class), func(t *testing.T) {
			ce := errorchain.NewChainedError(tc.class, tc.msg, nil)
			if ce.Class != tc.class {
				t.Errorf("Class = %q, want %q", ce.Class, tc.class)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Snapshot tests
// ---------------------------------------------------------------------------

func TestSnapshot_Fields(t *testing.T) {
	t.Parallel()

	snap := errorchain.Snapshot{
		CapturedAt: time.Now(),
		ToolCalls:  []errorchain.ToolCallEntry{{Name: "bash", CallID: "c1", Args: `{"cmd":"ls"}`, ErrorMsg: ""}},
		Messages:   []errorchain.MessageEntry{{Role: "user", Content: "hello"}},
		Depth:      5,
	}

	if snap.CapturedAt.IsZero() {
		t.Error("CapturedAt should not be zero")
	}
	if len(snap.ToolCalls) != 1 {
		t.Errorf("ToolCalls len = %d, want 1", len(snap.ToolCalls))
	}
	if len(snap.Messages) != 1 {
		t.Errorf("Messages len = %d, want 1", len(snap.Messages))
	}
	if snap.Depth != 5 {
		t.Errorf("Depth = %d, want 5", snap.Depth)
	}
}

func TestToolCallEntry_Fields(t *testing.T) {
	t.Parallel()

	e := errorchain.ToolCallEntry{
		Name:     "read",
		CallID:   "tc-001",
		Args:     `{"path":"/etc/hosts"}`,
		ErrorMsg: "permission denied",
	}

	if e.Name != "read" {
		t.Errorf("Name = %q, want %q", e.Name, "read")
	}
	if e.CallID != "tc-001" {
		t.Errorf("CallID = %q, want %q", e.CallID, "tc-001")
	}
	if e.ErrorMsg != "permission denied" {
		t.Errorf("ErrorMsg = %q", e.ErrorMsg)
	}
}

func TestMessageEntry_Fields(t *testing.T) {
	t.Parallel()

	e := errorchain.MessageEntry{
		Role:    "assistant",
		Content: "I will help you.",
	}
	if e.Role != "assistant" {
		t.Errorf("Role = %q, want %q", e.Role, "assistant")
	}
}

// ---------------------------------------------------------------------------
// SnapshotBuilder tests
// ---------------------------------------------------------------------------

func TestSnapshotBuilder_DefaultDepth(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(0)
	if sb == nil {
		t.Fatal("NewSnapshotBuilder returned nil")
	}
	snap := sb.Build()
	if snap.Depth != errorchain.DefaultSnapshotDepth {
		t.Errorf("Depth = %d, want %d", snap.Depth, errorchain.DefaultSnapshotDepth)
	}
}

func TestSnapshotBuilder_CustomDepth(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(3)
	snap := sb.Build()
	if snap.Depth != 3 {
		t.Errorf("Depth = %d, want 3", snap.Depth)
	}
}

func TestSnapshotBuilder_RecordToolCall_Build(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(10)
	sb.RecordToolCall("bash", "c1", `{"cmd":"echo"}`, "")
	sb.RecordToolCall("read", "c2", `{"path":"/tmp"}`, "file not found")

	snap := sb.Build()
	if len(snap.ToolCalls) != 2 {
		t.Fatalf("ToolCalls len = %d, want 2", len(snap.ToolCalls))
	}
	if snap.ToolCalls[0].Name != "bash" {
		t.Errorf("ToolCalls[0].Name = %q, want bash", snap.ToolCalls[0].Name)
	}
	if snap.ToolCalls[1].ErrorMsg != "file not found" {
		t.Errorf("ToolCalls[1].ErrorMsg = %q", snap.ToolCalls[1].ErrorMsg)
	}
}

func TestSnapshotBuilder_RecordMessage_Build(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(10)
	sb.RecordMessage("user", "what is the answer?")
	sb.RecordMessage("assistant", "42")

	snap := sb.Build()
	if len(snap.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(snap.Messages))
	}
	if snap.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want user", snap.Messages[0].Role)
	}
}

func TestSnapshotBuilder_RollingWindow_ToolCalls(t *testing.T) {
	t.Parallel()

	depth := 3
	sb := errorchain.NewSnapshotBuilder(depth)

	// Record more entries than the depth
	for i := 0; i < 10; i++ {
		sb.RecordToolCall(fmt.Sprintf("tool%d", i), fmt.Sprintf("c%d", i), "{}", "")
	}

	snap := sb.Build()
	if len(snap.ToolCalls) != depth {
		t.Errorf("ToolCalls len = %d, want %d (rolling window)", len(snap.ToolCalls), depth)
	}
	// Should keep the LAST `depth` entries
	if snap.ToolCalls[0].Name != "tool7" {
		t.Errorf("ToolCalls[0].Name = %q, want tool7 (last 3 of 10)", snap.ToolCalls[0].Name)
	}
	if snap.ToolCalls[2].Name != "tool9" {
		t.Errorf("ToolCalls[2].Name = %q, want tool9", snap.ToolCalls[2].Name)
	}
}

func TestSnapshotBuilder_RollingWindow_Messages(t *testing.T) {
	t.Parallel()

	depth := 4
	sb := errorchain.NewSnapshotBuilder(depth)

	for i := 0; i < 10; i++ {
		sb.RecordMessage("user", fmt.Sprintf("msg%d", i))
	}

	snap := sb.Build()
	if len(snap.Messages) != depth {
		t.Errorf("Messages len = %d, want %d", len(snap.Messages), depth)
	}
	// Last `depth` messages
	if snap.Messages[0].Content != "msg6" {
		t.Errorf("Messages[0].Content = %q, want msg6", snap.Messages[0].Content)
	}
}

func TestSnapshotBuilder_CapturedAt(t *testing.T) {
	t.Parallel()

	before := time.Now()
	sb := errorchain.NewSnapshotBuilder(5)
	snap := sb.Build()
	after := time.Now()

	if snap.CapturedAt.Before(before) || snap.CapturedAt.After(after) {
		t.Errorf("CapturedAt %v is outside [%v, %v]", snap.CapturedAt, before, after)
	}
}

func TestSnapshotBuilder_Empty(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(10)
	snap := sb.Build()

	if len(snap.ToolCalls) != 0 {
		t.Errorf("ToolCalls len = %d, want 0 for empty builder", len(snap.ToolCalls))
	}
	if len(snap.Messages) != 0 {
		t.Errorf("Messages len = %d, want 0 for empty builder", len(snap.Messages))
	}
}

// ---------------------------------------------------------------------------
// DiagnosticContext interface tests
// ---------------------------------------------------------------------------

func TestDiagnosticContext_Interface(t *testing.T) {
	t.Parallel()

	// A concrete tool that implements DiagnosticContext
	var _ errorchain.DiagnosticContext = (*mockDiagnosticTool)(nil)
}

// mockDiagnosticTool implements DiagnosticContext for test purposes.
type mockDiagnosticTool struct {
	info map[string]any
}

func (m *mockDiagnosticTool) DiagnosticInfo() map[string]any {
	return m.info
}

func TestDiagnosticContext_InfoReturned(t *testing.T) {
	t.Parallel()

	tool := &mockDiagnosticTool{info: map[string]any{
		"cwd":       "/tmp",
		"exit_code": 1,
	}}

	info := tool.DiagnosticInfo()
	if info["cwd"] != "/tmp" {
		t.Errorf("info[cwd] = %v, want /tmp", info["cwd"])
	}
}

// ---------------------------------------------------------------------------
// BuildErrorContextPayload tests
// ---------------------------------------------------------------------------

func TestBuildErrorContextPayload_BasicFields(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(5)
	sb.RecordToolCall("bash", "c1", `{"cmd":"ls"}`, "error")

	cause := errors.New("root")
	ce := errorchain.NewChainedError(errorchain.ClassToolExecution, "tool failed", cause)

	payload := errorchain.BuildErrorContextPayload(ce, sb)

	if payload["class"] != string(errorchain.ClassToolExecution) {
		t.Errorf("class = %v, want %q", payload["class"], errorchain.ClassToolExecution)
	}
	if payload["error"] == nil {
		t.Error("error field should not be nil")
	}
	if payload["snapshot"] == nil {
		t.Error("snapshot field should not be nil")
	}
}

func TestBuildErrorContextPayload_WithCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection refused")
	ce := errorchain.NewChainedError(errorchain.ClassProvider, "provider failed", cause)

	sb := errorchain.NewSnapshotBuilder(5)
	payload := errorchain.BuildErrorContextPayload(ce, sb)

	if payload["cause"] == nil {
		t.Error("cause field should not be nil when Cause is set")
	}
}

func TestBuildErrorContextPayload_NilCause(t *testing.T) {
	t.Parallel()

	ce := errorchain.NewChainedError(errorchain.ClassHallucination, "bad output", nil)
	sb := errorchain.NewSnapshotBuilder(5)
	payload := errorchain.BuildErrorContextPayload(ce, sb)

	// cause field should be absent or nil when no cause
	if v, ok := payload["cause"]; ok && v != nil {
		t.Errorf("cause should be absent/nil when Cause is nil, got %v", v)
	}
}

func TestBuildErrorContextPayload_SnapshotContents(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(5)
	sb.RecordToolCall("grep", "c1", `{}`, "")
	sb.RecordMessage("user", "run grep")

	ce := errorchain.NewChainedError(errorchain.ClassToolExecution, "grep failed", nil)
	payload := errorchain.BuildErrorContextPayload(ce, sb)

	snap, ok := payload["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot is not map[string]any, got %T", payload["snapshot"])
	}
	if _, ok := snap["tool_calls"]; !ok {
		t.Error("snapshot missing tool_calls")
	}
	if _, ok := snap["messages"]; !ok {
		t.Error("snapshot missing messages")
	}
}

// ---------------------------------------------------------------------------
// Concurrency / race tests
// ---------------------------------------------------------------------------

func TestSnapshotBuilder_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(20)
	var wg sync.WaitGroup

	// 20 goroutines each recording tool calls and messages
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			sb.RecordToolCall(fmt.Sprintf("tool%d", i), fmt.Sprintf("c%d", i), "{}", "")
		}()
		go func() {
			defer wg.Done()
			sb.RecordMessage("user", fmt.Sprintf("msg%d", i))
		}()
	}

	wg.Wait()
	// Build should not panic or race
	snap := sb.Build()
	if snap.CapturedAt.IsZero() {
		t.Error("CapturedAt should not be zero after concurrent access")
	}
}

func TestSnapshotBuilder_ConcurrentBuilds(t *testing.T) {
	t.Parallel()

	sb := errorchain.NewSnapshotBuilder(5)
	for i := 0; i < 5; i++ {
		sb.RecordToolCall(fmt.Sprintf("tool%d", i), fmt.Sprintf("c%d", i), "{}", "")
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap := sb.Build()
			if snap.Depth != 5 {
				t.Errorf("concurrent Build: Depth = %d, want 5", snap.Depth)
			}
		}()
	}
	wg.Wait()
}
