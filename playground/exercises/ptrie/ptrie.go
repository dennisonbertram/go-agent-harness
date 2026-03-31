package ptrie

// Node represents a single node in the persistent trie.
type Node struct {
	children map[rune]*Node
	terminal bool
}

// cloneChildren returns a shallow copy of the children map.
func cloneChildren(children map[rune]*Node) map[rune]*Node {
	cloned := make(map[rune]*Node, len(children))
	for k, v := range children {
		cloned[k] = v
	}
	return cloned
}

// Insert returns a new root Node with the word inserted.
// It does not mutate the original trie (copy-on-write).
func Insert(root *Node, word string) *Node {
	if root == nil {
		root = &Node{children: make(map[rune]*Node)}
	}
	if word == "" {
		newNode := &Node{children: cloneChildren(root.children), terminal: true}
		return newNode
	}
	return insertRec(root, []rune(word), 0)
}

func insertRec(curr *Node, word []rune, idx int) *Node {
	newNode := &Node{children: cloneChildren(curr.children), terminal: curr.terminal}
	if idx == len(word) {
		newNode.terminal = true
		return newNode
	}
	ch := word[idx]
	child, ok := curr.children[ch]
	if !ok {
		child = &Node{children: make(map[rune]*Node)}
	}
	newNode.children[ch] = insertRec(child, word, idx+1)
	return newNode
}

// Search returns true if word is present in the trie rooted at root.
func Search(root *Node, word string) bool {
	curr := root
	for _, ch := range word {
		if curr == nil {
			return false
		}
		child, ok := curr.children[ch]
		if !ok {
			return false
		}
		curr = child
	}
	return curr != nil && curr.terminal
}
