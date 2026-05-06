// Package crdt provides Conflict-free Replicated Data Types (CRDTs) for
// building eventually consistent, distributed systems in Go.
//
// The package's flagship type is [FugueTree], a sequence CRDT for collaborative
// text editing that mathematically guarantees "maximal non-interleaving":
// when two replicas concurrently insert text at the same position, their text
// is never interleaved character-by-character on convergence. Other types
// ([GSet], [TwoPSet], [LWWRegister], [VectorClock]) cover smaller building
// blocks for non-text data.
//
// # Core contract
//
// Every CRDT type in this package implements [CRDTNode], parameterised over
// its materialised state (string for FugueTree, []T for GSet, etc.). The
// contract guarantees three algebraic properties on Merge:
//
//   - Commutativity:  Merge(A, B) ≡ Merge(B, A)
//   - Associativity:  Merge(Merge(A, B), C) ≡ Merge(A, Merge(B, C))
//   - Idempotence:    Merge(A, A) ≡ A
//
// These properties together imply Strong Eventual Consistency: any two
// replicas that have received the same set of operations — in any order, with
// any duplicates — will converge to the same state.
//
// # FugueTree: collaborative text
//
// FugueTree models a document as a tree of single-rune nodes, each carrying a
// globally-unique [OpID]. Document order is the in-order traversal of the
// tree. Insertions choose a tree position relative to their neighbours
// (Fugue Parent Selection); deletions are tombstones, kept to preserve the
// CRDT contract until garbage collection (Phase 4+).
//
// Two ways to drive a tree:
//
//  1. Local API: [FugueTree.InsertAt], [FugueTree.DeleteAt] — return an
//     [Operation] that can be broadcast to other replicas.
//  2. Remote API: [FugueTree.ApplyRemoteInsert], [FugueTree.ApplyRemoteDelete]
//     — apply an operation that arrived from another replica.
//
// [FugueTree.Merge] is a state-based shortcut: given another tree, it pulls
// every missing node and tombstone in one shot. Useful for snapshot transfer
// after a partition.
//
// # Example
//
// See [Example] for a runnable walkthrough of insert + delete + concurrent
// merge. Briefly:
//
//	a := crdt.NewFugueTree("replica-A")
//	b := crdt.NewFugueTree("replica-B")
//
//	opA, _ := a.InsertAt(0, 'H')       // local edit on A
//	b.ApplyRemoteInsert(opA)           // ship to B
//
//	// Concurrent edits at the same position never interleave:
//	a.InsertAt(1, 'i')
//	b.InsertAt(1, '!')
//	_ = a.Merge(b)
//	_ = b.Merge(a)
//	// a.State() == b.State() — both replicas converge.
//
// # Choosing a CRDT
//
//	Type           When to use
//	──────────     ──────────────────────────────────────────────────
//	FugueTree      collaborative text / ordered sequences
//	GSet           grow-only sets (e.g., observed user IDs)
//	TwoPSet        sets with delete (add-once, remove-once)
//	LWWRegister    single value with last-writer-wins resolution
//	VectorClock    causal-order tracking — typically embedded in ops
//
// # Concurrency
//
// All public types are safe for concurrent use. Each carries an internal
// [sync.RWMutex] guarding mutation. Apply operations and merges from any
// goroutine; the engine handles serialisation.
//
// # Where to go from here
//
//   - For wiring this engine into a real-time collaborative server, see the
//     transport layer (internal/websocket, internal/redis) and the
//     orchestration layer (internal/usecase) in the same module.
//   - For the algorithmic background, see ADR-0005 in docs/adr.
package crdt
