package tooluse

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// TestTUI036_ToolChunksAccumulatePerCallID verifies that two different call IDs
// accumulate chunks independently without bleeding into each other.
func TestTUI036_ToolChunksAccumulatePerCallID(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-1", "chunk A1")
	a = a.Append("call-2", "chunk B1")
	a = a.Append("call-1", "chunk A2")
	a = a.Append("call-2", "chunk B2")

	got1 := a.Get("call-1")
	if got1 != "chunk A1chunk A2" {
		t.Errorf("call-1: expected %q, got %q", "chunk A1chunk A2", got1)
	}

	got2 := a.Get("call-2")
	if got2 != "chunk B1chunk B2" {
		t.Errorf("call-2: expected %q, got %q", "chunk B1chunk B2", got2)
	}
}

// TestTUI036_ChunkOutOfOrderHandled verifies that appending an identical chunk
// to the same callID that was the last chunk does not duplicate it (idempotency guard).
func TestTUI036_ChunkOutOfOrderHandled(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-1", "chunk X")
	a = a.Append("call-1", "chunk X") // duplicate — same as last
	a = a.Append("call-1", "chunk Y")

	got := a.Get("call-1")
	// "chunk X" should appear only once, then "chunk Y"
	if got != "chunk Xchunk Y" {
		t.Errorf("expected duplicate last chunk to be skipped, got %q", got)
	}
}

// TestTUI036_AccumulatorImmutable verifies that calling Append on an Accumulator
// returns a new Accumulator and leaves the original unchanged.
func TestTUI036_AccumulatorImmutable(t *testing.T) {
	original := NewAccumulator()
	original = original.Append("call-1", "first")

	modified := original.Append("call-1", "second")

	origContent := original.Get("call-1")
	if origContent != "first" {
		t.Errorf("original must be unchanged after Append, got %q", origContent)
	}

	modContent := modified.Get("call-1")
	if modContent != "firstsecond" {
		t.Errorf("modified must contain both chunks, got %q", modContent)
	}
}

// TestTUI036_AccumulatorDoneFlag verifies that Complete() returns false until
// a chunk with the "done" sentinel has been appended.
func TestTUI036_AccumulatorDoneFlag(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-1", "partial chunk")

	if a.Complete("call-1") {
		t.Error("Complete() must return false before Done sentinel")
	}

	a = a.AppendDone("call-1", "final chunk")

	if !a.Complete("call-1") {
		t.Error("Complete() must return true after AppendDone")
	}
}

// TestTUI036_AccumulatorCallIDOrder verifies that CallIDs() returns call IDs
// in insertion order.
func TestTUI036_AccumulatorCallIDOrder(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-C", "c1")
	a = a.Append("call-A", "a1")
	a = a.Append("call-B", "b1")
	a = a.Append("call-A", "a2") // second append to existing ID

	ids := a.CallIDs()

	if len(ids) != 3 {
		t.Fatalf("expected 3 call IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "call-C" || ids[1] != "call-A" || ids[2] != "call-B" {
		t.Errorf("expected insertion order [call-C, call-A, call-B], got %v", ids)
	}
}

// TestTUI036_AccumulatorReset verifies that Reset() clears one call ID's chunks
// while leaving others intact.
func TestTUI036_AccumulatorReset(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-1", "chunk1")
	a = a.Append("call-2", "chunk2")

	a = a.Reset("call-1")

	if a.Get("call-1") != "" {
		t.Errorf("after Reset, call-1 must be empty, got %q", a.Get("call-1"))
	}
	if a.Get("call-2") != "chunk2" {
		t.Errorf("call-2 must be preserved after resetting call-1, got %q", a.Get("call-2"))
	}
}

// TestTUI036_AccumulatorConcurrent verifies that 10 goroutines each working with
// their own Accumulator produce no data races (run with -race).
func TestTUI036_AccumulatorConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			a := NewAccumulator()
			callID := "call-" + strings.Repeat("x", id+1)
			for j := 0; j < 50; j++ {
				a = a.Append(callID, "chunk")
				_ = a.Get(callID)
				_ = a.Complete(callID)
				_ = a.CallIDs()
			}
			a = a.AppendDone(callID, "last")
			if !a.Complete(callID) {
				panic("Complete() must be true after AppendDone")
			}
		}(i)
	}
	wg.Wait()
}

