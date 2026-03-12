package crdt

import (
	"testing"
)

func TestGSet_AddAndContains(t *testing.T) {
	gset := NewGSet()
	
	if gset.Contains("apple") {
		t.Errorf("Expected 'apple' to NOT be in the set")
	}

	gset.Add("apple")

	if !gset.Contains("apple") {
		t.Errorf("Expected 'apple' to be in the set")
	}
}