package crdt

import (
	"fmt"
	"strings"
	"sync"
)

// VectorClock отслеживает каузальный порядок событий в распределённой системе.
// Каждая реплика имеет свой счётчик, который монотонно растёт при каждой локальной операции.
// Используется для обнаружения конкурентных операций и (в будущем) Epoch-based GC томбстоунов.
type VectorClock struct {
	clocks map[string]uint64
	mu     sync.RWMutex
}

func NewVectorClock() *VectorClock {
	return &VectorClock{clocks: make(map[string]uint64)}
}

func (vc *VectorClock) Increment(replicaID string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.clocks[replicaID]++
}

func (vc *VectorClock) Get(replicaID string) uint64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.clocks[replicaID]
}

func (vc *VectorClock) Set(replicaID string, value uint64) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.clocks[replicaID] = value
}

// Merge берёт максимум по каждому компоненту — это join в полурешётке (CvRDT).
func (vc *VectorClock) Merge(other *VectorClock) {
	other.mu.RLock()
	snapshot := make(map[string]uint64, len(other.clocks))
	for k, v := range other.clocks {
		snapshot[k] = v
	}
	other.mu.RUnlock()

	vc.mu.Lock()
	defer vc.mu.Unlock()
	for replicaID, otherVal := range snapshot {
		if otherVal > vc.clocks[replicaID] {
			vc.clocks[replicaID] = otherVal
		}
	}
}

// Compare определяет каузальное отношение: Before, After, Equal или Concurrent.
func (vc *VectorClock) Compare(other *VectorClock) CausalOrder {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	lessOrEqual := true
	greaterOrEqual := true

	allReplicas := make(map[string]struct{})
	for k := range vc.clocks {
		allReplicas[k] = struct{}{}
	}
	for k := range other.clocks {
		allReplicas[k] = struct{}{}
	}

	for replicaID := range allReplicas {
		my, their := vc.clocks[replicaID], other.clocks[replicaID]
		if my > their {
			lessOrEqual = false
		}
		if my < their {
			greaterOrEqual = false
		}
	}

	switch {
	case lessOrEqual && greaterOrEqual:
		return CausalEqual
	case lessOrEqual:
		return CausalBefore
	case greaterOrEqual:
		return CausalAfter
	default:
		return CausalConcurrent
	}
}

func (vc *VectorClock) Copy() *VectorClock {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	clone := NewVectorClock()
	for k, v := range vc.clocks {
		clone.clocks[k] = v
	}
	return clone
}

func (vc *VectorClock) String() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	parts := make([]string, 0, len(vc.clocks))
	for k, v := range vc.clocks {
		parts = append(parts, fmt.Sprintf("%s:%d", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// CausalOrder — результат сравнения двух векторных часов.
type CausalOrder int

const (
	CausalBefore     CausalOrder = iota // vc произошло строго раньше
	CausalAfter                         // vc произошло строго позже
	CausalEqual                         // идентичные часы
	CausalConcurrent                    // конкурентные события
)
