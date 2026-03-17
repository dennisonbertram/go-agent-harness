package main

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestIndex(t *testing.T) {
	r := NewRope("hello world")
	for i := 0; i < len("hello world"); i++ {
		if r.Index(i) != "hello world"[i] {
			t.Errorf("Index(%d) = %c, want %c", i, r.Index(i), "hello world"[i])
		}
	}
}

func TestConcat(t *testing.T) {
	left := "hello "
	right := "world!"
	r := NewRope(left).Concat(NewRope(right))
	if got := r.String(); got != left+right {
		t.Errorf("Concat result = %q, want %q", got, left+right)
	}
}

func TestSplit(t *testing.T) {
	orig := "abcdefghij"
	r := NewRope(orig)
	for i := 1; i < len(orig); i++ {
		left, right := r.Split(i)
		if left.String()+right.String() != orig {
			t.Errorf("Split+Concat at %d failed: got %q", i, left.String()+right.String())
		}
	}
}

func TestIndexLarge(t *testing.T) {
	N := 10000
	var sb strings.Builder
	for i := 0; i < N; i++ {
		sb.WriteByte(byte(i % 256))
	}
	str := sb.String()
	rope := NewRope(str)
	for i := 0; i < N; i++ {
		if rope.Index(i) != str[i] {
			t.Fatalf("Index mismatch at %d: got %v want %v", i, rope.Index(i), str[i])
		}
	}
}

func TestSplitConcatRoundTrip(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	N := 10000
	var sb strings.Builder
	for i := 0; i < N; i++ {
		sb.WriteByte(byte(rand.Intn(256)))
	}
	str := sb.String()
	rope := NewRope(str)
	for i := 0; i < 10; i++ {
		splitAt := rand.Intn(N)
		left, right := rope.Split(splitAt)
		joined := left.Concat(right).String()
		if joined != str {
			t.Fatalf("Split/Concat roundtrip failed at %d (seed %d)", splitAt, seed)
		}
	}
}
