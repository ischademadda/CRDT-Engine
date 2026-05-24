package crdt

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// AddOp — операция добавления элемента в GSet.
type AddOp[T any] struct {
	Value       T
	OperationID OpID
}

func (o AddOp[T]) OpType() string { return "add" }
func (o AddOp[T]) ID() OpID       { return o.OperationID }

// GSet — Grow-only Set. Элементы можно только добавлять.
// Merge = union множеств. Потокобезопасен через sync.RWMutex.
type GSet[T comparable] struct {
	elements map[T]struct{}
	mu       sync.RWMutex
}

func NewGSet[T comparable]() *GSet[T] {
	return &GSet[T]{elements: make(map[T]struct{})}
}

// Merge объединяет текущий GSet с другим через union.
func (g *GSet[T]) Merge(other CRDTNode[[]T]) error {
	otherGSet, ok := other.(*GSet[T])
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *GSet")
	}

	// Снимок чужого состояния под RLock, чтобы не держать два лока одновременно.
	otherGSet.mu.RLock()
	snapshot := make(map[T]struct{}, len(otherGSet.elements))
	for item := range otherGSet.elements {
		snapshot[item] = struct{}{}
	}
	otherGSet.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()
	for item := range snapshot {
		g.elements[item] = struct{}{}
	}
	return nil
}

func (g *GSet[T]) ApplyOperation(op Operation) error {
	addOp, ok := op.(AddOp[T])
	if !ok {
		return fmt.Errorf("unsupported operation type %q for GSet, expected AddOp", op.OpType())
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.elements[addOp.Value] = struct{}{}
	return nil
}

// State возвращает содержимое GSet как slice. Порядок не гарантирован.
func (g *GSet[T]) State() []T {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]T, 0, len(g.elements))
	for item := range g.elements {
		result = append(result, item)
	}
	return result
}

// Add добавляет элемент напрямую, без Operation-обёртки.
func (g *GSet[T]) Add(item T) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.elements[item] = struct{}{}
}

func (g *GSet[T]) Contains(item T) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.elements[item]
	return exists
}

func (g *GSet[T]) Len() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.elements)
}

// WriteTo записывает все элементы в writer (по одному на строку).
func (g *GSet[T]) WriteTo(w io.Writer) (int64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var total int64
	for item := range g.elements {
		n, err := fmt.Fprintln(w, item)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
