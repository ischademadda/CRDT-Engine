package crdt

import (
	"errors"
	"fmt"
	"sync"
)

// RemoveOp — операция удаления элемента из 2P-Set.
// Реализует интерфейс Operation.
type RemoveOp[T any] struct {
	Value       T
	OperationID OpID
}

func (o RemoveOp[T]) OpType() string { return "remove" }
func (o RemoveOp[T]) ID() OpID       { return o.OperationID }

// TwoPSet — Two-Phase Set (CRDT).
// Построен на двух GSet: addSet (добавления) и removeSet (удаления, «томбстоуны»).
// Элемент считается присутствующим, если он есть в addSet, но отсутствует в removeSet.
//
// Ограничения:
//   - Удалённый элемент НЕЛЬЗЯ добавить повторно (remove wins).
//   - Это фундаментальное свойство 2P-Set, решаемое в OR-Set (Observed-Remove Set).
//
// Математические свойства Merge:
//   - Коммутативность, ассоциативность, идемпотентность
//   - Merge = union обоих внутренних GSet
//
// Потокобезопасен: делегирует безопасность внутренним GSet (каждый с RWMutex).
type TwoPSet[T comparable] struct {
	addSet    *GSet[T]
	removeSet *GSet[T]
	mu        sync.RWMutex // для атомарности Contains и State
}

// NewTwoPSet создаёт новый пустой 2P-Set.
func NewTwoPSet[T comparable]() *TwoPSet[T] {
	return &TwoPSet[T]{
		addSet:    NewGSet[T](),
		removeSet: NewGSet[T](),
	}
}

// --- Реализация интерфейса CRDTNode[[]T] ---

// Merge объединяет два 2P-Set: union для addSet и union для removeSet.
func (s *TwoPSet[T]) Merge(other CRDTNode[[]T]) error {
	otherSet, ok := other.(*TwoPSet[T])
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *TwoPSet")
	}

	// Merge каждого внутреннего GSet
	if err := s.addSet.Merge(otherSet.addSet); err != nil {
		return fmt.Errorf("merge addSet: %w", err)
	}
	if err := s.removeSet.Merge(otherSet.removeSet); err != nil {
		return fmt.Errorf("merge removeSet: %w", err)
	}

	return nil
}

// ApplyOperation применяет AddOp или RemoveOp к 2P-Set.
func (s *TwoPSet[T]) ApplyOperation(op Operation) error {
	switch typedOp := op.(type) {
	case AddOp[T]:
		s.Add(typedOp.Value)
		return nil
	case RemoveOp[T]:
		s.Remove(typedOp.Value)
		return nil
	default:
		return fmt.Errorf("unsupported operation type %q for TwoPSet", op.OpType())
	}
}

// State возвращает элементы, которые есть в addSet, но НЕТ в removeSet.
func (s *TwoPSet[T]) State() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	added := s.addSet.State()
	result := make([]T, 0, len(added))
	for _, item := range added {
		if !s.removeSet.Contains(item) {
			result = append(result, item)
		}
	}

	return result
}

// --- Вспомогательные методы ---

// Add добавляет элемент в addSet.
// Если элемент уже был удалён (есть в removeSet), добавление не восстановит его.
func (s *TwoPSet[T]) Add(item T) {
	s.addSet.Add(item)
}

// Remove помечает элемент как удалённый (добавляет в removeSet).
// Элемент может быть удалён, только если он был ранее добавлен.
// Повторное удаление безопасно (идемпотентность).
func (s *TwoPSet[T]) Remove(item T) {
	s.removeSet.Add(item)
}

// Contains проверяет, присутствует ли элемент (добавлен И не удалён).
func (s *TwoPSet[T]) Contains(item T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addSet.Contains(item) && !s.removeSet.Contains(item)
}

// Len возвращает количество «живых» элементов (добавленных минус удалённые).
func (s *TwoPSet[T]) Len() int {
	return len(s.State())
}