// TestTUI036_AccumulatorEmptyChunk verifies that zero-size chunks don't corrupt
// the accumulator state and Get() still returns correct content.
func TestTUI036_AccumulatorEmptyChunk(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-1", "real content")
	a = a.Append("call-1", "") // empty chunk
	a = a.Append("call-1", "more content")

	got := a.Get("call-1")
	// Empty chunk should be skipped (idempotency: last was "real content", "" is different so it goes through,
	// but then "" != "more content" so more content appends too)
	// Actually empty string is a distinct value — it should append unless it's identical to the last.
	// "real content" -> "" (different, appends) -> "more content" (different, appends)
	if got == "" {
		t.Error("Get() must not return empty string after appending non-empty chunks")
	}
	if !strings.Contains(got, "real content") {
		t.Errorf("expected 'real content' in accumulated value, got %q", got)
	}
	if !strings.Contains(got, "more content") {
		t.Errorf("expected 'more content' in accumulated value, got %q", got)
	}
}

// TestTUI036_AccumulatorGetUnknownID verifies that Get() on an unknown call ID
// returns empty string without panicking.
func TestTUI036_AccumulatorGetUnknownID(t *testing.T) {
	a := NewAccumulator()
	got := a.Get("nonexistent")
	if got != "" {
		t.Errorf("Get() on unknown ID must return empty string, got %q", got)
	}
}

// TestTUI036_AccumulatorCompleteUnknownID verifies that Complete() on an unknown
// call ID returns false.
func TestTUI036_AccumulatorCompleteUnknownID(t *testing.T) {
	a := NewAccumulator()
	if a.Complete("nonexistent") {
		t.Error("Complete() on unknown ID must return false")
	}
}

// TestTUI036_VisualSnapshot_80x24 renders a streaming accumulator at 80 width
// and writes snapshot to testdata/snapshots/TUI-036-streaming-80x24.txt.
func TestTUI036_VisualSnapshot_80x24(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-bash-1", "Running tests...\n")
	a = a.Append("call-bash-1", "ok  go-agent-harness/cmd/harnesscli/tui\n")
	a = a.Append("call-bash-1", "ok  go-agent-harness/internal/harness\n")
	a = a.AppendDone("call-bash-1", "PASS\n")

	a = a.Append("call-read-2", "package main\n\nimport \"fmt\"\n")
	a = a.Append("call-read-2", "\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n")

	var sb strings.Builder
	for _, id := range a.CallIDs() {
		content := a.Get(id)
		done := a.Complete(id)
		doneStr := ""
		if done {
			doneStr = " [done]"
		}
		sb.WriteString("call ID: " + id + doneStr + "\n")
		b := BashOutput{
			Output: content,
			Width:  80,
		}
		sb.WriteString(b.View())
		sb.WriteString("\n")
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-036-streaming-80x24.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI036_VisualSnapshot_120x40 renders streaming accumulator at 120 width.
func TestTUI036_VisualSnapshot_120x40(t *testing.T) {
	a := NewAccumulator()
	for i := 0; i < 15; i++ {
		a = a.Append("call-long-1", "streaming chunk line\n")
	}
	a = a.AppendDone("call-long-1", "final\n")

	var sb strings.Builder
	for _, id := range a.CallIDs() {
		content := a.Get(id)
		done := a.Complete(id)
		doneStr := ""
		if done {
			doneStr = " [done]"
		}
		sb.WriteString("call ID: " + id + doneStr + "\n")
		b := BashOutput{
			Output: content,
			Width:  120,
		}
		sb.WriteString(b.View())
		sb.WriteString("\n")
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-036-streaming-120x40.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}

// TestTUI036_VisualSnapshot_200x50 renders streaming accumulator at 200 width.
func TestTUI036_VisualSnapshot_200x50(t *testing.T) {
	a := NewAccumulator()
	a = a.Append("call-wide-1", "some output line\n")
	a = a.AppendDone("call-wide-1", "done\n")

	var sb strings.Builder
	for _, id := range a.CallIDs() {
		content := a.Get(id)
		b := BashOutput{
			Output: content,
			Width:  200,
		}
		sb.WriteString(b.View())
		sb.WriteString("\n")
	}

	dir := "testdata/snapshots"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}
	path := dir + "/TUI-036-streaming-200x50.txt"
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
	t.Logf("snapshot written to %s", path)
}
