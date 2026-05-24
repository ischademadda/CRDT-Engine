package crdt

import (
	"errors"
	"fmt"
	"sync"
)

// RemoveOp — операция удаления элемента из 2P-Set.
type RemoveOp[T any] struct {
	Value       T
	OperationID OpID
}

func (o RemoveOp[T]) OpType() string { return "remove" }
func (o RemoveOp[T]) ID() OpID       { return o.OperationID }

// TwoPSet — Two-Phase Set.
// Построен на двух GSet: addSet и removeSet (томбстоуны).
// Элемент присутствует, если он есть в addSet, но отсутствует в removeSet.
// Ограничение: удалённый элемент нельзя добавить повторно (remove-wins).
type TwoPSet[T comparable] struct {
	addSet    *GSet[T]
	removeSet *GSet[T]
	mu        sync.RWMutex // для атомарности Contains и State
}

func NewTwoPSet[T comparable]() *TwoPSet[T] {
	return &TwoPSet[T]{addSet: NewGSet[T](), removeSet: NewGSet[T]()}
}

// Merge объединяет оба внутренних GSet через union.
func (s *TwoPSet[T]) Merge(other CRDTNode[[]T]) error {
	otherSet, ok := other.(*TwoPSet[T])
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *TwoPSet")
	}
	if err := s.addSet.Merge(otherSet.addSet); err != nil {
		return fmt.Errorf("merge addSet: %w", err)
	}
	if err := s.removeSet.Merge(otherSet.removeSet); err != nil {
		return fmt.Errorf("merge removeSet: %w", err)
	}
	return nil
}

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

// State возвращает элементы, которые есть в addSet, но нет в removeSet.
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

// Add добавляет элемент. Если уже удалён — добавление не восстановит его.
func (s *TwoPSet[T]) Add(item T) { s.addSet.Add(item) }

// Remove помечает элемент как удалённый. Повторное удаление безопасно.
func (s *TwoPSet[T]) Remove(item T) { s.removeSet.Add(item) }

func (s *TwoPSet[T]) Contains(item T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addSet.Contains(item) && !s.removeSet.Contains(item)
}

func (s *TwoPSet[T]) Len() int { return len(s.State()) }
