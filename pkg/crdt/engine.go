package crdt

import "fmt"

// Operation — базовый интерфейс для любой CRDT-операции (CmRDT).
// Каждая операция имеет тип (OpType) и глобально уникальный идентификатор (ID).
type Operation interface {
	// OpType возвращает строковый тип операции (например, "add", "remove", "insert").
	OpType() string

	// ID возвращает глобально уникальный идентификатор операции.
	// Формат: {ReplicaID, Counter} — обеспечивает уникальность в распределённой среде.
	ID() OpID
}

// OpID — глобальный идентификатор операции в кластере.
// Каждый узел (реплика) имеет уникальный ReplicaID, а Counter монотонно растёт.
// Пара {ReplicaID, Counter} гарантирует глобальную уникальность без координации.
type OpID struct {
	ReplicaID string
	Counter   uint64
}

// Compare возвращает -1, 0 или 1 для детерминированного порядка.
// Сначала сравнивает ReplicaID лексикографически, затем Counter.
// Используется в Fugue для порядка siblings в дереве.
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

// IsZero возвращает true для нулевого (неинициализированного) идентификатора.
func (id OpID) IsZero() bool {
	return id.ReplicaID == "" && id.Counter == 0
}

// String возвращает строковое представление для отладки.
func (id OpID) String() string {
	return fmt.Sprintf("%s:%d", id.ReplicaID, id.Counter)
}

// CRDTNode — универсальный интерфейс для любого CRDT-типа данных.
// Параметр State определяет тип материализованного состояния (например, []T для GSet, string для текста).
//
// Контракт:
//   - Merge должен быть коммутативным: merge(A,B) == merge(B,A)
//   - Merge должен быть ассоциативным: merge(merge(A,B),C) == merge(A,merge(B,C))
//   - Merge должен быть идемпотентным: merge(A,A) == A
type CRDTNode[State any] interface {
	// Merge объединяет состояние текущего узла с другим CRDT-узлом того же типа.
	// Возвращает ошибку, если типы узлов несовместимы.
	Merge(other CRDTNode[State]) error

	// ApplyOperation применяет инкрементальную операцию (CmRDT) к локальному состоянию.
	// Возвращает ошибку, если тип операции не поддерживается данным CRDT.
	ApplyOperation(op Operation) error

	// State возвращает текущее материализованное состояние CRDT.
	State() State
}
