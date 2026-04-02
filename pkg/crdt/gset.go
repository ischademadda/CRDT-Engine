package crdt

import (
	"fmt"
	"io"
	"sync"
)

type GSet[T comparable] struct {
	data map[T]struct{}
	mu   sync.RWMutex
}

func NewGSet[T comparable]() *GSet[T] {
	return &GSet[T]{
		data: make(map[T]struct{}),
	}
}

func (g *GSet[T]) Add(item T) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.data[item] = struct{}{}

}

func (g *GSet[T]) Contains(item T) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.data[item]

	return exists
}

func (g *GSet[T]) WriteTo(w io.Writer) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for item := range g.data {
		_, err := fmt.Fprintln(w, item)
		if err != nil {
			return err
		}
	}

	return nil
}
