package crdt

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// SetOp — операция установки значения в LWW-Register.
type SetOp[T any] struct {
	Value       T
	Timestamp   time.Time
	OperationID OpID
}

func (o SetOp[T]) OpType() string { return "set" }
func (o SetOp[T]) ID() OpID       { return o.OperationID }

// LWWRegister — Last-Writer-Wins Register.
// При конфликте побеждает запись с более поздним timestamp.
// Если timestamp совпадают — tiebreaker по ReplicaID (лексикографически больший побеждает).
type LWWRegister[T any] struct {
	value     T
	timestamp time.Time
	replicaID string
	mu        sync.RWMutex
}

func NewLWWRegister[T any](value T, timestamp time.Time, replicaID string) *LWWRegister[T] {
	return &LWWRegister[T]{value: value, timestamp: timestamp, replicaID: replicaID}
}

// LWWState — материализованное состояние регистра.
type LWWState[T any] struct {
	Value     T
	Timestamp time.Time
	ReplicaID string
}

func (r *LWWRegister[T]) Merge(other CRDTNode[LWWState[T]]) error {
	otherReg, ok := other.(*LWWRegister[T])
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *LWWRegister")
	}

	otherReg.mu.RLock()
	ts, val, rid := otherReg.timestamp, otherReg.value, otherReg.replicaID
	otherReg.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if ts.After(r.timestamp) || (ts.Equal(r.timestamp) && rid > r.replicaID) {
		r.value, r.timestamp, r.replicaID = val, ts, rid
	}
	return nil
}

func (r *LWWRegister[T]) ApplyOperation(op Operation) error {
	setOp, ok := op.(SetOp[T])
	if !ok {
		return fmt.Errorf("unsupported operation type %q for LWWRegister, expected SetOp", op.OpType())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rid := setOp.OperationID.ReplicaID
	if setOp.Timestamp.After(r.timestamp) || (setOp.Timestamp.Equal(r.timestamp) && rid > r.replicaID) {
		r.value, r.timestamp, r.replicaID = setOp.Value, setOp.Timestamp, rid
	}
	return nil
}

func (r *LWWRegister[T]) State() LWWState[T] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return LWWState[T]{Value: r.value, Timestamp: r.timestamp, ReplicaID: r.replicaID}
}

func (r *LWWRegister[T]) Get() T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.value
}

func (r *LWWRegister[T]) Set(value T, timestamp time.Time, replicaID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if timestamp.After(r.timestamp) || (timestamp.Equal(r.timestamp) && replicaID > r.replicaID) {
		r.value, r.timestamp, r.replicaID = value, timestamp, replicaID
	}
}
