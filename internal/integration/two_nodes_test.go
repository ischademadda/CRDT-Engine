package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	rds "github.com/ischademadda/CRDT-Engine/internal/redis"
	"github.com/ischademadda/CRDT-Engine/internal/repository"
	"github.com/ischademadda/CRDT-Engine/internal/usecase"
	"github.com/ischademadda/CRDT-Engine/pkg/crdt"
)

// nodeStack mirrors what cmd/demo-app builds for a single instance: an
// in-memory repo, a SyncUseCase, and a Redis subscriber wired to dispatch
// remote deltas back through HandleDelta. It deliberately does NOT include a
// WebSocket Hub — those are exercised in internal/websocket/hub_test.go and
// are not what this test is trying to verify.
type nodeStack struct {
	id     string
	repo   *repository.InMemoryRepository
	uc     *usecase.SyncUseCase
	sub    *rds.Subscriber
	pub    *rds.Publisher
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// pubAdapter mirrors cmd/demo-app/main.go: bridges the use-case Publisher
// port to the concrete redis.Publisher.
type pubAdapter struct{ pub *rds.Publisher }

func (a *pubAdapter) Publish(ctx context.Context, doc, typ string, payload json.RawMessage, origin string) error {
	return a.pub.Publish(ctx, rds.Delta{
		DocumentID:   doc,
		OriginNodeID: origin,
		Type:         typ,
		Payload:      payload,
	})
}

func newNodeStack(t *testing.T, id, docID string, client goredis.UniversalClient) *nodeStack {
	t.Helper()

	repo := repository.NewInMemory()
	pub := rds.NewPublisher(client)
	sub := rds.NewSubscriber(client, 0)
	if err := sub.Subscribe(context.Background(), docID); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	uc := usecase.NewSyncUseCase(repo, nil /* no WS broadcaster in this test */, &pubAdapter{pub: pub}, id)

	n := &nodeStack{
		id:     id,
		repo:   repo,
		uc:     uc,
		sub:    sub,
		pub:    pub,
		stopCh: make(chan struct{}),
	}

	// Run the redis-fan-in goroutine the same way cmd/demo-app does:
	// drop our own echoes, dispatch everything else as OriginRemote.
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for {
			select {
			case <-n.stopCh:
				return
			case d, ok := <-sub.Messages():
				if !ok {
					return
				}
				if d.OriginNodeID == n.id {
					continue
				}
				_ = n.uc.HandleDelta(context.Background(), usecase.Delta{
					DocumentID:   d.DocumentID,
					Type:         d.Type,
					Payload:      d.Payload,
					OriginNodeID: d.OriginNodeID,
					Origin:       usecase.OriginRemote,
				})
			}
		}
	}()

	t.Cleanup(n.close)
	return n
}

func (n *nodeStack) close() {
	select {
	case <-n.stopCh:
		return
	default:
	}
	close(n.stopCh)
	_ = n.sub.Close()
	n.wg.Wait()
}

// localInsert applies an InsertAt on this node's tree, then routes the
// resulting op through HandleDelta as OriginLocal — the same path the demo
// dispatcher takes on a WebSocket intent.
func (n *nodeStack) localInsert(t *testing.T, doc string, pos int, ch rune) {
	t.Helper()
	tree, err := n.repo.GetOrCreate(context.Background(), doc, n.id)
	if err != nil {
		t.Fatalf("%s: get tree: %v", n.id, err)
	}
	op, err := tree.InsertAt(pos, ch)
	if err != nil {
		t.Fatalf("%s: InsertAt: %v", n.id, err)
	}
	payload, _ := json.Marshal(op)
	if err := n.uc.HandleDelta(context.Background(), usecase.Delta{
		DocumentID: doc,
		Type:       usecase.OpTypeFugueInsert,
		Payload:    payload,
		Origin:     usecase.OriginLocal,
	}); err != nil {
		t.Fatalf("%s: HandleDelta: %v", n.id, err)
	}
}

func (n *nodeStack) snapshot(t *testing.T, doc string) string {
	t.Helper()
	s, err := n.uc.Snapshot(context.Background(), doc)
	if err != nil {
		// Document may not exist yet on this node — return empty.
		return ""
	}
	return s
}

// waitForSnapshot polls until the node's view of doc satisfies want or the
// deadline expires. The propagation path is goroutine-driven (subscriber →
// fan-in goroutine → use-case), so a short bounded wait is correct.
func waitForSnapshot(t *testing.T, n *nodeStack, doc, want string, deadline time.Duration) {
	t.Helper()
	start := time.Now()
	for time.Since(start) < deadline {
		if got := n.snapshot(t, doc); got == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s: snapshot of %q did not converge to %q within %v (last=%q)",
		n.id, doc, want, deadline, n.snapshot(t, doc))
}

