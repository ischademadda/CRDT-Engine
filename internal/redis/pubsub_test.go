package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	rds "github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) (*miniredis.Miniredis, rds.UniversalClient) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	c := rds.NewClient(&rds.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return mr, c
}

func waitForDelta(t *testing.T, ch <-chan Delta, timeout time.Duration) Delta {
	t.Helper()
	select {
	case d, ok := <-ch:
		if !ok {
			t.Fatal("channel closed")
		}
		return d
	case <-time.After(timeout):
		t.Fatal("timeout waiting for delta")
	}
	return Delta{}
}

func TestPubSub_RoundTrip(t *testing.T) {
	_, client := newTestClient(t)
	ctx := context.Background()

	sub := NewSubscriber(client, 0)
	defer sub.Close()
	if err := sub.Subscribe(ctx, "doc1"); err != nil {
		t.Fatal(err)
	}

	pub := NewPublisher(client)

	// miniredis subscribe требует немного времени — небольшая пауза перед публикацией.
	time.Sleep(20 * time.Millisecond)

	delta := Delta{
		DocumentID:   "doc1",
		OriginNodeID: "node-A",
		Type:         "fugue_insert",
		Payload:      json.RawMessage(`{"v":1}`),
	}
	if err := pub.Publish(ctx, delta); err != nil {
		t.Fatalf("publish: %v", err)
	}

	got := waitForDelta(t, sub.Messages(), 2*time.Second)
	if got.DocumentID != delta.DocumentID || got.OriginNodeID != delta.OriginNodeID || got.Type != delta.Type {
		t.Fatalf("delta mismatch: got %+v", got)
	}
}

func TestPubSub_DocumentIsolation(t *testing.T) {
	_, client := newTestClient(t)
	ctx := context.Background()

	sub := NewSubscriber(client, 0)
	defer sub.Close()
	if err := sub.Subscribe(ctx, "doc-A"); err != nil {
		t.Fatal(err)
	}

	pub := NewPublisher(client)
	time.Sleep(20 * time.Millisecond)

	if err := pub.Publish(ctx, Delta{DocumentID: "doc-B", Type: "x", Payload: json.RawMessage(`null`)}); err != nil {
		t.Fatal(err)
	}

	select {
	case d := <-sub.Messages():
		t.Fatalf("leaked from doc-B: %+v", d)
	case <-time.After(150 * time.Millisecond):
		// ok
	}
}

func TestSubscribe_Idempotent(t *testing.T) {
	_, client := newTestClient(t)
	ctx := context.Background()

	sub := NewSubscriber(client, 0)
	defer sub.Close()
	for i := 0; i < 3; i++ {
		if err := sub.Subscribe(ctx, "d"); err != nil {
			t.Fatalf("subscribe #%d: %v", i, err)
		}
	}
}

func TestPublish_EmptyDocID(t *testing.T) {
	_, client := newTestClient(t)
	pub := NewPublisher(client)
	if err := pub.Publish(context.Background(), Delta{Type: "x"}); err == nil {
		t.Fatal("expected error for empty DocumentID")
	}
}

func TestSubscriber_CloseIsIdempotent(t *testing.T) {
	_, client := newTestClient(t)
	sub := NewSubscriber(client, 0)
	if err := sub.Subscribe(context.Background(), "d"); err != nil {
		t.Fatal(err)
	}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sub.Close(); err != nil {
		t.Fatalf("second close should be no-op, got %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	_, client := newTestClient(t)
	ctx := context.Background()

	sub := NewSubscriber(client, 0)
	defer sub.Close()
	if err := sub.Subscribe(ctx, "d"); err != nil {
		t.Fatal(err)
	}
	if err := sub.Unsubscribe(ctx, "d"); err != nil {
		t.Fatal(err)
	}
	// Повторный Unsubscribe — no-op.
	if err := sub.Unsubscribe(ctx, "d"); err != nil {
		t.Fatal(err)
	}
}
