package crdt

import (
	"sync"
)

type GSet struct {
	data map[string]struct{}
	mu   sync.RWMutex
}

func NewGSet() *GSet {
	return &GSet{
		data: make(map[string]struct{}),
	}
}

func (g *GSet) Add(item string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.data[item] = struct{}{}

}

func (g *GSet) Contains(item string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.data[item]

	return exists
}
