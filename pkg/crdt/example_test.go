package crdt_test

import (
	"fmt"

	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

// Example shows the basic local API: insert two characters and read the
// resulting text.
func Example() {
	tree := crdt.NewFugueTree("replica-A")
	_, _ = tree.InsertAt(0, 'H')
	_, _ = tree.InsertAt(1, 'i')

	fmt.Println(tree.State())
	// Output: Hi
}

// ExampleFugueTree_remoteInsert demonstrates shipping a single insert from
// one replica to another.
func ExampleFugueTree_remoteInsert() {
	a := crdt.NewFugueTree("replica-A")
	b := crdt.NewFugueTree("replica-B")

	opA, _ := a.InsertAt(0, 'X')
	b.ApplyRemoteInsert(opA)

	fmt.Println(a.State(), b.State())
	// Output: X X
}

// ExampleFugueTree_concurrentInsert demonstrates Fugue's headline guarantee:
// when two replicas insert at the same position concurrently, their text
// never interleaves. Both replicas converge to the same string.
func ExampleFugueTree_concurrentInsert() {
	a := crdt.NewFugueTree("replica-A")
	b := crdt.NewFugueTree("replica-B")

	for _, r := range "AAA" {
		_, _ = a.InsertAt(a.Len(), r)
	}
	for _, r := range "BBB" {
		_, _ = b.InsertAt(b.Len(), r)
	}

	_ = a.Merge(b)
	_ = b.Merge(a)

	converged := a.State() == b.State()
	noInterleave := a.State() == "AAABBB" || a.State() == "BBBAAA"
	fmt.Println(converged, noInterleave)
	// Output: true true
}

// ExampleFugueTree_delete demonstrates that delete is a tombstone — the node
// stays in the tree but disappears from the materialised text.
func ExampleFugueTree_delete() {
	tree := crdt.NewFugueTree("replica-A")
	for _, r := range "Hello" {
		_, _ = tree.InsertAt(tree.Len(), r)
	}
	_, _ = tree.DeleteAt(0)

	fmt.Println(tree.State())
	// Output: ello
}

// ExampleFugueTree_merge demonstrates state-based merge: pulling every node
// from another replica's tree in one call.
func ExampleFugueTree_merge() {
	a := crdt.NewFugueTree("replica-A")
	b := crdt.NewFugueTree("replica-B")

	_, _ = a.InsertAt(0, 'A')
	_, _ = b.InsertAt(0, 'B')

	_ = a.Merge(b)
	_ = b.Merge(a)

	fmt.Println(a.State() == b.State())
	// Output: true
}

// ExampleGSet shows a grow-only set converging from two replicas.
func ExampleGSet() {
	a := crdt.NewGSet[string]()
	b := crdt.NewGSet[string]()

	a.Add("alice")
	b.Add("bob")
	a.Add("carol")

	_ = a.Merge(b)
	_ = b.Merge(a)

	fmt.Println(a.Len() == b.Len(), a.Contains("alice") && a.Contains("bob"))
	// Output: true true
}
