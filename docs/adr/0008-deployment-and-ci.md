# ADR-0008: Reproducible deployment — Docker, Compose, and CI
*Status*: Accepted

## Context
By the end of Phase 3 the engine was a working multi-node system, but trying it required a Go install, a local Redis, two terminals, and the right environment variables in each. Two consequences followed:

1. **Nobody outside the original developer could run it in under five minutes.** That kills the "show a friend / a reviewer / a recruiter" loop, which is much of the project's point.
2. **Regressions had no automatic gate.** Tests existed, but ran only when manually invoked. A bad merge could land on `main` and stay there until the next manual `go test` run.

Phase 4 had two infra items on the roadmap (Docker + compose, GitHub Actions). Both were accepted as the most leveraged things to ship before chasing performance optimisations like RLE or Epoch GC, on the grounds that "no one can run it" and "no one notices when tests break" are more painful than "memory layout is suboptimal."

## Decision

### Docker image — multi-stage, distroless, static
- Build stage on `golang:1.26-alpine`. Module download is its own step so the layer cache survives source-only changes.
- `CGO_ENABLED=0`, `-trimpath`, `-ldflags "-s -w"` → a static, debug-stripped binary.
- Runtime stage on `gcr.io/distroless/static-debian12:nonroot`. No shell, no package manager, no root user. Image surface ≈ binary + CA bundle.
- `EXPOSE 8080` and sensible defaults for `HTTP_ADDR` / `REDIS_ADDR` / `DOC_ID` baked in via `ENV`, so a bare `docker run` works.

### docker-compose — two-node cluster out of the box
- Three services: `redis` (with `redis-cli ping` healthcheck) and two `demo-app` instances (`node-a` on host port 8080, `node-b` on 8081), both pointing at `redis:6379` via the compose network.
- `depends_on … condition: service_healthy` so the nodes don't race the broker on cold start.
- Distinct `NODE_ID` per service — important because the use-case relies on `OriginNodeID` to filter Redis echo.
- One command (`docker compose up --build`) reproduces the multi-node Pub/Sub story end-to-end. This is what `TESTING.md` recommends to anyone who wants to see horizontal scale work.

### GitHub Actions CI
Three jobs, each runs on every push to `main` and every PR:

- **`test`** — `setup-go` with cache, `go mod tidy` diff check (catches forgotten tidy commits), `go build`, `go vet`, `go test ./... -count=1 -coverprofile=coverage.out`, then a coverage summary line.
- **`lint`** — `golangci-lint` with the project config (errcheck, govet, staticcheck, unused, gofmt, goimports, misspell, unconvert; `_test.go` exempted from errcheck).
- **`docker`** — `buildx` + `build-push-action` against the same Dockerfile, with `cache-from/to: type=gha` so the second run is fast. Doesn't push anywhere — only proves the image still builds.

`-race` is intentionally not in CI yet: the project runs CGO_ENABLED=0 by default to keep the binary static, and the existing race coverage on the developer's Windows box is zero. Adding race-detection in a CI-only job is a follow-up worth doing once the test surface grows further.

### `.dockerignore`, `.golangci.yml`
Trimmed `.dockerignore` (excludes `.git`, `.github`, docs, claude memory, build artefacts, local env files) keeps the build context small. `.golangci.yml` pins the linter set so future contributors don't get a different result locally vs. in CI.

## Consequences

**Pros:**
- Anyone with Docker can run a full two-node cluster in one command. The README's "Getting Started" went from "install Go, install Redis, set env vars, run two terminals" to "`docker compose up --build`."
- Distroless + static binary keeps the production image at ~10 MB (no glibc, no shell, no CVEs from a base distro), and forces operators to deal with config via env vars rather than mutating the container.
- Every PR runs the full test suite, lints, and validates the Dockerfile. Regressions surface in minutes, not at next demo.
- `go mod tidy` is now policed by CI. The first version of Phase 4 was missing a tidy and would have failed its own check — caught locally before merge, which is exactly the intended workflow.

**Cons:**
- Docker Desktop is a dependency for the recommended path. Native (non-Docker) instructions stay in `TESTING.md` for users who can't or won't install it.
- CI is GitHub-specific. Migrating off GitHub means rewriting the workflow file. Acceptable lock-in.
- `golang:1.26-alpine` may lag behind `go.mod`'s exact patch (1.26.x). Pinned to the latest 1.26 minor available; a lockstep update is a one-line bump.

**Rejected alternatives:**
- *Single-stage Dockerfile* — smaller Dockerfile, ~10× larger image, ships the full Go toolchain. Not worth it.
- *`alpine` runtime instead of `distroless`* — gives you a shell for debugging in exchange for ~7 MB and an apk surface. The distroless `static-debian12:debug` variant exists for the rare debugging case; not the default.
- *Self-hosted CI* — strictly more work for no win at this scale.
- *Adding `-race` immediately* — race-detector requires CGO_ENABLED=1 in the test job, which is fine but cosmetically diverges from the production build. Deferred until the integration surface justifies the divergence.

## Status of related work
- `Dockerfile`, `docker-compose.yml`, `.dockerignore` — implemented and merged to `main`.
- `.github/workflows/ci.yml`, `.golangci.yml` — implemented; first run is the next push to `main`.
- `TESTING.md` — full walkthrough of the five recommended scenarios (single-machine, LAN, multi-node Redis, ngrok, compose).
