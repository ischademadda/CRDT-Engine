package crdt

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// SetOp — операция установки значения в LWW-Register.
// Реализует интерфейс Operation.
type SetOp[T any] struct {
	Value       T
	Timestamp   time.Time
	OperationID OpID
}

func (o SetOp[T]) OpType() string { return "set" }
func (o SetOp[T]) ID() OpID       { return o.OperationID }

// LWWRegister — Last-Writer-Wins Register (CRDT).
// Хранит одно значение; при конфликте побеждает запись с более поздним timestamp.
// Если timestamp совпадают, используется лексикографическое сравнение ReplicaID как tiebreaker.
//
// Математические свойства:
//   - Merge коммутативен: результат не зависит от порядка слияния
//   - Merge ассоциативен: группировка не влияет на результат
//   - Merge идемпотентен: повторное слияние безопасно
//
// Потокобезопасен через sync.RWMutex.
type LWWRegister[T any] struct {
	value     T
	timestamp time.Time
	replicaID string // для tiebreaker при совпадающих timestamp
	mu        sync.RWMutex
}

// NewLWWRegister создаёт новый регистр с начальным значением.
func NewLWWRegister[T any](value T, timestamp time.Time, replicaID string) *LWWRegister[T] {
	return &LWWRegister[T]{
		value:     value,
		timestamp: timestamp,
		replicaID: replicaID,
	}
}

// --- Реализация интерфейса CRDTNode[T] ---

// LWWState содержит материализованное состояние LWW-Register.
type LWWState[T any] struct {
	Value     T
	Timestamp time.Time
	ReplicaID string
}

// Merge объединяет текущий регистр с другим.
// Побеждает значение с более поздним timestamp.
// При совпадении timestamp побеждает больший ReplicaID (детерминированный tiebreaker).
func (r *LWWRegister[T]) Merge(other CRDTNode[LWWState[T]]) error {
	otherReg, ok := other.(*LWWRegister[T])
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *LWWRegister")
	}

	otherReg.mu.RLock()
	otherTimestamp := otherReg.timestamp
	otherValue := otherReg.value
	otherReplicaID := otherReg.replicaID
	otherReg.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if otherTimestamp.After(r.timestamp) ||
		(otherTimestamp.Equal(r.timestamp) && otherReplicaID > r.replicaID) {
		r.value = otherValue
		r.timestamp = otherTimestamp
		r.replicaID = otherReplicaID
	}

	return nil
}

// ApplyOperation применяет операцию SetOp к регистру.
// Значение обновляется только если timestamp операции новее текущего.
func (r *LWWRegister[T]) ApplyOperation(op Operation) error {
	setOp, ok := op.(SetOp[T])
	if !ok {
		return fmt.Errorf("unsupported operation type %q for LWWRegister, expected SetOp", op.OpType())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	opReplicaID := setOp.OperationID.ReplicaID

	if setOp.Timestamp.After(r.timestamp) ||
		(setOp.Timestamp.Equal(r.timestamp) && opReplicaID > r.replicaID) {
		r.value = setOp.Value
		r.timestamp = setOp.Timestamp
		r.replicaID = opReplicaID
	}

	return nil
}

// State возвращает текущее состояние регистра.
func (r *LWWRegister[T]) State() LWWState[T] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return LWWState[T]{
		Value:     r.value,
		Timestamp: r.timestamp,
		ReplicaID: r.replicaID,
	}
}

// --- Вспомогательные методы ---

// Get возвращает текущее значение регистра.
func (r *LWWRegister[T]) Get() T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.value
}

// Set устанавливает новое значение с данным timestamp.
func (r *LWWRegister[T]) Set(value T, timestamp time.Time, replicaID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if timestamp.After(r.timestamp) ||
		(timestamp.Equal(r.timestamp) && replicaID > r.replicaID) {
		r.value = value
		r.timestamp = timestamp
		r.replicaID = replicaID
	}
}
