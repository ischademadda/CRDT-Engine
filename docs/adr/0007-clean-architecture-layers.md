# ADR-0007: Clean Architecture layering — repository / use-case / delivery
*Status*: Accepted

## Context
After the transport layer (ADR-0006) we had three pieces — the WebSocket Hub, the worker pool, and the Redis Pub/Sub adapter — and a question: **where does the CRDT actually get applied, and who owns the decision to broadcast or republish?**

A naive implementation would have the Hub call into the Fugue tree and then into Redis directly. That works for one read of the codebase and rots on the second:

- The Hub is a network primitive; teaching it about CRDTs welds two unrelated concerns together.
- Tests must spin up a real WebSocket every time they want to exercise the merge logic.
- Swapping Redis Pub/Sub for NATS, or the in-memory store for PostgreSQL, becomes a refactor that touches every layer.
- The "should I republish this delta?" decision (yes if the delta is local, no if it came from another node — otherwise we get an echo loop) ends up scattered across whoever happens to publish.

The project explicitly aimed at Clean Architecture, so the structure was planned, not improvised — but the decision deserves to be recorded because the alternative (a single fat handler) is easier to write and looks shorter on a slide.

## Decision

Three layers between transport and storage:

### Repository layer (`internal/repository/`)
- One interface, `DocumentRepository` (`Get`, `Create`, `GetOrCreate`, `Save`, `Exists`, `Delete`).
- One implementation today: `InMemoryRepository`, a `map[string]*FugueTree` behind an `RWMutex`. Save is a no-op because the tree is held by pointer — the same in-memory model that PostgreSQL/Redis-snapshot implementations will satisfy later.
- The repository owns no business logic. It does not know what a CRDT is; it stores opaque trees by ID.

### Use-case layer (`internal/usecase/`)
- `SyncUseCase.HandleDelta(ctx, Delta)` is the **single entry point** for every CRDT operation reaching this node, regardless of source.
- The `Delta` envelope carries an `Origin` field (`OriginLocal` from a WS client; `OriginRemote` from Redis). The use-case enforces the rule: **always broadcast to local WS clients; only publish to Redis when the origin is local.** That's how the echo loop is avoided in exactly one place.
- The use-case depends on two **ports** declared inline (`Broadcaster`, `Publisher`), not on the concrete `internal/websocket` and `internal/redis` packages. This is the practical payoff: `sync_test.go` runs against fake implementations of both ports — no sockets, no Redis, no goroutines under test.
- `DocumentUseCase` is a thin façade for document lifecycle (`LoadOrCreate`, `Text`).

### Delivery layer (`cmd/demo-app/`)
- The wiring lives only here. `main` constructs every concrete (Hub, WorkerPool, InMemoryRepo, SyncUseCase, Publisher, Subscriber) and wires them together via two small adapters (`hubAdapter`, `pubAdapter`) that satisfy the use-case ports.
- The dispatcher converts client *intents* (`{"pos": N, "char": "x"}`) into Fugue operations on the local tree, then routes them through the use-case. Inbound Redis messages skip the intent-decoding step but go through the same `HandleDelta`.
- All graceful-shutdown choreography (HTTP → Hub → WorkerPool → Redis) lives here, not inside any reusable package.

### Dependency direction
```
delivery (cmd/demo-app)  ──►  use-case (internal/usecase)  ──►  repository + ports
       │                              │
       └──── adapters (websocket / redis impls of ports)
```
Use-case depends on nothing but the repository interface and its own ports. Reversing this — `pkg/crdt` importing `internal/usecase`, or `internal/usecase` importing `internal/websocket` — is the failure mode this layout exists to prevent.

## Consequences

**Pros:**
- The CRDT-merge code path is testable without a network. `sync_test.go` runs in milliseconds against fake ports and covers the echo-loop rule, unknown op types, and end-to-end insert/delete.
- The Postgres repository (Phase 4+) is a drop-in: `cmd/demo-app/main.go` is the only file that learns about it.
- The "where is the broadcast policy?" question has one answer: `SyncUseCase.HandleDelta`. Future contributors don't have to reverse-engineer it.
- The `Delta` envelope decouples the use-case from both `websocket.Message` and `redis.Delta`; either transport struct can change shape without breaking the orchestration.

**Cons:**
- Three small adapters (~20 LOC each) that exist purely for layering. For a single-transport project this would be ceremony; here it's the minimum tax for two transports + persistence to come.
- Slightly more files to navigate when reading the system top-down. Mitigated by the "single entry point" rule — `HandleDelta` is the one symbol you must understand.

**Rejected alternatives:**
- *Fat handler in `cmd/demo-app`* — works at MVP scale, but the broadcast/republish policy ends up duplicated between WS and Redis paths, and the Fugue tree gets reached from inside HTTP handler closures, which makes the test setup costlier than the test itself.
- *Use-case importing `internal/websocket` directly* — saves the adapter, costs the unit-testability. Considered a bad trade because we already accept the same overhead for `internal/redis` (Publisher port).
- *Repository as part of the use-case package* — keeps the intra-package coupling loose, but blocks the future Postgres impl from being swapped without recompiling the use-case. The interface boundary costs almost nothing and pays for itself the first time persistence changes.

## Status of related work
- `internal/repository/` — implemented (interface + in-memory), 6 tests including concurrent `GetOrCreate`.
- `internal/usecase/` — implemented (SyncUseCase + DocumentUseCase), 6 port-driven tests.
- `cmd/demo-app/` — implemented, wires every layer, optional Redis with stand-alone fallback.
- PostgreSQL repository — Phase 4+.
