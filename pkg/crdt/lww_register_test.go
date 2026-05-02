package crdt

import (
	"testing"
	"time"
)

var (
	t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = t0.Add(1 * time.Second)
	t2 = t0.Add(2 * time.Second)
	t3 = t0.Add(3 * time.Second)
)

// --- Базовые операции ---

func TestLWWRegister_GetSet(t *testing.T) {
	reg := NewLWWRegister("initial", t0, "node-1")

	if got := reg.Get(); got != "initial" {
		t.Errorf("Get() = %q, want %q", got, "initial")
	}

	reg.Set("updated", t1, "node-1")

	if got := reg.Get(); got != "updated" {
		t.Errorf("Get() after Set = %q, want %q", got, "updated")
	}
}

func TestLWWRegister_Set_IgnoresOlderTimestamp(t *testing.T) {
	reg := NewLWWRegister("current", t2, "node-1")

	// Попытка установить значение с более старым timestamp — должно быть проигнорировано
	reg.Set("old", t1, "node-1")

	if got := reg.Get(); got != "current" {
		t.Errorf("old timestamp should be ignored, Get() = %q, want %q", got, "current")
	}
}

// --- Merge: Last-Writer-Wins ---

func TestLWWRegister_Merge_NewerWins(t *testing.T) {
	a := NewLWWRegister("A-value", t1, "node-a")
	b := NewLWWRegister("B-value", t2, "node-b")

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// b имеет более поздний timestamp, поэтому его значение побеждает
	if got := a.Get(); got != "B-value" {
		t.Errorf("newer timestamp should win, Get() = %q, want %q", got, "B-value")
	}
}

func TestLWWRegister_Merge_OlderLoses(t *testing.T) {
	a := NewLWWRegister("A-value", t3, "node-a")
	b := NewLWWRegister("B-value", t1, "node-b")

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// a имеет более поздний timestamp — его значение остаётся
	if got := a.Get(); got != "A-value" {
		t.Errorf("older timestamp should lose, Get() = %q, want %q", got, "A-value")
	}
}

func TestLWWRegister_Merge_SameTimestamp_Tiebreaker(t *testing.T) {
	// При одинаковом timestamp побеждает больший ReplicaID (детерминированный tiebreaker)
	a := NewLWWRegister("A-value", t1, "node-a")
	b := NewLWWRegister("B-value", t1, "node-b")

	if err := a.Merge(b); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// "node-b" > "node-a" → B побеждает
	if got := a.Get(); got != "B-value" {
		t.Errorf("higher ReplicaID should win tiebreaker, Get() = %q, want %q", got, "B-value")
	}
}

// --- CRDT свойства ---

func TestLWWRegister_Merge_Commutativity(t *testing.T) {
	makeA := func() *LWWRegister[string] {
		return NewLWWRegister("X", t1, "node-1")
	}
	makeB := func() *LWWRegister[string] {
		return NewLWWRegister("Y", t2, "node-2")
	}

	// merge(A, B)
	ab := makeA()
	ab.Merge(makeB())

	// merge(B, A)
	ba := makeB()
	ba.Merge(makeA())

	if ab.Get() != ba.Get() {
		t.Errorf("commutativity violated: merge(A,B)=%q, merge(B,A)=%q", ab.Get(), ba.Get())
	}
}

func TestLWWRegister_Merge_Idempotency(t *testing.T) {
	reg := NewLWWRegister("value", t1, "node-1")

	before := reg.Get()
	reg.Merge(reg) // merge с самим собой

	if got := reg.Get(); got != before {
		t.Errorf("idempotency violated: before=%q, after=%q", before, got)
	}
}

// --- ApplyOperation ---

func TestLWWRegister_ApplyOperation(t *testing.T) {
	reg := NewLWWRegister("old", t0, "node-1")

	op := SetOp[string]{
		Value:       "new",
		Timestamp:   t2,
		OperationID: OpID{ReplicaID: "node-2", Counter: 1},
	}

	if err := reg.ApplyOperation(op); err != nil {
		t.Fatalf("ApplyOperation failed: %v", err)
	}

	if got := reg.Get(); got != "new" {
		t.Errorf("ApplyOperation should update value, Get() = %q, want %q", got, "new")
	}
}

func TestLWWRegister_ApplyOperation_IgnoresOlder(t *testing.T) {
	reg := NewLWWRegister("current", t3, "node-1")

	op := SetOp[string]{
		Value:       "outdated",
		Timestamp:   t1,
		OperationID: OpID{ReplicaID: "node-2", Counter: 1},
	}

	if err := reg.ApplyOperation(op); err != nil {
		t.Fatalf("ApplyOperation failed: %v", err)
	}

	if got := reg.Get(); got != "current" {
		t.Errorf("older operation should be ignored, Get() = %q, want %q", got, "current")
	}
}

// --- State ---

func TestLWWRegister_State(t *testing.T) {
	reg := NewLWWRegister("hello", t1, "node-1")

	state := reg.State()
	if state.Value != "hello" {
		t.Errorf("State().Value = %q, want %q", state.Value, "hello")
	}
	if !state.Timestamp.Equal(t1) {
		t.Errorf("State().Timestamp = %v, want %v", state.Timestamp, t1)
	}
	if state.ReplicaID != "node-1" {
		t.Errorf("State().ReplicaID = %q, want %q", state.ReplicaID, "node-1")
	}
}

// --- Generic type (int) ---

func TestLWWRegister_IntType(t *testing.T) {
	a := NewLWWRegister(100, t1, "node-1")
	b := NewLWWRegister(200, t2, "node-2")

	a.Merge(b)

	if got := a.Get(); got != 200 {
		t.Errorf("int register merge failed, Get() = %d, want 200", got)
	}
}
