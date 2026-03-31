package main

import (
	"sort"
	sync "sync"
)

type Allocator struct {
	slabSizes []int
	freeLists map[int][][]byte // map from slab size -> list of []byte slabs
	lock      sync.Mutex
}

// NewAllocator returns a new Allocator managing the given slab sizes (must be power-of-2 and sorted).
func NewAllocator(slabSizes []int) *Allocator {
	sizes := append([]int(nil), slabSizes...)
	sort.Ints(sizes)
	freeLists := make(map[int][][]byte, len(sizes))
	for _, sz := range sizes {
		freeLists[sz] = nil
	}
	return &Allocator{
		slabSizes: sizes,
		freeLists: freeLists,
	}
}

// Alloc returns a slice of at least the requested size.
func (a *Allocator) Alloc(size int) []byte {
	a.lock.Lock()
	defer a.lock.Unlock()
	// Find the smallest slab that fits size
	idx := sort.Search(len(a.slabSizes), func(i int) bool { return a.slabSizes[i] >= size })
	if idx == len(a.slabSizes) {
		// Too big, fall back to make
		return make([]byte, size)
	}
	slabSize := a.slabSizes[idx]
	lst := a.freeLists[slabSize]
	if len(lst) > 0 {
		slab := lst[len(lst)-1]
		a.freeLists[slabSize] = lst[:len(lst)-1]
		return slab[:size]
	}
	return make([]byte, size, slabSize)
}

// Free returns a previously allocated []byte to the appropriate free list if slab size matched exactly.
func (a *Allocator) Free(b []byte) {
	a.lock.Lock()
	defer a.lock.Unlock()
	capb := cap(b)
	for _, sz := range a.slabSizes {
		if sz == capb {
			a.freeLists[sz] = append(a.freeLists[sz], b[:sz])
			return
		}
	}
	// Large allocation, ignore
}
