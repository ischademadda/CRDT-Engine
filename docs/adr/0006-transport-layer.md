# ADR-0006: Transport layer — WebSocket + Redis Pub/Sub
*Status*: Accepted

## Context
The engine needs two transport responsibilities that are easy to conflate but should be separated:

1. **Edge transport** — push CRDT operations to/from end-user browsers in real time, with low latency, ordered delivery, and a long-lived connection.
2. **Inter-node fan-out** — when a document is edited on node A, every other node holding a replica of that document must learn about the operation.

A single websocket-only design would force every editor session to either (a) pin to one server (no horizontal scale, no resilience to node failure) or (b) maintain N×N node-to-node connections (operationally hostile, doesn't grow).

A single pub/sub-only design has no edge-to-server channel suitable for browsers.

The two layers solve different problems and need different tools.

## Decision

### Edge transport: WebSocket (`internal/websocket/`)
- `Hub` keeps a per-document client registry (`map[documentID]map[*Client]struct{}`).
- All inbound messages from all clients converge on a single channel (`Inbound() <-chan Message`) — **Fan-In**. The use-case layer reads from this channel, applies the operation to the CRDT, and asks the Hub to rebroadcast (**Fan-Out**) excluding the original sender.
- Each `Client` has a bounded `send` buffer; if it fills up, the client is treated as slow and disconnected. This protects the Hub from head-of-line blocking on a single stuck reader.
- `gorilla/websocket` for the framing layer — battle-tested, has the right ping/pong knobs.

### Inter-node fan-out: Redis Pub/Sub (`internal/redis/`)
- After applying an op locally, the use-case publishes a `Delta` envelope to channel `crdt:doc:<id>`.
- Every node subscribes to the documents it currently hosts. Receiving a delta on a node feeds it into the same merge pipeline as a local op.
- `OriginNodeID` is part of the envelope so a node can drop its own echo (Redis Pub/Sub fans out to publishers too).
- JSON for the MVP. Protobuf is on the Phase 4 list — the `Delta.Payload` field is already `json.RawMessage`, which is opaque to the adapter, so swapping the codec is a localized change.

### Worker pool (`internal/worker/`)
A separate concern, but lives in this layer because it's how the use-case keeps up with the Fan-In stream without doing CRDT work on the Hub goroutine. Fixed-size pool, bounded queue, graceful `Stop(ctx)` that cancels job context on timeout.

## Consequences

**Pros:**
- Clean separation: WebSocket talks to humans, Redis talks to peers. Each can be replaced without touching the other (e.g., swap Redis Pub/Sub for NATS later).
- Horizontal scale: any node can serve any client because state propagates through Redis.
- The Hub never blocks on a single slow client.
- Worker pool gives a clean knob for backpressure on the CRDT side without coupling it to the network.

**Cons:**
- Redis Pub/Sub is **fire-and-forget**: a brief network blip silently drops messages. For an at-most-once channel between strongly-eventually-consistent CRDT replicas this is acceptable — convergence is guaranteed once any later message arrives — but it does mean a node coming back from a partition must run a state-transfer/merge on reconnect rather than relying on the pub/sub stream alone.
- Two transports means two failure modes to monitor.
- JSON over Redis is fine at MVP scale; a high-throughput document with many ops per second will want Protobuf.

**Rejected alternatives:**
- *Redis Streams* — gives at-least-once delivery but adds consumer-group bookkeeping the engine doesn't need; CRDTs make duplicate delivery harmless and missed messages recoverable via state transfer, so the simpler primitive wins.
- *Direct node-to-node gRPC mesh* — ops burden grows quadratically with node count; deferred unless/until pub/sub becomes a bottleneck.
- *Server-Sent Events instead of WebSocket* — one-directional, would force HTTP POST for client→server, which fragments the per-client ordering guarantee that a single duplex socket gives for free.

## Status of related work
- `internal/websocket/` — implemented, 6 tests on `httptest`.
- `internal/redis/` — implemented, 6 tests on `miniredis`.
- `internal/worker/` — implemented, 6 tests.
- Wiring into a real `cmd/demo-app` is Phase 3 (Clean Architecture / use-case layer).
