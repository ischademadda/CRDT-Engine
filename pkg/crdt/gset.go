package crdt

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

type AddOp[T any] struct {
	Value T
}

func (o AddOp[T]) OpType() string {
	return "add"
}

type GSet[T comparable] struct {
	elements map[T]struct{}
	mu       sync.RWMutex
}

func NewGSet[T comparable]() *GSet[T] {
	return &GSet[T]{
		elements: make(map[T]struct{}),
	}
}

// реализация CRDTNode

func (g *GSet[T]) Merge(other CRDTNode[[]T]) error {
	otherGSet, ok := other.(*GSet[T])
	if !ok {
		return errors.New("cannot merge with different CRDT type")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	for item := range otherGSet.elements {
		g.elements[item] = struct{}{}
	}

	return nil
}

func (g *GSet[T]) State() []T {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]T, 0, len(g.elements))
	for item := range g.elements {
		result = append(result, item)
	}

	return result
}

func (g *GSet[T]) ApplyOperation(op Operation) error {
	addOp, ok := op.(AddOp[T])
	if !ok {
		return errors.New("unsupported operation type for GSet")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.elements[addOp.Value] = struct{}{}
	return nil
}

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

func (g *GSet[T]) WriteTo(w io.Writer) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for item := range g.elements {
		_, err := fmt.Fprintln(w, item)
		if err != nil {
			return err
		}
	}

	return nil
}
