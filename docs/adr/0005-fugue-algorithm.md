# ADR-0005: Fugue algorithm over YATA for sequence CRDTs
*Status*: Accepted

## Context
The engine needs a sequence CRDT for collaborative text editing. Two primary algorithms were evaluated:

1. **YATA** (Yet Another Text Allocation) — used by Yjs. Each element stores IDs of its left and right neighbors at insertion time.
2. **Fugue** — newer algorithm. Elements form a tree; document order is determined by in-order traversal.

The critical difference is **text interleaving**: when two users concurrently type at the same position, YATA can produce mixed output ("HWeolrllod" instead of "HelloWorld"). Fugue mathematically guarantees "maximal non-interleaving."

## Decision
Implement the **Fugue algorithm** with tree-based parent selection.

### Fugue Parent Selection Rule
When inserting between left neighbor L and right neighbor R:
- If R ≠ nil AND parent(R) = L → new node becomes **left child of R**
- Otherwise → new node becomes **right child of L**

### Document ordering
In-order tree traversal: left children (sorted by OpID) → node → right children (sorted by OpID).

### Why this guarantees non-interleaving
When two users concurrently insert text starting at the same position, each user's characters form a **chain of right children**. These chains are separate subtrees of a common ancestor. During traversal, each subtree is visited completely before moving to the next, so characters from different users never alternate.

## Consequences

**Pros:**
- Mathematically proven non-interleaving (Fugue paper, 2023).
- Simpler conflict resolution than YATA (no complex neighbor ID tracking).
- Tree structure naturally supports future optimizations (RLE blocks, B-tree).
- Deterministic sibling ordering via OpID comparison — no coordination needed.

**Cons:**
- Tree traversal for position lookup is O(n) in current implementation. Future optimization: index structure for O(log n).
- Insert-at-beginning by same user creates non-intuitive tree shapes (third prepend goes to different subtree). Does not affect correctness.
- Merge requires topological sorting of operations by Counter to ensure parent-before-child ordering.
