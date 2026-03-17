package merkle

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func toBytes(arr []string) [][]byte {
	var result [][]byte
	for _, v := range arr {
		result = append(result, []byte(v))
	}
	return result
}

func TestMerkleTree_SmallCases(t *testing.T) {
	cases := [][]string{
		{"a"},
		{"a", "b"},
		{"a", "b", "c", "d"},
		{"a", "b", "c", "d", "e", "f", "g"},
	}

	for _, in := range cases {
		leaves := toBytes(in)
		tree := Build(leaves)
		if tree.Root() == nil {
			t.Fatalf("Root should not be nil for non-empty input")
		}
		for i, leaf := range leaves {
			proof := tree.Proof(i)
			if !Verify(leaf, proof, tree.Root()) {
				t.Errorf("Proof for leaf %q (index %d) did not verify", leaf, i)
			}
		}
	}
}

func TestMerkleTree_TamperedLeaf_Fails(t *testing.T) {
	leavesStr := []string{"foo", "bar", "baz", "qux"}
	leaves := toBytes(leavesStr)
	tree := Build(leaves)
	for i, leaf := range leaves {
		proof := tree.Proof(i)
		// Tamper with the leaf
		tampered := make([]byte, len(leaf))
		copy(tampered, leaf)
		tampered[0] ^= 0xff
		if Verify(tampered, proof, tree.Root()) {
			t.Errorf("Tampered leaf at index %d was incorrectly accepted!", i)
		}
	}
}

func TestMerkleTree_ProofLength(t *testing.T) {
	leaves := toBytes([]string{"a", "b", "c", "d"})
	tree := Build(leaves)
	// For 4 leaves, proof should be 2 for non-root leaves
	for i := range leaves {
		proof := tree.Proof(i)
		if len(proof) != 2 && len(leaves) == 4 {
			t.Errorf("Proof for leaf %d has wrong length: %d", i, len(proof))
		}
	}
}

func TestMerkleTree_ReproducibleRoot(t *testing.T) {
	leaves := toBytes([]string{"x", "y", "z"})
	t1 := Build(leaves)
	t2 := Build(leaves)
	if !bytes.Equal(t1.Root(), t2.Root()) {
		t.Fatal("Trees built from same data have different roots")
	}
}

func TestMerkleTree_SingleLeafRoot(t *testing.T) {
	leaf := []byte("unique")
	tree := Build([][]byte{leaf})
	h := sha256.Sum256(leaf)
	if !bytes.Equal(tree.Root(), h[:]) {
		t.Errorf("Single leaf tree, root = sha256(leaf): want %x, got %x", h[:], tree.Root())
	}
}
