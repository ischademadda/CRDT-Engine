# ADR-0004: CRDT primitive types selection
*Status*: Accepted

## Context
The engine needs a hierarchy of CRDT types to support different use cases, from simple counters and flags to complex collaborative text editing. The initial MVP must include a variety of primitives to demonstrate the generality of the `CRDTNode[State]` interface and serve as building blocks for more complex structures.

Each primitive must satisfy the three CRDT properties:
1. **Commutativity**: merge(A,B) == merge(B,A)
2. **Associativity**: merge(merge(A,B),C) == merge(A,merge(B,C))
3. **Idempotency**: merge(A,A) == A

## Decision
Implement the following CRDT types in Phase 1:

### 1. VectorClock (`vclock.go`)
- **Purpose**: Foundation for causal ordering and Epoch-based GC.
- **Algorithm**: Pointwise maximum (join semilattice).
- **Key feature**: `Compare()` method detects causal relationships (before, after, equal, concurrent).
- **Usage**: Every `OpID` will eventually carry a VectorClock snapshot for conflict detection.

### 2. GSet — Grow-only Set (`gset.go`)
- **Purpose**: Simplest CRDT; used as building block for 2P-Set.
- **Algorithm**: Union of sets.
- **Constraint**: Elements can only be added, never removed.

### 3. LWW-Register — Last-Writer-Wins (`lww_register.go`)
- **Purpose**: Single-value register for metadata (user names, titles, settings).
- **Algorithm**: Timestamp comparison with deterministic ReplicaID tiebreaker.
- **Tiebreaker**: When timestamps are equal, the higher ReplicaID (lexicographic) wins. This ensures determinism without coordination.

### 4. 2P-Set — Two-Phase Set (`twopset.go`)
- **Purpose**: Set with remove support; demonstrates tombstone semantics.
- **Algorithm**: Two GSets (add-set + remove-set). Element is "alive" iff in add-set AND NOT in remove-set.
- **Limitation**: Once removed, an element cannot be re-added (remove-wins). This motivates the need for OR-Set in future work.

### Why not OR-Set immediately?
OR-Set (Observed-Remove Set) allows re-adding removed elements using unique tags per add operation. It is more complex and depends on a robust `OpID` + `VectorClock` infrastructure. We implement 2P-Set first as a stepping stone and will add OR-Set in a later phase.

## Consequences

**Pros:**
- Four CRDT types demonstrate the generality of the `CRDTNode` interface.
- VectorClock provides the foundation for all future causal ordering needs.
- LWW-Register with tiebreaker is production-ready for metadata fields.
- 2P-Set teaches tombstone semantics, directly relevant to the Fugue algorithm's delete handling.

**Cons:**
- 2P-Set's "no re-add" limitation may confuse users who expect Set behavior. Must be documented clearly.
- LWW-Register relies on synchronized clocks; in production, hybrid logical clocks (HLC) would be more robust.
