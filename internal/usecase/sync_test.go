package usecase

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/ischademadda/CRDT-Engine/internal/repository"
	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

type fakeBroadcaster struct {
	mu    sync.Mutex
	calls []bcastCall
}
type bcastCall struct {
	doc, typ, exclude string
	payload           json.RawMessage
}

func (f *fakeBroadcaster) Broadcast(doc, typ string, payload json.RawMessage, exclude string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, bcastCall{doc, typ, exclude, payload})
}

type fakePublisher struct {
	mu    sync.Mutex
	calls []pubCall
	err   error
}
type pubCall struct {
	doc, typ, origin string
	payload          json.RawMessage
}

func (f *fakePublisher) Publish(_ context.Context, doc, typ string, payload json.RawMessage, origin string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, pubCall{doc, typ, origin, payload})
	return f.err
}

func mustPayload(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHandleDelta_LocalInsertFanOutAndPublish(t *testing.T) {
	repo := repository.NewInMemory()
	bc := &fakeBroadcaster{}
	pub := &fakePublisher{}
	uc := NewSyncUseCase(repo, bc, pub, "node-A")

	op := crdt.FugueInsertOp{
		NodeID:   crdt.OpID{ReplicaID: "node-A", Counter: 1},
		Value:    'h',
		ParentID: crdt.OpID{ReplicaID: "__root__", Counter: 0},
		Side:     crdt.FugueRight,
	}

	err := uc.HandleDelta(context.Background(), Delta{
		DocumentID: "doc1",
		Type:       OpTypeFugueInsert,
		Payload:    mustPayload(t, op),
		SenderID:   "client-1",
		Origin:     OriginLocal,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(bc.calls) != 1 || bc.calls[0].exclude != "client-1" || bc.calls[0].doc != "doc1" {
		t.Fatalf("broadcast wrong: %+v", bc.calls)
	}
	if len(pub.calls) != 1 || pub.calls[0].origin != "node-A" {
		t.Fatalf("publish wrong: %+v", pub.calls)
	}

	got, err := uc.Snapshot(context.Background(), "doc1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "h" {
		t.Fatalf("snapshot=%q want %q", got, "h")
	}
}

func TestHandleDelta_RemoteDoesNotPublish(t *testing.T) {
	repo := repository.NewInMemory()
	bc := &fakeBroadcaster{}
	pub := &fakePublisher{}
	uc := NewSyncUseCase(repo, bc, pub, "node-A")

	op := crdt.FugueInsertOp{
		NodeID:   crdt.OpID{ReplicaID: "node-B", Counter: 1},
		Value:    'x',
		ParentID: crdt.OpID{ReplicaID: "__root__", Counter: 0},
		Side:     crdt.FugueRight,
	}
	err := uc.HandleDelta(context.Background(), Delta{
		DocumentID: "doc1",
		Type:       OpTypeFugueInsert,
		Payload:    mustPayload(t, op),
		Origin:     OriginRemote,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(bc.calls) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.calls))
	}
	if len(pub.calls) != 0 {
		t.Fatalf("remote delta must not be republished, got %d", len(pub.calls))
	}
}

func TestHandleDelta_UnknownTypeFails(t *testing.T) {
	uc := NewSyncUseCase(repository.NewInMemory(), nil, nil, "n")
	err := uc.HandleDelta(context.Background(), Delta{
		DocumentID: "d",
		Type:       "bogus",
		Payload:    json.RawMessage(`{}`),
		Origin:     OriginLocal,
	})
	if err == nil {
		t.Fatal("expected error for unknown op type")
	}
}

func TestHandleDelta_EmptyDocID(t *testing.T) {
	uc := NewSyncUseCase(repository.NewInMemory(), nil, nil, "n")
	if err := uc.HandleDelta(context.Background(), Delta{Type: OpTypeFugueInsert}); err == nil {
		t.Fatal("expected error for empty DocumentID")
	}
}

func TestHandleDelta_DeleteThenSnapshot(t *testing.T) {
	repo := repository.NewInMemory()
	uc := NewSyncUseCase(repo, nil, nil, "node-A")
	ctx := context.Background()

	tree, _ := repo.GetOrCreate(ctx, "d", "node-A")
	insertOp, err := tree.InsertAt(0, 'a')
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tree.InsertAt(1, 'b'); err != nil {
		t.Fatal(err)
	}

	delOp := crdt.FugueDeleteOp{
		TargetID: insertOp.NodeID,
		SourceID: crdt.OpID{ReplicaID: "node-A", Counter: 99},
	}
	if err := uc.HandleDelta(ctx, Delta{
		DocumentID: "d",
		Type:       OpTypeFugueDelete,
		Payload:    mustPayload(t, delOp),
		Origin:     OriginLocal,
	}); err != nil {
		t.Fatal(err)
	}

	got, _ := uc.Snapshot(ctx, "d")
	if got != "b" {
		t.Fatalf("snapshot=%q want %q", got, "b")
	}
}

func TestDocumentUseCase_LoadOrCreate(t *testing.T) {
	repo := repository.NewInMemory()
	uc := NewDocumentUseCase(repo, "n")
	tree, err := uc.LoadOrCreate(context.Background(), "doc1")
	if err != nil {
		t.Fatal(err)
	}
	if tree == nil || tree.Len() != 0 {
		t.Fatal("expected empty new tree")
	}
	text, err := uc.Text(context.Background(), "doc1")
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		t.Fatalf("text=%q want empty", text)
	}
}
