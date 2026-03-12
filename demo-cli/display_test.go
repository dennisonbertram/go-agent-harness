package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// ansiStripRe matches ANSI escape sequences for color/formatting codes.
var ansiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape sequences from s so content can be checked as plain text.
func stripANSI(s string) string {
	return ansiStripRe.ReplaceAllString(s, "")
}

// TestPrintDelta_BuffersContent verifies that PrintDelta accumulates content
// in the internal buffer without printing it immediately.
func TestPrintDelta_BuffersContent(t *testing.T) {
	d := &Display{NoColor: true}

	// Buffer should start empty.
	if d.assistantBuf.Len() != 0 {
		t.Fatalf("expected empty buffer before any PrintDelta, got len=%d", d.assistantBuf.Len())
	}

	d.PrintDelta("Hello, ")
	d.PrintDelta("world")
	d.PrintDelta("!")

	got := d.assistantBuf.String()
	if got != "Hello, world!" {
		t.Errorf("expected buffer %q, got %q", "Hello, world!", got)
	}
}

// TestFlushAssistantMessage_ClearsBuffer ensures the buffer is empty after flush.
func TestFlushAssistantMessage_ClearsBuffer(t *testing.T) {
	d := &Display{NoColor: true}
	d.PrintDelta("some content")

	var buf bytes.Buffer
	d.fprintAssistantMessage(&buf)

	if d.assistantBuf.Len() != 0 {
		t.Errorf("expected empty buffer after flush, got len=%d", d.assistantBuf.Len())
	}
}

// TestFlushAssistantMessage_EmptyBuffer verifies flush on an empty buffer is a no-op.
func TestFlushAssistantMessage_EmptyBuffer(t *testing.T) {
	d := &Display{NoColor: true}

	var buf bytes.Buffer
	d.fprintAssistantMessage(&buf)

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty buffer flush, got %q", buf.String())
	}
}

// TestFlushAssistantMessage_NoColor_PassesRawText verifies that when NoColor is
// true, the raw markdown text is written without glamour rendering.
func TestFlushAssistantMessage_NoColor_PassesRawText(t *testing.T) {
	d := &Display{NoColor: true}
	input := "**bold** and _italic_"
	d.PrintDelta(input)

	var buf bytes.Buffer
	d.fprintAssistantMessage(&buf)

	got := buf.String()
	if got != input {
		t.Errorf("expected raw text %q in no-color mode, got %q", input, got)
	}
}

// TestFlushAssistantMessage_WithColor_ProducesANSI verifies that when NoColor is
// false, the rendered output contains ANSI escape sequences (glamour decorates text).
func TestFlushAssistantMessage_WithColor_ProducesANSI(t *testing.T) {
	d := &Display{NoColor: false}
	// Use markdown that glamour will definitely render with ANSI codes.
	d.PrintDelta("**bold text**")

	var buf bytes.Buffer
	d.fprintAssistantMessage(&buf)

	got := buf.String()
	// Glamour-rendered output always contains at least one ANSI escape sequence.
	if !strings.Contains(got, "\033[") {
		t.Errorf("expected ANSI escape sequences in rendered output, got: %q", got)
	}
}

// TestFlushAssistantMessage_CodeBlock verifies a fenced code block is rendered.
// Glamour fragments text across individual ANSI-wrapped characters, so we strip
// ANSI codes before checking for the plain source text.
func TestFlushAssistantMessage_CodeBlock(t *testing.T) {
	d := &Display{NoColor: false}
	md := "```go\nfmt.Println(\"hello\")\n```"
	d.PrintDelta(md)

	var buf bytes.Buffer
	d.fprintAssistantMessage(&buf)

	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output for code block, got empty string")
	}
	// Strip ANSI codes to check plain text content.
	plain := stripANSI(got)
	if !strings.Contains(plain, "fmt.Println") {
		t.Errorf("expected source content in rendered output (plain: %q)", plain)
	}
}

