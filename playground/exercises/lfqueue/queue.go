package lfqueue

import (
	"sync/atomic"
	"unsafe"
)

// Node is a single element in the queue
// value is only valid for data nodes (not dummy nodes)
type node[T any] struct {
	value T
	next  unsafe.Pointer // *node[T]
}

// Queue is a lock-free MPMC Michael-Scott queue
// Only atomic operations and unsafe.Pointer are used
// Head and tail are *node[T], but stored as unsafe.Pointer for atomic ops
type Queue[T any] struct {
	head unsafe.Pointer // *node[T]
	tail unsafe.Pointer // *node[T]
}

// NewQueue creates a new lock-free queue
func NewQueue[T any]() *Queue[T] {
	dummy := unsafe.Pointer(&node[T]{})
	return &Queue[T]{
		head: dummy,
		tail: dummy,
	}
}

// Enqueue adds an item to the queue
func (q *Queue[T]) Enqueue(v T) {
	n := &node[T]{value: v}

	for {
		tail := loadPtr[*node[T]](&q.tail)
		next := loadPtr[*node[T]](&tail.next)
		if tail == loadPtr[*node[T]](&q.tail) { // still consistent?
			if next == nil {
				if casPtr(&tail.next, nil, n) {
					// Move tail forward (even if fails, enqueue succeeded)
					casPtr(&q.tail, tail, n)
					return
				}
			} else {
				// Move tail forward
				casPtr(&q.tail, tail, next)
			}
		}
	}
}

// Dequeue removes an item from the queue
// Returns zero-value, false if empty
func (q *Queue[T]) Dequeue() (T, bool) {
	for {
		head := loadPtr[*node[T]](&q.head)
		tail := loadPtr[*node[T]](&q.tail)
		next := loadPtr[*node[T]](&head.next)
		if head == loadPtr[*node[T]](&q.head) { // still consistent?
			if next == nil {
				var zero T
				return zero, false // empty
			}
			value := next.value
			if casPtr(&q.head, head, next) {
				return value, true
			}
		}
	}
}

// Helpers for atomic pointer ops, generics safe:

func loadPtr[T any](p *unsafe.Pointer) T {
	return *(*T)(atomic.LoadPointer(p))
}

func casPtr[T any](p *unsafe.Pointer, old, new T) bool {
	return atomic.CompareAndSwapPointer(p, unsafe.Pointer(old), unsafe.Pointer(new))
}