func newRedis(t *testing.T) goredis.UniversalClient {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	c := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestTwoNodes_LocalEditPropagates is the canonical wiring test: a write on
// node A must be visible on node B's tree after Pub/Sub propagation. If this
// breaks, something between the use-case's republish rule, the Redis
// publisher, the subscriber, and the remote-side dispatch is broken.
func TestTwoNodes_LocalEditPropagates(t *testing.T) {
	const doc = "doc1"
	client := newRedis(t)

	a := newNodeStack(t, "node-a", doc, client)
	b := newNodeStack(t, "node-b", doc, client)

	// miniredis needs a brief moment to register the SUBSCRIBE before the
	// first PUBLISH — this is the same 20ms grace used in pubsub_test.go.
	time.Sleep(20 * time.Millisecond)

	a.localInsert(t, doc, 0, 'H')
	a.localInsert(t, doc, 1, 'i')

	waitForSnapshot(t, b, doc, "Hi", 2*time.Second)
	if got := a.snapshot(t, doc); got != "Hi" {
		t.Fatalf("node-a snapshot=%q want %q", got, "Hi")
	}
}

// TestTwoNodes_NoEchoLoop covers the rule encoded in
// SyncUseCase.HandleDelta: a delta that arrives via Redis (OriginRemote)
// must NOT be republished, otherwise nodes ping-pong each other's writes.
//
// We assert the property indirectly: after a single local insert on A, the
// total number of frames seen on the wire over a quiet period must be 1, not
// growing without bound.
func TestTwoNodes_NoEchoLoop(t *testing.T) {
	const doc = "doc-echo"
	client := newRedis(t)

	// Side-channel subscriber that just counts frames on the document
	// channel — independent of the nodes' own subscribers.
	counter := goredis.NewClient(&goredis.Options{Addr: client.(*goredis.Client).Options().Addr})
	t.Cleanup(func() { _ = counter.Close() })
	ps := counter.Subscribe(context.Background(), "crdt:doc:"+doc)
	t.Cleanup(func() { _ = ps.Close() })

	a := newNodeStack(t, "node-a", doc, client)
	_ = newNodeStack(t, "node-b", doc, client)

	time.Sleep(20 * time.Millisecond)

	var (
		mu    sync.Mutex
		count int
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ch := ps.Channel()
		timer := time.NewTimer(500 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-ch:
				mu.Lock()
				count++
				mu.Unlock()
			case <-timer.C:
				return
			}
		}
	}()

	a.localInsert(t, doc, 0, 'X')

	<-done
	mu.Lock()
	got := count
	mu.Unlock()

	// One local insert → exactly one frame on the wire. If the echo-loop
	// guard is broken, B will republish A's frame, A's subscriber will
	// re-publish B's republish, and the count will explode.
	if got != 1 {
		t.Fatalf("expected exactly 1 frame on the wire, got %d (echo loop?)", got)
	}
}

// TestTwoNodes_ConvergeUnderConcurrentInserts is the Fugue headline guarantee
// observed end-to-end: when both nodes insert at position 0 concurrently,
// both eventually see the same string and the result is one full run of A's
// chars followed by B's, or vice versa — never interleaved.
func TestTwoNodes_ConvergeUnderConcurrentInserts(t *testing.T) {
	const doc = "doc-concurrent"
	client := newRedis(t)

	a := newNodeStack(t, "node-a", doc, client)
	b := newNodeStack(t, "node-b", doc, client)

	time.Sleep(20 * time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			a.localInsert(t, doc, i, 'A')
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			b.localInsert(t, doc, i, 'B')
		}
	}()
	wg.Wait()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sa, sb := a.snapshot(t, doc), b.snapshot(t, doc)
		if sa == sb && len(sa) == 6 {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	sa := a.snapshot(t, doc)
	sb := b.snapshot(t, doc)
	if sa != sb {
		t.Fatalf("nodes did not converge: A=%q B=%q", sa, sb)
	}
	if len(sa) != 6 {
		t.Fatalf("expected 6 chars, got %q", sa)
	}

	// Non-interleaving: every A must be contiguous, every B must be
	// contiguous. Either AAABBB or BBBAAA, never any other pattern.
	if sa != "AAABBB" && sa != "BBBAAA" {
		t.Fatalf("interleaved result %q — Fugue invariant violated", sa)
	}

	// Sanity: the operation log shape — every node must have the same
	// underlying tree state, including tombstones (here, none).
	_ = crdt.NewFugueTree // pin the import even if all use is via repo
}
