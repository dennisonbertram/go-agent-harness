package main

import (
	"sync"
)

type TrieNode struct {
	children map[rune]*TrieNode
	terminal bool
}

type Trie struct {
	root  *TrieNode
	mutex sync.RWMutex
}

func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{children: make(map[rune]*TrieNode)},
	}
}

// Insert adds a word to the trie, thread-safe.
func (t *Trie) Insert(word string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	node := t.root
	for _, ch := range word {
		if node.children[ch] == nil {
			node.children[ch] = &TrieNode{children: make(map[rune]*TrieNode)}
		}
		node = node.children[ch]
	}
	node.terminal = true
}

// Search returns true if the word exists in the trie, thread-safe.
func (t *Trie) Search(word string) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	node := t.root
	for _, ch := range word {
		next := node.children[ch]
		if next == nil {
			return false
		}
		node = next
	}
	return node.terminal
}

// Delete removes a word from the trie if it exists.
func (t *Trie) Delete(word string) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return deleteHelper(t.root, word, 0)
}

func deleteHelper(node *TrieNode, word string, idx int) bool {
	if node == nil {
		return false
	}
	if idx == len(word) {
		if node.terminal {
			node.terminal = false
			return len(node.children) == 0
		}
		return false
	}
	ch := rune(word[idx])
	child, ok := node.children[ch]
	if !ok || child == nil {
		return false
	}
	shouldDelete := deleteHelper(child, word, idx+1)
	if shouldDelete {
		delete(node.children, ch)
	}
	return !node.terminal && len(node.children) == 0
}
