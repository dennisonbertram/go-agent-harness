package tools

import (
	"fmt"
	"sync"
)

const (
	defaultMaxCommandOutputBytes = 16 * 1024
	truncatedOutputMarker        = "\n...[truncated output]...\n"
)

type headTailBuffer struct {
	mu      sync.Mutex
	max     int
	headCap int
	tailCap int
	total   int
	head    []byte
	tail    []byte
}

func newHeadTailBuffer(max int) *headTailBuffer {
	if max <= 0 {
		max = defaultMaxCommandOutputBytes
	}
	headCap := max / 2
	tailCap := max - headCap
	return &headTailBuffer{
		max:     max,
		headCap: headCap,
		tailCap: tailCap,
		head:    make([]byte, 0, headCap),
		tail:    make([]byte, 0, tailCap),
	}
}

func (b *headTailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.total += len(p)
	remaining := p

	if len(b.head) < b.headCap {
		n := b.headCap - len(b.head)
		if n > len(remaining) {
			n = len(remaining)
		}
		b.head = append(b.head, remaining[:n]...)
		remaining = remaining[n:]
	}

	if b.tailCap > 0 && len(remaining) > 0 {
		if len(remaining) >= b.tailCap {
			b.tail = append(b.tail[:0], remaining[len(remaining)-b.tailCap:]...)
		} else {
			b.tail = append(b.tail, remaining...)
			if len(b.tail) > b.tailCap {
				b.tail = append([]byte{}, b.tail[len(b.tail)-b.tailCap:]...)
			}
		}
	}

	return len(p), nil
}

func (b *headTailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.total <= b.max {
		combined := make([]byte, 0, len(b.head)+len(b.tail))
		combined = append(combined, b.head...)
		combined = append(combined, b.tail...)
		return string(combined)
	}

	combined := make([]byte, 0, len(b.head)+len(truncatedOutputMarker)+len(b.tail))
	combined = append(combined, b.head...)
	combined = append(combined, []byte(truncatedOutputMarker)...)
	combined = append(combined, b.tail...)
	return string(combined)
}

func mergeCommandStreams(stdout, stderr string) string {
	switch {
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return fmt.Sprintf("%s\n%s", stdout, stderr)
	}
}
