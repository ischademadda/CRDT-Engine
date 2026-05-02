package crdt

import (
	"sort"
	"sync"
	"testing"
)

// --- Базовые операции ---

func TestGSet_Add_And_Contains(t *testing.T) {
	gset := NewGSet[string]()

	gset.Add("apple")
	gset.Add("banana")
	gset.Add("cherry")

	if !gset.Contains("apple") {
		t.Error("expected GSet to contain 'apple'")
	}
	if !gset.Contains("banana") {
		t.Error("expected GSet to contain 'banana'")
	}
	if gset.Contains("orange") {
		t.Error("expected GSet NOT to contain 'orange'")
	}
}

func TestGSet_Add_Idempotent(t *testing.T) {
	gset := NewGSet[string]()

	gset.Add("apple")
	gset.Add("apple")
	gset.Add("apple")

	if gset.Len() != 1 {
		t.Errorf("expected Len=1 after adding same element 3 times, got %d", gset.Len())
	}
}

func TestGSet_Len(t *testing.T) {
	gset := NewGSet[int]()

	if gset.Len() != 0 {
		t.Errorf("expected Len=0 for empty GSet, got %d", gset.Len())
	}

	for i := 0; i < 100; i++ {
		gset.Add(i)
	}

	if gset.Len() != 100 {
		t.Errorf("expected Len=100, got %d", gset.Len())
	}
}

// --- Merge: математические свойства CRDT ---

func TestGSet_Merge_Union(t *testing.T) {
	a := NewGSet[string]()
	a.Add("apple")
	a.Add("banana")
	a.Add("cherry")

	b := NewGSet[string]()
	b.Add("banana") // пересекается
	b.Add("orange")
	b.Add("kiwi")

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// a должен содержать union: apple, banana, cherry, orange, kiwi
	expected := []string{"apple", "banana", "cherry", "kiwi", "orange"}
	state := a.State()
	sort.Strings(state)

	if len(state) != len(expected) {
		t.Fatalf("expected %d elements, got %d: %v", len(expected), len(state), state)
	}

	for i, v := range expected {
		if state[i] != v {
			t.Errorf("state[%d] = %q, want %q", i, state[i], v)
		}
	}
}

func TestGSet_Merge_Commutativity(t *testing.T) {
	// Коммутативность: merge(A,B) == merge(B,A)
	makeA := func() *GSet[string] {
		s := NewGSet[string]()
		s.Add("x")
		s.Add("y")
		return s
	}
	makeB := func() *GSet[string] {
		s := NewGSet[string]()
		s.Add("y")
		s.Add("z")
		return s
	}

	// merge(A, B)
	ab := makeA()
	ab.Merge(makeB())

	// merge(B, A)
	ba := makeB()
	ba.Merge(makeA())

	stateAB := ab.State()
	stateBA := ba.State()
	sort.Strings(stateAB)
	sort.Strings(stateBA)

	if len(stateAB) != len(stateBA) {
		t.Fatalf("commutativity violated: len(AB)=%d, len(BA)=%d", len(stateAB), len(stateBA))
	}

	for i := range stateAB {
		if stateAB[i] != stateBA[i] {
			t.Errorf("commutativity violated at [%d]: AB=%q, BA=%q", i, stateAB[i], stateBA[i])
		}
	}
}

func TestGSet_Merge_Associativity(t *testing.T) {
	// Ассоциативность: merge(merge(A,B), C) == merge(A, merge(B,C))
	makeA := func() *GSet[int] {
		s := NewGSet[int]()
		s.Add(1)
		s.Add(2)
		return s
	}
	makeB := func() *GSet[int] {
		s := NewGSet[int]()
		s.Add(2)
		s.Add(3)
		return s
	}
	makeC := func() *GSet[int] {
		s := NewGSet[int]()
		s.Add(3)
		s.Add(4)
		return s
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

	state1 := ab_c.State()
	state2 := a_bc.State()
	sort.Ints(state1)
	sort.Ints(state2)

	if len(state1) != len(state2) {
		t.Fatalf("associativity violated: len1=%d, len2=%d", len(state1), len(state2))
	}

	for i := range state1 {
		if state1[i] != state2[i] {
			t.Errorf("associativity violated at [%d]: %d != %d", i, state1[i], state2[i])
		}
	}
}

func TestGSet_Merge_Idempotency(t *testing.T) {
	// Идемпотентность: merge(A,A) == A
	a := NewGSet[string]()
	a.Add("x")
	a.Add("y")
	a.Add("z")

	before := a.State()
	sort.Strings(before)

	if err := a.Merge(a); err != nil {
		t.Fatalf("self-merge failed: %v", err)
	}

	after := a.State()
	sort.Strings(after)

	if len(before) != len(after) {
		t.Fatalf("idempotency violated: len before=%d, after=%d", len(before), len(after))
	}

	for i := range before {
		if before[i] != after[i] {
			t.Errorf("idempotency violated at [%d]: %q != %q", i, before[i], after[i])
		}
	}
}

// --- ApplyOperation ---

func TestGSet_ApplyOperation(t *testing.T) {
	gset := NewGSet[string]()

	op := AddOp[string]{
		Value:       "apple",
		OperationID: OpID{ReplicaID: "node-1", Counter: 1},
	}

	if err := gset.ApplyOperation(op); err != nil {
		t.Fatalf("ApplyOperation failed: %v", err)
	}

	if !gset.Contains("apple") {
		t.Error("expected GSet to contain 'apple' after ApplyOperation")
	}
}

// --- Concurrent Safety ---

func TestGSet_ConcurrentSafety(t *testing.T) {
	gset := NewGSet[int]()

	var wg sync.WaitGroup
	workers := 10
	itemsPerWorker := 100

	// 10 горутин параллельно добавляют по 100 элементов
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for i := 0; i < itemsPerWorker; i++ {
				gset.Add(offset*itemsPerWorker + i)
			}
		}(w)
	}

	wg.Wait()

	expectedLen := workers * itemsPerWorker
	if gset.Len() != expectedLen {
		t.Errorf("expected %d elements after concurrent adds, got %d", expectedLen, gset.Len())
	}
}

func TestGSet_ConcurrentMerge(t *testing.T) {
	target := NewGSet[int]()
	target.Add(0)

	var wg sync.WaitGroup

	// 5 горутин параллельно мёржат свои сеты в один целевой
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			other := NewGSet[int]()
			for i := 1; i <= 20; i++ {
				other.Add(id*100 + i)
			}
			target.Merge(other)
		}(w)
	}

	wg.Wait()

	// 0 (original) + 5*20 = 101
	if target.Len() != 101 {
		t.Errorf("expected 101 elements after concurrent merges, got %d", target.Len())
	}
}
