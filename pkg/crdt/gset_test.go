package crdt

import (
	"os"
	"testing"
)

func TestGSet_ConcurrentAdd(t *testing.T) {
	gset := NewGSet[string]()
	gset.Add("apple")
	gset.Add("banana")
	gset.Add("cherry")

	gset_int := NewGSet[int]()
	for i := 0; i < 100; i++ {
		gset_int.Add(i)
	}

	file, _ := os.Create("output.txt")
	defer file.Close()

	gset.WriteTo(file)
	gset.WriteTo(os.Stdout)

	gset_int.WriteTo(os.Stdout)
}
