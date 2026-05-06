package usecase

import (
	"context"

	"github.com/ischademadda/CRDT-Engine/internal/repository"
	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

// DocumentUseCase — операции жизненного цикла документа (create/load/snapshot).
type DocumentUseCase struct {
	repo   repository.DocumentRepository
	nodeID string
}

func NewDocumentUseCase(repo repository.DocumentRepository, nodeID string) *DocumentUseCase {
	return &DocumentUseCase{repo: repo, nodeID: nodeID}
}

// LoadOrCreate возвращает дерево документа, создавая пустое при отсутствии.
func (uc *DocumentUseCase) LoadOrCreate(ctx context.Context, documentID string) (*crdt.FugueTree, error) {
	return uc.repo.GetOrCreate(ctx, documentID, uc.nodeID)
}

// Text возвращает текущий текст документа.
func (uc *DocumentUseCase) Text(ctx context.Context, documentID string) (string, error) {
	tree, err := uc.repo.GetOrCreate(ctx, documentID, uc.nodeID)
	if err != nil {
		return "", err
	}
	return tree.State(), nil
}
