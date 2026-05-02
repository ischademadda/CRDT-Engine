# ADR-0002: Repository structure
*Status*: Accepted

## Context
For maximum engine compatibility and an easy-to-integrate approach, the repository structure must follow the Standard Go Project Layout conventions widely adopted by the community.

## Decision
Implement the Standard Go Project Layout with the following directories:
- `/cmd` — Main applications for this project (demo-app, CLI tools).
- `/internal` — Private application and library code (WebSocket, Redis adapters).
- `/pkg` — Engine core code (CRDT primitives, public API).
- `/docs` — Architectural documentation (ADR, C4 diagrams, flow diagrams).

## Consequences

**Pros:**
- Familiar structure for any Go developer — reduces onboarding time.
- Clear separation between public API (`/pkg`) and private internals (`/internal`).
- Go compiler enforces `/internal` package visibility rules.

**Cons:**
- Requires discipline to keep boundaries clean as the project grows.



