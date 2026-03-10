package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreprocessCommands_SimpleCommand(t *testing.T) {
	t.Parallel()
	content := "Output: !`echo hello`"
	result := preprocessCommands(context.Background(), content, "")
	if result != "Output: hello" {
		t.Fatalf("expected %q, got %q", "Output: hello", result)
	}
}

func TestPreprocessCommands_MultipleCommands(t *testing.T) {
	t.Parallel()
	content := "A: !`echo alpha` B: !`echo beta`"
	result := preprocessCommands(context.Background(), content, "")
	if result != "A: alpha B: beta" {
		t.Fatalf("expected %q, got %q", "A: alpha B: beta", result)
	}
}

func TestPreprocessCommands_NoCommands(t *testing.T) {
	t.Parallel()
	content := "No commands here, just plain text."
	result := preprocessCommands(context.Background(), content, "")
	if result != content {
		t.Fatalf("expected unchanged content, got %q", result)
	}
}

func TestPreprocessCommands_InsideCodeBlock(t *testing.T) {
	t.Parallel()
	content := "Before\n```\n!`echo should-not-run`\n```\nAfter !`echo ran`"
	result := preprocessCommands(context.Background(), content, "")
	// The command inside the fenced block should NOT be executed.
	if !strings.Contains(result, "!`echo should-not-run`") {
		t.Fatalf("command inside code block was executed: %q", result)
	}
	// The command outside should be executed.
	if !strings.Contains(result, "After ran") {
		t.Fatalf("command outside code block was not executed: %q", result)
	}
}

func TestPreprocessCommands_CommandFailure(t *testing.T) {
	t.Parallel()
	content := "Result: !`exit 1`"
	result := preprocessCommands(context.Background(), content, "")
	if !strings.Contains(result, "[shell-preprocess error:") {
		t.Fatalf("expected error marker, got %q", result)
	}
	if !strings.Contains(result, "exit 1") {
		t.Fatalf("expected command in error marker, got %q", result)
	}
}

func TestPreprocessCommands_CommandTimeout(t *testing.T) {
	t.Parallel()
	// Use a very short context deadline to force timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	content := "Result: !`sleep 10`"
	result := preprocessCommands(ctx, content, "")
	if !strings.Contains(result, "[shell-preprocess error:") {
		t.Fatalf("expected error marker for timeout, got %q", result)
	}
}

func TestPreprocessCommands_OutputTruncation(t *testing.T) {
	t.Parallel()
	// Generate >32KB of output.
	content := "Result: !`dd if=/dev/zero bs=1024 count=64 2>/dev/null | tr '\\0' 'A'`"
	result := preprocessCommands(context.Background(), content, "")
	// Strip "Result: " prefix.
	output := strings.TrimPrefix(result, "Result: ")
	if len(output) > maxCommandOutput {
		t.Fatalf("output should be truncated to %d bytes, got %d", maxCommandOutput, len(output))
	}
}

func TestPreprocessCommands_EmptyOutput(t *testing.T) {
	t.Parallel()
	content := "Result: !`true`"
	result := preprocessCommands(context.Background(), content, "")
	if result != "Result: " {
		t.Fatalf("expected %q, got %q", "Result: ", result)
	}
}

func TestPreprocessCommands_WorkingDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")
	if err := os.WriteFile(marker, []byte("found-it"), 0o644); err != nil {
		t.Fatal(err)
	}

	content := "Content: !`cat marker.txt`"
	result := preprocessCommands(context.Background(), content, dir)
	if result != "Content: found-it" {
		t.Fatalf("expected %q, got %q", "Content: found-it", result)
	}
}

func TestPreprocessCommands_ContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	content := "Result: !`echo hello`"
	result := preprocessCommands(ctx, content, "")
	if !strings.Contains(result, "[shell-preprocess error:") {
		t.Fatalf("expected error marker for cancelled context, got %q", result)
	}
}

func TestPreprocessCommands_MultilineOutput(t *testing.T) {
	t.Parallel()
	content := "Lines: !`printf 'line1\\nline2\\nline3'`"
	result := preprocessCommands(context.Background(), content, "")
	expected := "Lines: line1\nline2\nline3"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestPreprocessCommands_CommandWithSpecialChars(t *testing.T) {
	t.Parallel()
	// Pipes.
	content := "Result: !`echo hello world | tr ' ' '-'`"
	result := preprocessCommands(context.Background(), content, "")
	if result != "Result: hello-world" {
		t.Fatalf("expected %q, got %q", "Result: hello-world", result)
	}
}

func TestPreprocessCommands_MultipleCodeBlocks(t *testing.T) {
	t.Parallel()
	content := "Before !`echo a`\n```\n!`echo skip1`\n```\nMiddle !`echo b`\n```\n!`echo skip2`\n```\nEnd !`echo c`"
	result := preprocessCommands(context.Background(), content, "")
	if !strings.Contains(result, "Before a") {
		t.Fatalf("first command not processed: %q", result)
	}
	if !strings.Contains(result, "Middle b") {
		t.Fatalf("middle command not processed: %q", result)
	}
	if !strings.Contains(result, "End c") {
		t.Fatalf("end command not processed: %q", result)
	}
	if !strings.Contains(result, "!`echo skip1`") {
		t.Fatalf("code block 1 command was executed: %q", result)
	}
	if !strings.Contains(result, "!`echo skip2`") {
		t.Fatalf("code block 2 command was executed: %q", result)
	}
}

func TestPreprocessCommands_EmptyWorkDir(t *testing.T) {
	t.Parallel()
	// When workDir is empty, the command inherits the current process working dir.
	content := "Result: !`echo ok`"
	result := preprocessCommands(context.Background(), content, "")
	if result != "Result: ok" {
		t.Fatalf("expected %q, got %q", "Result: ok", result)
	}
}

func TestPreprocessCommands_ContentWithBackticksButNoCommand(t *testing.T) {
	t.Parallel()
	// Backticks without the ! prefix should be left alone.
	content := "Use `echo hello` to print."
	result := preprocessCommands(context.Background(), content, "")
	if result != content {
		t.Fatalf("expected unchanged content, got %q", result)
	}
}

func TestPreprocessCommands_TrailingNewlineStripped(t *testing.T) {
	t.Parallel()
	content := "Result: !`echo hello`"
	result := preprocessCommands(context.Background(), content, "")
	// echo adds a trailing newline, but we strip it.
	if strings.HasSuffix(result, "\n") {
		t.Fatalf("trailing newline should be stripped, got %q", result)
	}
	if result != "Result: hello" {
		t.Fatalf("expected %q, got %q", "Result: hello", result)
	}
}