// TestFlushAssistantMessage_List verifies a markdown list is rendered.
func TestFlushAssistantMessage_List(t *testing.T) {
	d := &Display{NoColor: false}
	md := "- item one\n- item two\n- item three\n"
	d.PrintDelta(md)

	var buf bytes.Buffer
	d.fprintAssistantMessage(&buf)

	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output for list, got empty string")
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "item one") {
		t.Errorf("expected 'item one' in rendered output (plain: %q)", plain)
	}
	if !strings.Contains(plain, "item two") {
		t.Errorf("expected 'item two' in rendered output (plain: %q)", plain)
	}
}

// TestFlushAssistantMessage_BoldText verifies bold markdown is rendered.
func TestFlushAssistantMessage_BoldText(t *testing.T) {
	d := &Display{NoColor: false}
	md := "This is **very important**."
	d.PrintDelta(md)

	var buf bytes.Buffer
	d.fprintAssistantMessage(&buf)

	got := buf.String()
	if got == "" {
		t.Fatal("expected non-empty output for bold text, got empty string")
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "very important") {
		t.Errorf("expected 'very important' in rendered output (plain: %q)", plain)
	}
}

// TestPrintDelta_MultipleFlushes verifies that successive flush+delta cycles work
// correctly — each flush resets the buffer so the next cycle is independent.
func TestPrintDelta_MultipleFlushes(t *testing.T) {
	d := &Display{NoColor: true}

	d.PrintDelta("first message")
	var buf1 bytes.Buffer
	d.fprintAssistantMessage(&buf1)

	d.PrintDelta("second message")
	var buf2 bytes.Buffer
	d.fprintAssistantMessage(&buf2)

	if buf1.String() != "first message" {
		t.Errorf("first flush: expected %q, got %q", "first message", buf1.String())
	}
	if buf2.String() != "second message" {
		t.Errorf("second flush: expected %q, got %q", "second message", buf2.String())
	}
}

// TestFprintRunStarted_VerboseOff verifies that when Verbose is false, no output is produced.
func TestFprintRunStarted_VerboseOff(t *testing.T) {
	d := &Display{NoColor: true, Verbose: false}
	var buf bytes.Buffer
	d.fprintRunStarted(&buf, "run_abc123", "hello world")
	if buf.Len() != 0 {
		t.Errorf("expected empty output when Verbose=false, got %q", buf.String())
	}
}

// TestFprintRunStarted_VerboseOn verifies that when Verbose is true, the run ID and prompt appear.
func TestFprintRunStarted_VerboseOn(t *testing.T) {
	d := &Display{NoColor: true, Verbose: true}
	var buf bytes.Buffer
	d.fprintRunStarted(&buf, "run_abc123", "hello world")
	got := buf.String()
	if !strings.Contains(got, "run_abc123") {
		t.Errorf("expected run ID in output, got %q", got)
	}
	if !strings.Contains(got, "hello world") {
		t.Errorf("expected prompt in output, got %q", got)
	}
}

// TestFprintRunStarted_LongPrompt verifies that prompts longer than 40 chars are truncated with ellipsis.
func TestFprintRunStarted_LongPrompt(t *testing.T) {
	d := &Display{NoColor: true, Verbose: true}
	var buf bytes.Buffer
	longPrompt := "This is a prompt that is definitely longer than forty characters"
	d.fprintRunStarted(&buf, "run_xyz", longPrompt)
	got := buf.String()
	// The truncated snippet should appear (first 40 chars)
	snippet := longPrompt[:40]
	if !strings.Contains(got, snippet) {
		t.Errorf("expected truncated snippet %q in output, got %q", snippet, got)
	}
	// Should contain the ellipsis character
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis '…' in output for long prompt, got %q", got)
	}
	// The full prompt should NOT appear
	if strings.Contains(got, longPrompt) {
		t.Errorf("expected full long prompt to be truncated, but got full prompt in output: %q", got)
	}
}
