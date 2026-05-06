// Package usecase реализует прикладной слой Clean Architecture.
//
// SyncUseCase — оркестратор: получает инкрементальную дельту (с WebSocket-клиента
// или из Redis Pub/Sub), применяет её к CRDT-движку, рассылает результат
// другим клиентам этого узла (WebSocket Fan-Out) и публикует на другие узлы
// (Redis Pub/Sub). DocumentUseCase — простой фасад над репозиторием для
// загрузки/создания документов.
//
// Слой намеренно НЕ импортирует пакеты `internal/websocket` и `internal/redis`,
// а работает с минимальными портами Broadcaster и Publisher. Это обеспечивает
// тестируемость (mock-порты) и развязку (можно подменить транспорт).
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ischademadda/CRDT-Engine/internal/repository"
	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

// OpType — строковые типы CRDT-операций, используемые в envelope.
const (
	OpTypeFugueInsert = "fugue_insert"
	OpTypeFugueDelete = "fugue_delete"
)

// Origin — источник дельты. Влияет на политику рассылки: локальные операции
// публикуются и в Redis (для других узлов), и в WS (для других клиентов того же узла);
// удалённые (пришедшие из Redis) — только в WS, чтобы не образовывать петли.
type Origin int

const (
	OriginLocal  Origin = iota // дельта от WebSocket-клиента этого узла
	OriginRemote               // дельта получена из Redis Pub/Sub от другого узла
)

// Delta — внутренний envelope use-case слоя.
//
// Payload хранит сырые данные операции, тип определяется Type. Это позволяет
// единообразно обрабатывать и WS-Message, и Redis-Delta, не завися от их структур.
type Delta struct {
	DocumentID   string
	Type         string
	Payload      json.RawMessage
	OriginNodeID string // ID узла-источника (для фильтрации эхо в Redis)
	SenderID     string // ID клиента-источника на этом узле (для фильтрации эхо в WS)
	Origin       Origin
}

// Broadcaster — порт для рассылки дельт WS-клиентам этого узла (Fan-Out).
type Broadcaster interface {
	Broadcast(documentID, msgType string, payload json.RawMessage, excludeSenderID string)
}

// Publisher — порт для публикации дельт другим узлам (Redis Pub/Sub).
type Publisher interface {
	Publish(ctx context.Context, documentID, msgType string, payload json.RawMessage, originNodeID string) error
}

// SyncUseCase — оркестратор синхронизации CRDT.
type SyncUseCase struct {
	repo        repository.DocumentRepository
	broadcaster Broadcaster
	publisher   Publisher
	nodeID      string
}

// NewSyncUseCase собирает оркестратор. broadcaster и publisher могут быть nil
// для тестов — в этом случае соответствующая ветвь рассылки пропускается.
func NewSyncUseCase(repo repository.DocumentRepository, b Broadcaster, p Publisher, nodeID string) *SyncUseCase {
	return &SyncUseCase{repo: repo, broadcaster: b, publisher: p, nodeID: nodeID}
}

// HandleDelta — единая точка входа для всех дельт (локальных и удалённых).
//
// Алгоритм:
//  1. Загрузить (или создать) дерево документа.
//  2. Десериализовать Payload в конкретную CRDT-операцию.
//  3. Применить операцию к дереву.
//  4. Рассылка:
//     - WebSocket Broadcast — всегда, исключая источник на этом узле.
//     - Redis Publish — только для локальных дельт (чтобы не зацикливаться).
func (uc *SyncUseCase) HandleDelta(ctx context.Context, d Delta) error {
	if d.DocumentID == "" {
		return errors.New("usecase: empty DocumentID")
	}

	tree, err := uc.repo.GetOrCreate(ctx, d.DocumentID, uc.nodeID)
	if err != nil {
		return fmt.Errorf("usecase: load document: %w", err)
	}

	op, err := decodeOp(d.Type, d.Payload)
	if err != nil {
		return fmt.Errorf("usecase: decode op: %w", err)
	}

	if err := tree.ApplyOperation(op); err != nil {
		return fmt.Errorf("usecase: apply op: %w", err)
	}

	if uc.broadcaster != nil {
		uc.broadcaster.Broadcast(d.DocumentID, d.Type, d.Payload, d.SenderID)
	}

	if d.Origin == OriginLocal && uc.publisher != nil {
		if err := uc.publisher.Publish(ctx, d.DocumentID, d.Type, d.Payload, uc.nodeID); err != nil {
			return fmt.Errorf("usecase: publish: %w", err)
		}
	}

	return nil
}

// Snapshot возвращает текущий текст документа.
func (uc *SyncUseCase) Snapshot(ctx context.Context, documentID string) (string, error) {
	tree, err := uc.repo.Get(ctx, documentID)
	if err != nil {
		return "", err
	}
	return tree.State(), nil
}

// decodeOp разбирает Payload по Type в соответствующую crdt.Operation.
func decodeOp(opType string, payload json.RawMessage) (crdt.Operation, error) {
	switch opType {
	case OpTypeFugueInsert:
		var op crdt.FugueInsertOp
		if err := json.Unmarshal(payload, &op); err != nil {
			return nil, err
		}
		return op, nil
	case OpTypeFugueDelete:
		var op crdt.FugueDeleteOp
		if err := json.Unmarshal(payload, &op); err != nil {
			return nil, err
		}
		return op, nil
	default:
		return nil, fmt.Errorf("unknown op type %q", opType)
	}
}
