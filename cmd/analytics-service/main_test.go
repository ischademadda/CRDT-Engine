package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/ischademadda/CRDT-Engine/internal/redis"
	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

type mockStatsRecorder struct {
	mu      sync.Mutex
	inserts map[string][]string // docID -> list of replicaIDs
	deletes map[string][]string // docID -> list of replicaIDs
}

func newMockStatsRecorder() *mockStatsRecorder {
	return &mockStatsRecorder{
		inserts: make(map[string][]string),
		deletes: make(map[string][]string),
	}
}

func (m *mockStatsRecorder) RecordInsert(ctx context.Context, docID string, replicaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inserts[docID] = append(m.inserts[docID], replicaID)
	return nil
}

func (m *mockStatsRecorder) RecordDelete(ctx context.Context, docID string, replicaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletes[docID] = append(m.deletes[docID], replicaID)
	return nil
}

func (m *mockStatsRecorder) getInserts(docID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inserts[docID]
}

func (m *mockStatsRecorder) getDeletes(docID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deletes[docID]
}

func TestRunRedisPatternConsumer(t *testing.T) {
	// 1. Запускаем miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	// 2. Создаем клиент go-redis
	rclient := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer rclient.Close()

	// 3. Создаем mock StatsRecorder
	mockRecorder := newMockStatsRecorder()

	// 4. Запускаем consumer в отдельной горутине
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runRedisPatternConsumer(ctx, rclient, mockRecorder)

	// Даем немного времени на подписку
	time.Sleep(100 * time.Millisecond)

	// 5. Генерируем тестовую операцию вставки
	insertOp := crdt.FugueInsertOp{
		NodeID: crdt.OpID{ReplicaID: "replica-tester-1", Counter: 10},
		Value:  'A',
	}
	insertPayload, _ := json.Marshal(insertOp)

	insertDelta := redis.Delta{
		DocumentID:   "doc-test-123",
		OriginNodeID: "node-xyz",
		Type:         "fugue_insert",
		Payload:      insertPayload,
	}
	deltaJSON, _ := json.Marshal(insertDelta)

	// Публикуем в Redis в канал "crdt:doc:doc-test-123"
	// (так как подписка идет на паттерн "crdt:doc:*")
	mr.Publish("crdt:doc:doc-test-123", string(deltaJSON))

	// Ждем обработки сообщения
	time.Sleep(150 * time.Millisecond)

	// Проверяем, что вставка записалась
	inserts := mockRecorder.getInserts("doc-test-123")
	if len(inserts) != 1 {
		t.Errorf("expected 1 insert, got %d", len(inserts))
	} else if inserts[0] != "replica-tester-1" {
		t.Errorf("expected replica ID 'replica-tester-1', got %q", inserts[0])
	}

	// 6. Генерируем тестовую операцию удаления
	deleteOp := crdt.FugueDeleteOp{
		TargetID: crdt.OpID{ReplicaID: "replica-tester-1", Counter: 10},
		SourceID: crdt.OpID{ReplicaID: "replica-tester-2", Counter: 11},
	}
	deletePayload, _ := json.Marshal(deleteOp)

	deleteDelta := redis.Delta{
		DocumentID:   "doc-test-123",
		OriginNodeID: "node-xyz",
		Type:         "fugue_delete",
		Payload:      deletePayload,
	}
	deleteDeltaJSON, _ := json.Marshal(deleteDelta)

	// Публикуем
	mr.Publish("crdt:doc:doc-test-123", string(deleteDeltaJSON))

	time.Sleep(150 * time.Millisecond)

	// Проверяем удаление
	deletes := mockRecorder.getDeletes("doc-test-123")
	if len(deletes) != 1 {
		t.Errorf("expected 1 delete, got %d", len(deletes))
	} else if deletes[0] != "replica-tester-2" {
		t.Errorf("expected replica ID 'replica-tester-2', got %q", deletes[0])
	}
}
