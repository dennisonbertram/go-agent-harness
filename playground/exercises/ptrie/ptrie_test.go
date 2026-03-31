package ptrie

import (
	"math/rand"
	"testing"
)

// genWords generates n unique random words.
func genWords(n int, seed int64) []string {
	words := make(map[string]struct{})
	rand.Seed(seed)
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	for len(words) < n {
		ln := rand.Intn(7) + 3 // words of length 3-9
		w := make([]rune, ln)
		for i := range w {
			w[i] = letters[rand.Intn(len(letters))]
		}
		word := string(w)
		words[word] = struct{}{}
	}
	arr := make([]string, 0, n)
	for k := range words {
		arr = append(arr, k)
	}
	return arr
}

func TestPersistentInsertVersions(t *testing.T) {
	nWords := 1000
	words := genWords(nWords, 123456)
	// Up to 50 versions, each extends previous with +20 new words
	nVersions := 50
	wordsPerVersion := nWords / nVersions
	roots := make([]*Node, nVersions+1) // roots[0]=empty
	wordSets := make([]map[string]struct{}, nVersions+1)
	roots[0] = nil
	wordSets[0] = make(map[string]struct{})
	for v := 1; v <= nVersions; v++ {
		prev := roots[v-1]
		roots[v] = prev
		wordSets[v] = make(map[string]struct{})
		for w := range wordSets[v-1] {
			wordSets[v][w] = struct{}{}
		}
		start := (v - 1) * wordsPerVersion
		end := v * wordsPerVersion
		for i := start; i < end; i++ {
			word := words[i]
			roots[v] = Insert(roots[v], word)
			wordSets[v][word] = struct{}{}
		}
	}
	// Check every version contains all its words, and earlier versions still only have theirs
	for v := 1; v <= nVersions; v++ {
		for w := range wordSets[v] {
			if !Search(roots[v], w) {
				t.Errorf("Version %d should contain %q", v, w)
			}
		}
		for w := range wordSets[v-1] {
			if !Search(roots[v-1], w) {
				t.Errorf("Prev version %d should still contain %q", v-1, w)
			}
			if Search(roots[v-1], words[(v-1)*wordsPerVersion]) && wordSets[v-1][words[(v-1)*wordsPerVersion]] == struct{}{} {
				// prev version must NOT contain first new word of version v
				t.Errorf("Version %d should NOT contain %q", v-1, words[(v-1)*wordsPerVersion])
			}
		}
	}
}

func TestInsertAndSearchIndependent(t *testing.T) {
	root1 := Insert(nil, "cat")
	root2 := Insert(root1, "dog")
	root3 := Insert(root2, "fish")
	if !Search(root1, "cat") || Search(root1, "dog") || Search(root1, "fish") {
		t.Errorf("root1 incorrect immutability")
	}
	if !Search(root2, "cat") || !Search(root2, "dog") || Search(root2, "fish") {
		t.Errorf("root2 incorrect immutability")
	}
	if !Search(root3, "cat") || !Search(root3, "dog") || !Search(root3, "fish") {
		t.Errorf("root3 should have all words")
	}
}
