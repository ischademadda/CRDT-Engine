package crdt

import "testing"

func TestParseDelta(t *testing.T) {
	ParseDelta("apple")
	ParseDelta([]string{"apple", "banana", "cherry"})
	ParseDelta(123)

}