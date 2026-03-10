package tools

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestJobManagerRunForegroundStreaming(t *testing.T) {
	t.Parallel()

	mgr := NewJobManager(t.TempDir(), nil)

	var mu sync.Mutex
	var chunks []string
	streamer := func(chunk string) {
		mu.Lock()
		defer mu.Unlock()
		chunks = append(chunks, chunk)
	}

	ctx := context.WithValue(context.Background(), ContextKeyOutputStreamer, streamer)

	result, err := mgr.runForeground(ctx, "echo hello; echo world", 5, "")
	if err != nil {
		t.Fatalf("runForeground: %v", err)
	}

	output, _ := result["output"].(string)
	if output != "hello\nworld" {
		t.Fatalf("expected output %q, got %q", "hello\nworld", output)
	}

	mu.Lock()
	defer mu.Unlock()

	// Should have received streaming chunks for both lines.
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 streaming chunks, got %d: %v", len(chunks), chunks)
	}
	combined := strings.Join(chunks, "")
	if !strings.Contains(combined, "hello") || !strings.Contains(combined, "world") {
		t.Fatalf("streaming output missing expected content; chunks: %v", chunks)
	}
}

func TestJobManagerRunForegroundNoStreamer(t *testing.T) {
	t.Parallel()

	mgr := NewJobManager(t.TempDir(), nil)

	// No output streamer in context — should behave exactly as before.
	result, err := mgr.runForeground(context.Background(), "echo hello", 5, "")
	if err != nil {
		t.Fatalf("runForeground: %v", err)
	}

	output, _ := result["output"].(string)
	if output != "hello" {
		t.Fatalf("expected output %q, got %q", "hello", output)
	}
}

func TestJobManagerRunForegroundStreamingCapturesFull(t *testing.T) {
	t.Parallel()

	mgr := NewJobManager(t.TempDir(), nil)

	var mu sync.Mutex
	var chunks []string
	streamer := func(chunk string) {
		mu.Lock()
		defer mu.Unlock()
		chunks = append(chunks, chunk)
	}
	ctx := context.WithValue(context.Background(), ContextKeyOutputStreamer, streamer)

	// Command produces multiple lines; the full output must still be correct.
	result, err := mgr.runForeground(ctx, "printf 'line1\\nline2\\nline3\\n'", 5, "")
	if err != nil {
		t.Fatalf("runForeground: %v", err)
	}

	output, _ := result["output"].(string)
	if output != "line1\nline2\nline3" {
		t.Fatalf("expected trimmed output %q, got %q", "line1\nline2\nline3", output)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks for 3 lines, got %d", len(chunks))
	}
}

func TestJobManagerRunForegroundStreamingConcurrency(t *testing.T) {
	t.Parallel()

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			mgr := NewJobManager(t.TempDir(), nil)

			var mu sync.Mutex
			var chunks []string
			streamer := func(chunk string) {
				mu.Lock()
				defer mu.Unlock()
				chunks = append(chunks, chunk)
			}
			ctx := context.WithValue(context.Background(), ContextKeyOutputStreamer, streamer)

			result, err := mgr.runForeground(ctx, "echo concurrent", 5, "")
			if err != nil {
				t.Errorf("runForeground: %v", err)
				return
			}
			output, _ := result["output"].(string)
			if output != "concurrent" {
				t.Errorf("expected %q, got %q", "concurrent", output)
			}
		}()
	}
	wg.Wait()
}

func TestOutputStreamerFromContext(t *testing.T) {
	t.Parallel()

	// nil context should return false.
	if _, ok := OutputStreamerFromContext(nil); ok {
		t.Fatal("expected false for nil context")
	}

	// Empty context should return false.
	if _, ok := OutputStreamerFromContext(context.Background()); ok {
		t.Fatal("expected false for context without streamer")
	}

	// Context with streamer should return the function.
	called := false
	fn := func(chunk string) { called = true }
	ctx := context.WithValue(context.Background(), ContextKeyOutputStreamer, fn)
	got, ok := OutputStreamerFromContext(ctx)
	if !ok {
		t.Fatal("expected true for context with streamer")
	}
	got("x")
	if !called {
		t.Fatal("streamer was not called")
	}
}
