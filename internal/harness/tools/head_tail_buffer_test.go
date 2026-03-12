package tools

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestHeadTailBufferDefaultCap30KB(t *testing.T) {
	t.Parallel()
	buf := newHeadTailBuffer(0)
	if buf.max != 30*1024 {
		t.Fatalf("expected default max %d, got %d", 30*1024, buf.max)
	}
}

func TestHeadTailBufferTruncatedAtExactCap(t *testing.T) {
	t.Parallel()
	const cap = 30 * 1024
	buf := newHeadTailBuffer(cap)

	data := make([]byte, cap)
	for i := range data {
		data[i] = 'A'
	}
	if _, err := buf.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if buf.Truncated() {
		t.Fatal("expected Truncated() == false when written == cap")
	}
	s := buf.String()
	if strings.Contains(s, "truncated") {
		t.Fatal("output should not contain truncation marker at exact cap")
	}
	if len(s) != cap {
		t.Fatalf("expected output length %d, got %d", cap, len(s))
	}
}

func TestHeadTailBufferTruncatedOneByteOver(t *testing.T) {
	t.Parallel()
	const cap = 30 * 1024
	buf := newHeadTailBuffer(cap)

	data := make([]byte, cap+1)
	for i := range data {
		data[i] = byte('A' + (i % 26))
	}
	if _, err := buf.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !buf.Truncated() {
		t.Fatal("expected Truncated() == true when written > cap")
	}
	s := buf.String()
	if !strings.Contains(s, "truncated") {
		t.Fatal("output should contain truncation marker when over cap")
	}
}

func TestHeadTailBufferUTF8Safety(t *testing.T) {
	t.Parallel()
	// Use a small cap to force truncation with multi-byte runes.
	// "€" is 3 bytes (U+20AC).
	const cap = 30
	buf := newHeadTailBuffer(cap)

	// Write enough euros to exceed the cap.
	euro := "€"
	data := strings.Repeat(euro, 100) // 300 bytes, well over 30
	if _, err := buf.Write([]byte(data)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !buf.Truncated() {
		t.Fatal("expected truncation")
	}

	s := buf.String()
	// Remove the truncation marker to check only the head and tail portions.
	parts := strings.SplitN(s, truncatedOutputMarker, 2)
	if len(parts) != 2 {
		t.Fatalf("expected truncation marker in output, got: %q", s)
	}
	head := parts[0]
	tail := parts[1]

	if !utf8.ValidString(head) {
		t.Fatalf("head is not valid UTF-8: %q", head)
	}
	if !utf8.ValidString(tail) {
		t.Fatalf("tail is not valid UTF-8: %q", tail)
	}
}

func TestHeadTailBufferTruncatedMethod(t *testing.T) {
	t.Parallel()

	t.Run("not truncated when empty", func(t *testing.T) {
		t.Parallel()
		buf := newHeadTailBuffer(100)
		if buf.Truncated() {
			t.Fatal("empty buffer should not be truncated")
		}
	})

	t.Run("not truncated under cap", func(t *testing.T) {
		t.Parallel()
		buf := newHeadTailBuffer(100)
		buf.Write([]byte("hello"))
		if buf.Truncated() {
			t.Fatal("under-cap buffer should not be truncated")
		}
	})

	t.Run("truncated over cap", func(t *testing.T) {
		t.Parallel()
		buf := newHeadTailBuffer(10)
		buf.Write([]byte("12345678901")) // 11 bytes > 10
		if !buf.Truncated() {
			t.Fatal("over-cap buffer should be truncated")
		}
	})
}

func TestHeadTailBufferConcurrentTruncation(t *testing.T) {
	t.Parallel()

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			buf := newHeadTailBuffer(100)
			// Write more than 100 bytes to trigger truncation.
			data := make([]byte, 200)
			for j := range data {
				data[j] = byte('a' + (j % 26))
			}
			if _, err := buf.Write(data); err != nil {
				t.Errorf("Write: %v", err)
				return
			}
			if !buf.Truncated() {
				t.Error("expected Truncated() == true")
			}
			s := buf.String()
			if !strings.Contains(s, "truncated") {
				t.Error("expected truncation marker in output")
			}
		}()
	}
	wg.Wait()
}
