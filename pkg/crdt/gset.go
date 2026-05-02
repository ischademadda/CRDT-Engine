package crdt

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

// AddOp — операция добавления элемента в GSet.
// Реализует интерфейс Operation.
type AddOp[T any] struct {
	Value     T
	OperationID OpID
}

func (o AddOp[T]) OpType() string { return "add" }
func (o AddOp[T]) ID() OpID       { return o.OperationID }

// GSet — Grow-only Set (CRDT).
// Элементы можно только добавлять, удаление невозможно.
// Merge двух GSet — это объединение (union) множеств.
//
// Математические свойства:
//   - Merge коммутативен: A ∪ B == B ∪ A
//   - Merge ассоциативен: (A ∪ B) ∪ C == A ∪ (B ∪ C)
//   - Merge идемпотентен: A ∪ A == A
//
// Потокобезопасен через sync.RWMutex.
type GSet[T comparable] struct {
	elements map[T]struct{}
	mu       sync.RWMutex
}

// NewGSet создаёт новый пустой GSet.
func NewGSet[T comparable]() *GSet[T] {
	return &GSet[T]{
		elements: make(map[T]struct{}),
	}
}

// --- Реализация интерфейса CRDTNode[[]T] ---

// Merge объединяет текущий GSet с другим.
// Операция union: все элементы из other добавляются в текущий сет.
// Возвращает ошибку, если other не является *GSet[T].
func (g *GSet[T]) Merge(other CRDTNode[[]T]) error {
	otherGSet, ok := other.(*GSet[T])
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *GSet")
	}

	// Блокируем оба сета в правильном порядке для предотвращения deadlock.
	// Сначала читаем из other, потом пишем в g.
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

// ApplyOperation применяет операцию к GSet.
// Поддерживается только AddOp[T].
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

// State возвращает текущее содержимое GSet как slice.
// Порядок элементов не гарантирован (природа map в Go).
func (g *GSet[T]) State() []T {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]T, 0, len(g.elements))
	for item := range g.elements {
		result = append(result, item)
	}

	return result
}

// --- Вспомогательные методы ---

// Add добавляет элемент напрямую (без Operation обёртки).
// Удобно для локального использования без распределённой синхронизации.
func (g *GSet[T]) Add(item T) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.elements[item] = struct{}{}
}

// Contains проверяет наличие элемента в сете.
func (g *GSet[T]) Contains(item T) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.elements[item]
	return exists
}

// Len возвращает количество элементов в сете.
func (g *GSet[T]) Len() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.elements)
}

// WriteTo записывает все элементы в writer (по одному на строку).
// Реализует паттерн io.WriterTo для интеграции с файловой системой.
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
