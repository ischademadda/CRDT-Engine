// Package repository абстрагирует хранилище CRDT-документов.
//
// Use-case слой работает только с интерфейсом DocumentRepository, не зная,
// лежат ли документы в памяти, в PostgreSQL или где-то ещё. In-memory
// реализация предназначена для тестов и MVP; production-реализация
// (PostgreSQL/Redis snapshot) появится в фазе 4.
package repository

import (
	"context"
	"errors"

	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

// ErrDocumentNotFound возвращается, когда запрошенного документа нет в хранилище.
var ErrDocumentNotFound = errors.New("repository: document not found")

// ErrDocumentExists возвращается при попытке создать документ с уже существующим ID.
var ErrDocumentExists = errors.New("repository: document already exists")

// DocumentRepository — хранилище CRDT-документов.
//
// Контракт:
//   - Get/Save/Exists потокобезопасны.
//   - Get возвращает указатель на «живой» FugueTree; вызывающая сторона
//     несёт ответственность за корректность параллельных изменений
//     (FugueTree имеет собственный RWMutex).
//   - Create создаёт пустое дерево с заданным replicaID и регистрирует его.
//   - Save может быть no-op для in-memory, но обязан существовать как контракт
//     — production-реализация на PostgreSQL персистит снапшот.
type DocumentRepository interface {
	Get(ctx context.Context, documentID string) (*crdt.FugueTree, error)
	Create(ctx context.Context, documentID, replicaID string) (*crdt.FugueTree, error)
	GetOrCreate(ctx context.Context, documentID, replicaID string) (*crdt.FugueTree, error)
	Save(ctx context.Context, documentID string, tree *crdt.FugueTree) error
	Exists(ctx context.Context, documentID string) bool
	Delete(ctx context.Context, documentID string) error
}
