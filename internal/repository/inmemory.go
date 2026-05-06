package repository

import (
	"context"
	"sync"

	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

// InMemoryRepository — потокобезопасное in-memory хранилище документов.
// FugueTree хранится по указателю; внутренняя синхронизация дерева — за ним самим.
type InMemoryRepository struct {
	mu   sync.RWMutex
	docs map[string]*crdt.FugueTree
}

// NewInMemory создаёт пустой репозиторий.
func NewInMemory() *InMemoryRepository {
	return &InMemoryRepository{docs: make(map[string]*crdt.FugueTree)}
}

// Get возвращает дерево или ErrDocumentNotFound.
func (r *InMemoryRepository) Get(_ context.Context, documentID string) (*crdt.FugueTree, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.docs[documentID]
	if !ok {
		return nil, ErrDocumentNotFound
	}
	return t, nil
}

// Create создаёт пустое дерево. Если документ уже есть — ErrDocumentExists.
func (r *InMemoryRepository) Create(_ context.Context, documentID, replicaID string) (*crdt.FugueTree, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.docs[documentID]; ok {
		return nil, ErrDocumentExists
	}
	t := crdt.NewFugueTree(replicaID)
	r.docs[documentID] = t
	return t, nil
}

// GetOrCreate возвращает существующее дерево либо создаёт новое атомарно.
func (r *InMemoryRepository) GetOrCreate(_ context.Context, documentID, replicaID string) (*crdt.FugueTree, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.docs[documentID]; ok {
		return t, nil
	}
	t := crdt.NewFugueTree(replicaID)
	r.docs[documentID] = t
	return t, nil
}

// Save для in-memory — no-op (изменения уже применены к указателю),
// но валидирует наличие документа, чтобы поведение совпадало с контрактом
// будущих персистентных реализаций.
func (r *InMemoryRepository) Save(_ context.Context, documentID string, tree *crdt.FugueTree) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.docs[documentID]; ok && existing == tree {
		return nil
	}
	// Поведение «сохранить новое дерево» допускается, чтобы upper-layer мог
	// перезаписать состояние (например, после load-from-disk на старте).
	r.docs[documentID] = tree
	return nil
}

// Exists проверяет наличие документа.
func (r *InMemoryRepository) Exists(_ context.Context, documentID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.docs[documentID]
	return ok
}

// Delete удаляет документ. Идемпотентно — отсутствие документа не ошибка.
func (r *InMemoryRepository) Delete(_ context.Context, documentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.docs, documentID)
	return nil
}
