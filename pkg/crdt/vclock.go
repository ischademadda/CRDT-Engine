package crdt

import (
	"fmt"
	"strings"
	"sync"
)

// VectorClock отслеживает каузальный порядок событий в распределённой системе.
// Каждый узел (реплика) имеет свой счётчик, который монотонно растёт.
//
// Используется для:
//   - Определения каузального порядка операций (happened-before)
//   - Обнаружения конкурентных операций (neither happened-before the other)
//   - Epoch-based GC: определение, когда томбстоун можно безопасно удалить
//
// Потокобезопасен через sync.RWMutex.
type VectorClock struct {
	clocks map[string]uint64
	mu     sync.RWMutex
}

// NewVectorClock создаёт пустой вектор часов.
func NewVectorClock() *VectorClock {
	return &VectorClock{
		clocks: make(map[string]uint64),
	}
}

// Increment увеличивает счётчик для данного узла на 1.
// Вызывается при каждой локальной операции на узле.
func (vc *VectorClock) Increment(replicaID string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.clocks[replicaID]++
}

// Get возвращает текущее значение счётчика для данного узла.
func (vc *VectorClock) Get(replicaID string) uint64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.clocks[replicaID]
}

// Set устанавливает значение счётчика для данного узла.
func (vc *VectorClock) Set(replicaID string, value uint64) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.clocks[replicaID] = value
}

// Merge объединяет два вектора часов, беря максимум по каждому компоненту.
//
// Математические свойства:
//   - Коммутативность: merge(A,B) == merge(B,A)
//   - Ассоциативность: merge(merge(A,B),C) == merge(A,merge(B,C))
//   - Идемпотентность: merge(A,A) == A
//
// Это операция join (верхняя грань) в полурешётке — основа CvRDT.
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

// Compare определяет каузальное отношение между двумя векторами часов.
//
// Возвращает:
//   - CausalBefore:  vc < other (vc произошло раньше other)
//   - CausalAfter:   vc > other (vc произошло позже other)
//   - CausalEqual:   vc == other (идентичные)
//   - CausalConcurrent: конкурентные (нет каузальной связи)
func (vc *VectorClock) Compare(other *VectorClock) CausalOrder {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	isLessOrEqual := true    // vc <= other по всем компонентам
	isGreaterOrEqual := true // vc >= other по всем компонентам

	// Собираем все уникальные ключи из обоих векторов
	allReplicas := make(map[string]struct{})
	for k := range vc.clocks {
		allReplicas[k] = struct{}{}
	}
	for k := range other.clocks {
		allReplicas[k] = struct{}{}
	}

	// Сравниваем по каждому компоненту (отсутствующий ключ = 0)
	for replicaID := range allReplicas {
		myVal := vc.clocks[replicaID]
		otherVal := other.clocks[replicaID]

		if myVal > otherVal {
			isLessOrEqual = false
		}
		if myVal < otherVal {
			isGreaterOrEqual = false
		}
	}

	switch {
	case isLessOrEqual && isGreaterOrEqual:
		return CausalEqual
	case isLessOrEqual:
		return CausalBefore
	case isGreaterOrEqual:
		return CausalAfter
	default:
		return CausalConcurrent
	}
}

// Copy возвращает глубокую копию вектора часов.
func (vc *VectorClock) Copy() *VectorClock {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	clone := NewVectorClock()
	for k, v := range vc.clocks {
		clone.clocks[k] = v
	}
	return clone
}

// String возвращает строковое представление для отладки.
func (vc *VectorClock) String() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	parts := make([]string, 0, len(vc.clocks))
	for k, v := range vc.clocks {
		parts = append(parts, fmt.Sprintf("%s:%d", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// CausalOrder описывает каузальное отношение между двумя событиями.
type CausalOrder int

const (
	// CausalBefore означает, что первое событие произошло строго раньше второго.
	CausalBefore CausalOrder = iota
	// CausalAfter означает, что первое событие произошло строго позже второго.
	CausalAfter
	// CausalEqual означает, что события идентичны.
	CausalEqual
	// CausalConcurrent означает, что события конкурентны (нет каузальной связи).
	CausalConcurrent
)
