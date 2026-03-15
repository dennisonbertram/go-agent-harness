package main

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
)

func TestTrieSequential(t *testing.T) {
	trie := NewTrie()
	words := []string{"cat", "car", "cart", "dog", "dot", "dorm", "dormant"}
	for _, w := range words {
		trie.Insert(w)
	}
	for _, w := range words {
		if !trie.Search(w) {
			t.Errorf("Expected to find %s", w)
		}
	}
	// Try deletions
	for _, w := range words {
		if !trie.Delete(w) {
			t.Errorf("Expected to delete %s", w)
		}
		if trie.Search(w) {
			t.Errorf("Should not find deleted word %s", w)
		}
	}
}

func TestTrieConcurrentStress(t *testing.T) {
	trie := NewTrie()
	words := make([]string, 0, 1000)
	for i := 0; i < 1000; i++ {
		words = append(words, "word"+strconv.Itoa(i))
	}
	wg := sync.WaitGroup{}
	for g := 0; g < 20; g++ {
		g := g
		t.Run("goroutine_"+strconv.Itoa(g), func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 100; i++ {
				idx := rand.Intn(len(words))
				w := words[idx]
				trie.Insert(w)
				_ = trie.Search(w)
				trie.Delete(w)
			}
		})
	}
	wg.Wait()
}

func BenchmarkConcurrentInsert(b *testing.B) {
	trie := NewTrie()
	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			word := "bench" + strconv.Itoa(i)
			trie.Insert(word)
			i++
		}
	})
}
