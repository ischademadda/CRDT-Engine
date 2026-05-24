package crdt

import "fmt"

// Operation — любая CRDT-операция: имеет тип и глобально уникальный ID.
type Operation interface {
	OpType() string
	ID() OpID
}

// OpID — глобальный идентификатор операции {ReplicaID, Counter}.
// Уникальность без координации: каждый узел имеет свой ReplicaID и монотонный счётчик.
type OpID struct {
	ReplicaID string
	Counter   uint64
}

// Compare возвращает -1, 0 или 1. Сначала по ReplicaID, потом по Counter.
// Используется в Fugue для детерминированного порядка siblings.
func (id OpID) Compare(other OpID) int {
	if id.ReplicaID < other.ReplicaID {
		return -1
	}
	if id.ReplicaID > other.ReplicaID {
		return 1
	}
	if id.Counter < other.Counter {
		return -1
	}
	if id.Counter > other.Counter {
		return 1
	}
	return 0
}

func (id OpID) IsZero() bool {
	return id.ReplicaID == "" && id.Counter == 0
}

func (id OpID) String() string {
	return fmt.Sprintf("%s:%d", id.ReplicaID, id.Counter)
}

// CRDTNode — универсальный интерфейс для любого CRDT-типа.
// Контракт: Merge должен быть коммутативным, ассоциативным и идемпотентным.
type CRDTNode[State any] interface {
	Merge(other CRDTNode[State]) error
	ApplyOperation(op Operation) error
	State() State
}
