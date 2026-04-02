package crdt

import (
	"fmt"
	"testing"
)

func TestGSet_Merge(t *testing.T) {
	gset := NewGSet[string]()
	gset.Add("apple")
	gset.Add("banana")
	gset.Add("cherry")

	gset2 := NewGSet[string]()
	gset2.Add("apple")
	gset2.Add("banana")
	gset2.Add("orange")
	gset2.Add("kiwi")

	gset.Merge(gset2)
	fmt.Println(gset.State())
}
