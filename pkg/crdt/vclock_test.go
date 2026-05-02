package crdt

import (
	"testing"
)

// --- Базовые операции ---

func TestVectorClock_Increment_And_Get(t *testing.T) {
	vc := NewVectorClock()

	vc.Increment("node-1")
	vc.Increment("node-1")
	vc.Increment("node-2")

	if got := vc.Get("node-1"); got != 2 {
		t.Errorf("node-1 counter = %d, want 2", got)
	}
	if got := vc.Get("node-2"); got != 1 {
		t.Errorf("node-2 counter = %d, want 1", got)
	}
	if got := vc.Get("node-3"); got != 0 {
		t.Errorf("node-3 counter = %d, want 0 (absent)", got)
	}
}

func TestVectorClock_Set(t *testing.T) {
	vc := NewVectorClock()
	vc.Set("node-1", 42)

	if got := vc.Get("node-1"); got != 42 {
		t.Errorf("node-1 counter = %d, want 42", got)
	}
}

// --- Merge: математические свойства ---

func TestVectorClock_Merge_PointwiseMax(t *testing.T) {
	a := NewVectorClock()
	a.Set("node-1", 3)
	a.Set("node-2", 1)

	b := NewVectorClock()
	b.Set("node-1", 2)
	b.Set("node-2", 5)
	b.Set("node-3", 1)

	a.Merge(b)

	tests := []struct {
		replica  string
		expected uint64
	}{
		{"node-1", 3}, // max(3, 2) = 3
		{"node-2", 5}, // max(1, 5) = 5
		{"node-3", 1}, // max(0, 1) = 1
	}

	for _, tc := range tests {
		if got := a.Get(tc.replica); got != tc.expected {
			t.Errorf("after merge, %s = %d, want %d", tc.replica, got, tc.expected)
		}
	}
}

func TestVectorClock_Merge_Commutativity(t *testing.T) {
	makeA := func() *VectorClock {
		vc := NewVectorClock()
		vc.Set("x", 3)
		vc.Set("y", 1)
		return vc
	}
	makeB := func() *VectorClock {
		vc := NewVectorClock()
		vc.Set("y", 5)
		vc.Set("z", 2)
		return vc
	}

	// merge(A, B)
	ab := makeA()
	ab.Merge(makeB())

	// merge(B, A)
	ba := makeB()
	ba.Merge(makeA())

	for _, replica := range []string{"x", "y", "z"} {
		if ab.Get(replica) != ba.Get(replica) {
			t.Errorf("commutativity violated for %s: ab=%d, ba=%d",
				replica, ab.Get(replica), ba.Get(replica))
		}
	}
}

func TestVectorClock_Merge_Associativity(t *testing.T) {
	makeA := func() *VectorClock {
		vc := NewVectorClock()
		vc.Set("x", 1)
		return vc
	}
	makeB := func() *VectorClock {
		vc := NewVectorClock()
		vc.Set("x", 2)
		vc.Set("y", 3)
		return vc
	}
	makeC := func() *VectorClock {
		vc := NewVectorClock()
		vc.Set("y", 1)
		vc.Set("z", 4)
		return vc
	}

	// (A ∪ B) ∪ C
	ab_c := makeA()
	ab_c.Merge(makeB())
	ab_c.Merge(makeC())

	// A ∪ (B ∪ C)
	bc := makeB()
	bc.Merge(makeC())
	a_bc := makeA()
	a_bc.Merge(bc)

	for _, replica := range []string{"x", "y", "z"} {
		if ab_c.Get(replica) != a_bc.Get(replica) {
			t.Errorf("associativity violated for %s: (A∪B)∪C=%d, A∪(B∪C)=%d",
				replica, ab_c.Get(replica), a_bc.Get(replica))
		}
	}
}

func TestVectorClock_Merge_Idempotency(t *testing.T) {
	vc := NewVectorClock()
	vc.Set("node-1", 5)
	vc.Set("node-2", 3)

	before := vc.Copy()
	vc.Merge(vc)

	for _, replica := range []string{"node-1", "node-2"} {
		if vc.Get(replica) != before.Get(replica) {
			t.Errorf("idempotency violated for %s: before=%d, after=%d",
				replica, before.Get(replica), vc.Get(replica))
		}
	}
}

// --- Compare: каузальный порядок ---

func TestVectorClock_Compare_Equal(t *testing.T) {
	a := NewVectorClock()
	a.Set("node-1", 3)
	a.Set("node-2", 2)

	b := NewVectorClock()
	b.Set("node-1", 3)
	b.Set("node-2", 2)

	if order := a.Compare(b); order != CausalEqual {
		t.Errorf("expected CausalEqual, got %d", order)
	}
}

func TestVectorClock_Compare_Before(t *testing.T) {
	// a happened-before b: a[all] <= b[all], with at least one strict <
	a := NewVectorClock()
	a.Set("node-1", 2)
	a.Set("node-2", 3)

	b := NewVectorClock()
	b.Set("node-1", 3)
	b.Set("node-2", 3)

	if order := a.Compare(b); order != CausalBefore {
		t.Errorf("expected CausalBefore, got %d", order)
	}
}

func TestVectorClock_Compare_After(t *testing.T) {
	a := NewVectorClock()
	a.Set("node-1", 5)
	a.Set("node-2", 3)

	b := NewVectorClock()
	b.Set("node-1", 3)
	b.Set("node-2", 2)

	if order := a.Compare(b); order != CausalAfter {
		t.Errorf("expected CausalAfter, got %d", order)
	}
}

func TestVectorClock_Compare_Concurrent(t *testing.T) {
	// Конкурентные: a > b по одному компоненту, a < b по другому
	a := NewVectorClock()
	a.Set("node-1", 3)
	a.Set("node-2", 1)

	b := NewVectorClock()
	b.Set("node-1", 1)
	b.Set("node-2", 3)

	if order := a.Compare(b); order != CausalConcurrent {
		t.Errorf("expected CausalConcurrent, got %d", order)
	}
}

func TestVectorClock_Compare_DifferentKeys(t *testing.T) {
	// a знает о node-1, b знает о node-2 — конкурентны
	a := NewVectorClock()
	a.Set("node-1", 1)

	b := NewVectorClock()
	b.Set("node-2", 1)

	if order := a.Compare(b); order != CausalConcurrent {
		t.Errorf("expected CausalConcurrent for disjoint keys, got %d", order)
	}
}

// --- Copy ---

func TestVectorClock_Copy_Independence(t *testing.T) {
	original := NewVectorClock()
	original.Set("node-1", 5)

	clone := original.Copy()
	clone.Increment("node-1")

	if original.Get("node-1") != 5 {
		t.Error("modifying copy affected original")
	}
	if clone.Get("node-1") != 6 {
		t.Error("copy increment did not work")
	}
}
