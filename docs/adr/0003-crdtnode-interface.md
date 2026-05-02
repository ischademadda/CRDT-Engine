# ADR-0003: CRDTNode generic interface as the base abstraction
*Status*: Accepted

## Context
The engine must support multiple CRDT data types (GSet, LWW-Register, 2P-Set, Fugue sequences) behind a uniform contract. Without a shared interface, each CRDT type would have ad-hoc merge/apply methods, making it impossible to build generic transport, persistence, and synchronization layers.

Go 1.18+ introduced type parameters (Generics), which allow us to define a single interface parameterized by the materialized state type. This eliminates the need for `interface{}` / `any` casts and provides compile-time type safety.

## Decision
Define two core interfaces in `pkg/crdt/engine.go`:

1. **`Operation`** — represents a single CmRDT operation with a type identifier (`OpType()`) and a globally unique ID (`ID() OpID`). The `OpID` struct uses `{ReplicaID, Counter}` pairs to guarantee uniqueness without coordination.

2. **`CRDTNode[State any]`** — the universal contract for any CRDT type:
   - `Merge(other CRDTNode[State]) error` — state-based merge (CvRDT)
   - `ApplyOperation(op Operation) error` — operation-based apply (CmRDT)
   - `State() State` — materialize current state

Each concrete CRDT (e.g., `GSet[T]`) implements `CRDTNode` with a specific `State` type (e.g., `[]T`).

### Why Generics over `interface{}`
- **Type safety at compile time**: `GSet[string]` cannot accidentally merge with `GSet[int]`.
- **Better Developer Experience**: consumers of the API get autocompletion and type inference.
- **Follows Go idiom**: "Accept interfaces, return structs."

### Why a single interface for both CvRDT and CmRDT
The engine supports a hybrid approach:
- **`Merge`** is used for state-based synchronization (initial load, reconnect after long offline).
- **`ApplyOperation`** is used for operation-based incremental sync (real-time WebSocket deltas).

This allows the transport layer to choose the optimal strategy per situation.

## Consequences

**Pros:**
- All CRDT types share one contract — transport, persistence, and testing code is reusable.
- Generic type parameter prevents cross-type merge bugs at compile time.
- `OpID{ReplicaID, Counter}` provides a foundation for Vector Clocks and causal ordering.

**Cons:**
- Go's Generics have limitations: you cannot use `CRDTNode[any]` as a universal container (no covariance). Each concrete state type needs its own handling.
- Adding new methods to the interface in the future is a breaking change for all implementors.
