package crdt

import (
	"sort"
	"testing"
)

// --- Базовые операции ---

func TestTwoPSet_Add_And_Contains(t *testing.T) {
	s := NewTwoPSet[string]()

	s.Add("apple")
	s.Add("banana")

	if !s.Contains("apple") {
		t.Error("expected 2P-Set to contain 'apple'")
	}
	if s.Contains("cherry") {
		t.Error("expected 2P-Set NOT to contain 'cherry'")
	}
}

func TestTwoPSet_Remove(t *testing.T) {
	s := NewTwoPSet[string]()

	s.Add("apple")
	s.Add("banana")
	s.Remove("apple")

	if s.Contains("apple") {
		t.Error("expected 'apple' to be removed")
	}
	if !s.Contains("banana") {
		t.Error("expected 'banana' to still be present")
	}
}

func TestTwoPSet_Remove_Wins_Over_ReAdd(t *testing.T) {
	// Ключевое свойство 2P-Set: удалённый элемент нельзя вернуть
	s := NewTwoPSet[string]()

	s.Add("apple")
	s.Remove("apple")
	s.Add("apple") // попытка повторного добавления

	if s.Contains("apple") {
		t.Error("2P-Set: remove should permanently win over add, but 'apple' is still present")
	}
}

func TestTwoPSet_Remove_Idempotent(t *testing.T) {
	s := NewTwoPSet[string]()

	s.Add("apple")
	s.Remove("apple")
	s.Remove("apple") // повторное удаление
	s.Remove("apple") // и ещё раз

	if s.Contains("apple") {
		t.Error("expected 'apple' to remain removed after multiple removes")
	}
}

func TestTwoPSet_Len(t *testing.T) {
	s := NewTwoPSet[string]()

	s.Add("apple")
	s.Add("banana")
	s.Add("cherry")
	s.Remove("banana")

	if got := s.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2 (apple, cherry)", got)
	}
}

// --- State ---

func TestTwoPSet_State(t *testing.T) {
	s := NewTwoPSet[string]()

	s.Add("apple")
	s.Add("banana")
	s.Add("cherry")
	s.Remove("banana")

	state := s.State()
	sort.Strings(state)

	expected := []string{"apple", "cherry"}
	if len(state) != len(expected) {
		t.Fatalf("State() has %d elements, want %d: %v", len(state), len(expected), state)
	}

	for i, v := range expected {
		if state[i] != v {
			t.Errorf("State()[%d] = %q, want %q", i, state[i], v)
		}
	}
}

// --- Merge: CRDT свойства ---

func TestTwoPSet_Merge_Union(t *testing.T) {
	a := NewTwoPSet[string]()
	a.Add("apple")
	a.Add("banana")
	a.Remove("banana")

	b := NewTwoPSet[string]()
	b.Add("banana")
	b.Add("cherry")
	b.Add("orange")
	b.Remove("orange")

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// apple: добавлен в a, не удалён → жив
	// banana: добавлен в обоих, удалён в a → union removeSet → мёртв
	// cherry: добавлен в b → жив
	// orange: добавлен в b, удалён в b → мёртв

	state := a.State()
	sort.Strings(state)

	expected := []string{"apple", "cherry"}
	if len(state) != len(expected) {
		t.Fatalf("after merge State() = %v, want %v", state, expected)
	}

	for i, v := range expected {
		if state[i] != v {
			t.Errorf("State()[%d] = %q, want %q", i, state[i], v)
		}
	}
}

func TestTwoPSet_Merge_RemoteRemove_Propagates(t *testing.T) {
	// Узел A добавил apple. Узел B добавил и удалил apple.
	// После merge: apple удалён (removeSet — union).
	a := NewTwoPSet[string]()
	a.Add("apple")

	b := NewTwoPSet[string]()
	b.Add("apple")
	b.Remove("apple")

	a.Merge(b)

	if a.Contains("apple") {
		t.Error("remote remove should propagate through merge")
	}
}

func TestTwoPSet_Merge_Commutativity(t *testing.T) {
	makeA := func() *TwoPSet[string] {
		s := NewTwoPSet[string]()
		s.Add("x")
		s.Add("y")
		s.Remove("y")
		return s
	}
	makeB := func() *TwoPSet[string] {
		s := NewTwoPSet[string]()
		s.Add("y")
		s.Add("z")
		return s
	}

	// merge(A, B)
	ab := makeA()
	ab.Merge(makeB())
	stateAB := ab.State()
	sort.Strings(stateAB)

	// merge(B, A)
	ba := makeB()
	ba.Merge(makeA())
	stateBA := ba.State()
	sort.Strings(stateBA)

	if len(stateAB) != len(stateBA) {
		t.Fatalf("commutativity violated: AB=%v, BA=%v", stateAB, stateBA)
	}

	for i := range stateAB {
		if stateAB[i] != stateBA[i] {
			t.Errorf("commutativity violated at [%d]: AB=%q, BA=%q", i, stateAB[i], stateBA[i])
		}
	}
}

func TestTwoPSet_Merge_Idempotency(t *testing.T) {
	s := NewTwoPSet[string]()
	s.Add("x")
	s.Add("y")
	s.Remove("y")

	before := s.State()
	sort.Strings(before)

	s.Merge(s) // merge с самим собой

	after := s.State()
	sort.Strings(after)

	if len(before) != len(after) {
		t.Fatalf("idempotency violated: before=%v, after=%v", before, after)
	}

	for i := range before {
		if before[i] != after[i] {
			t.Errorf("idempotency violated at [%d]: %q != %q", i, before[i], after[i])
		}
	}
}

// --- ApplyOperation ---

func TestTwoPSet_ApplyOperation_Add(t *testing.T) {
	s := NewTwoPSet[string]()

	op := AddOp[string]{
		Value:       "apple",
		OperationID: OpID{ReplicaID: "node-1", Counter: 1},
	}

	if err := s.ApplyOperation(op); err != nil {
		t.Fatalf("ApplyOperation(AddOp) failed: %v", err)
	}

	if !s.Contains("apple") {
		t.Error("expected 'apple' after AddOp")
	}
}

func TestTwoPSet_ApplyOperation_Remove(t *testing.T) {
	s := NewTwoPSet[string]()
	s.Add("apple")

	op := RemoveOp[string]{
		Value:       "apple",
		OperationID: OpID{ReplicaID: "node-1", Counter: 2},
	}

	if err := s.ApplyOperation(op); err != nil {
		t.Fatalf("ApplyOperation(RemoveOp) failed: %v", err)
	}

	if s.Contains("apple") {
		t.Error("expected 'apple' to be removed after RemoveOp")
	}
}
