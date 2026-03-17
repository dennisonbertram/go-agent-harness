package main

import (
	"math/rand"
	"sort"
	"testing"
)

func TestSequentialInsertAndInOrder(t *testing.T) {
	tree := NewBTree(3) // t=3 is common
	for i := 1; i <= 1000; i++ {
		tree.Insert(i)
	}
	res := tree.InOrder()
	if len(res) != 1000 {
		t.Fatalf("expected 1000 keys, got %d", len(res))
	}
	for i := 0; i < 1000; i++ {
		if res[i] != i+1 {
			t.Fatalf("inorder[%d]=%d, expected %d", i, res[i], i+1)
		}
	}
}

func TestRandomDeletePreservesValidity(t *testing.T) {
	values := rand.Perm(1000)
	tree := NewBTree(3)
	for _, v := range values {
		tree.Insert(v)
	}
	// Delete 500 values at random
	perm := rand.Perm(1000)
	toDelete := perm[:500]
	for _, v := range toDelete {
		tree.Delete(v)
	}
	res := tree.InOrder()
	if !sort.IntsAreSorted(res) {
		t.Fatalf("inorder traversal not sorted: %v", res)
	}
	for _, v := range toDelete {
		if tree.Search(v) {
			t.Fatalf("deleted key %d still found", v)
		}
	}
	for _, v := range perm[500:] {
		if !tree.Search(v) {
			t.Fatalf("retained key %d not found", v)
		}
	}
}

func TestDeleteNonexistentKey(t *testing.T) {
	tree := NewBTree(2)
	tree.Insert(10)
	tree.Delete(20) // should do nothing, not panic
	if !tree.Search(10) {
		t.Fatal("key 10 should still be present")
	}
}

func TestDeleteSingleKey(t *testing.T) {
	tree := NewBTree(2)
	tree.Insert(42)
	tree.Delete(42)
	if tree.Search(42) {
		t.Fatal("key 42 should have been deleted")
	}
	if got := len(tree.InOrder()); got != 0 {
		t.Fatalf("expected 0 keys, got %d", got)
	}
}
