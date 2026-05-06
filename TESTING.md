# Testing CRDT-Engine

How to run the engine and try concurrent editing across multiple browser tabs, multiple machines on the same LAN, or over the internet.

All scenarios use the demo app at `cmd/demo-app`. Open `/` in a browser, click **Connect**, and start typing — concurrent edits at the same caret position will not interleave thanks to Fugue.

---

## TL;DR

| Want to… | Use |
|---|---|
| Quickest possible try-out (one machine, two tabs) | [Scenario 1](#scenario-1-single-machine-no-redis) |
| Two laptops on the same Wi-Fi | [Scenario 2](#scenario-2-multiple-machines-on-the-same-lan) |
| Two CRDT server instances syncing through Redis | [Scenario 3](#scenario-3-multi-node-cluster-with-redis) |
| Show a demo to someone over the internet | [Scenario 4](#scenario-4-public-demo-via-ngrok) |
| Reproducible local cluster in one command | [Scenario 5](#scenario-5-docker-compose-two-node-cluster) |

---

## Prerequisites

- **Go 1.26+** — `go version`
- **Docker** (only for Scenarios 3 and 5) — `docker --version`
- The repo cloned: `git clone https://github.com/ischademadda/CRDT-Engine && cd CRDT-Engine`

The demo app reads four environment variables, all optional:

| Var | Default | Meaning |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Address the HTTP server binds to |
| `REDIS_ADDR` | `localhost:6379` | Redis for inter-node Pub/Sub. Empty string or unreachable → server runs stand-alone (single-node) |
| `NODE_ID` | random | Identifier used for Redis echo-filtering |
| `DOC_ID` | `demo` | Document this node subscribes to in Redis |

---

## Scenario 1: Single machine, no Redis

Simplest possible test — one server, multiple tabs. Useful for verifying Fugue convergence on the same machine.

```bash
# from repo root
REDIS_ADDR= go run ./cmd/demo-app
```

(On Windows PowerShell: `$env:REDIS_ADDR=""; go run ./cmd/demo-app`)

Open **two tabs** of `http://localhost:8080`, click **Connect** in each, and type. Concurrent inserts at the same position will not interleave.

> Empty `REDIS_ADDR` skips Redis entirely. The server logs `running stand-alone` if it can't reach Redis on startup, so even keeping the default works — there's just a noisy ping warning.

---

## Scenario 2: Multiple machines on the same LAN

One machine runs the server; other devices on the same Wi-Fi/Ethernet connect to it.

### On the server machine

1. **Find your LAN IP.**
   - Windows: `ipconfig` → look for the `IPv4 Address` under your active adapter (typically `192.168.x.x` or `10.x.x.x`).
   - macOS: `ipconfig getifaddr en0` (Wi-Fi) or `en1`.
   - Linux: `hostname -I` or `ip addr show`.

2. **Start the server bound to all interfaces** (not just loopback):

   ```bash
   HTTP_ADDR=0.0.0.0:8080 REDIS_ADDR= go run ./cmd/demo-app
   ```

   `0.0.0.0` is essential — `:8080` alone may bind only to the loopback interface on some Windows setups, and other machines won't see it.

3. **Allow the firewall prompt.** On the first run Windows asks whether to allow Go through the firewall — pick **Private networks**. On macOS, accept the incoming-connections prompt. On Linux you may need `sudo ufw allow 8080/tcp`.

### On other machines on the same network

Open `http://<server-lan-ip>:8080` in a browser, click **Connect**, type. Multiple devices can connect at the same time.

### Troubleshooting

- **Connection refused / page never loads** — almost always a firewall. Temporarily disable the OS firewall to confirm, then add a rule for port 8080.
- **Different subnets** — guest Wi-Fi networks sometimes isolate clients from each other. Connect both machines to the same SSID and the same band (some routers have separate 2.4 GHz / 5 GHz subnets).
- **`Connect` button shows `error`** — open the browser console; usually a CORS or mixed-content issue. Use `http://`, not `https://`, since the demo doesn't ship a TLS cert.

---

## Scenario 3: Multi-node cluster with Redis

Two CRDT-Engine servers, both syncing through one Redis. This is what production deployment looks like — any client can connect to any node and see everyone's edits.

### Without Docker

1. Start Redis somewhere reachable. Easiest:

   ```bash
   docker run --rm -p 6379:6379 redis:7-alpine
   ```

2. Start node A:

   ```bash
   HTTP_ADDR=:8080 REDIS_ADDR=localhost:6379 NODE_ID=node-a go run ./cmd/demo-app
   ```

3. In another terminal, start node B on a different port:

   ```bash
   HTTP_ADDR=:8081 REDIS_ADDR=localhost:6379 NODE_ID=node-b go run ./cmd/demo-app
   ```

4. Open `http://localhost:8080` in one tab and `http://localhost:8081` in another. Edits made in one tab propagate through Redis to the other.

If Redis runs on a different machine, point `REDIS_ADDR` at its LAN IP (e.g., `REDIS_ADDR=192.168.1.42:6379`).

### With Docker

See [Scenario 5](#scenario-5-docker-compose-two-node-cluster) — `docker compose up` does all the wiring.

---

## Scenario 4: Public demo via ngrok

Showing the demo to someone outside your network without deploying anywhere.

1. Install [ngrok](https://ngrok.com/download) and authenticate (free tier is enough).
2. Start the server normally:

   ```bash
   REDIS_ADDR= go run ./cmd/demo-app
   ```
3. In a second terminal:

   ```bash
   ngrok http 8080
   ```
4. ngrok prints a public `https://<random>.ngrok-free.app` URL. Share it. Anyone with the URL can connect.

> ⚠️ This exposes your machine to the internet for as long as ngrok runs. The demo has no auth — close the tunnel when you're done.

---

## Scenario 5: Docker Compose two-node cluster

Reproducible local cluster — one Redis + two CRDT-Engine instances. Zero Go install required for the user, only Docker.

```bash
docker compose up --build
```

This brings up:

| Service | Port | Role |
|---|---|---|
| `redis` | `6379` | Pub/Sub broker |
| `node-a` | `http://localhost:8080` | CRDT server #1 |
| `node-b` | `http://localhost:8081` | CRDT server #2 |

Open `http://localhost:8080` and `http://localhost:8081` in two browser tabs. Edits cross between the nodes through Redis.

```bash
docker compose down            # stop and remove containers
docker compose down -v         # also remove the redis volume
docker compose logs -f node-a  # follow one node's logs
```

To expose the cluster on your LAN, the published ports already bind to `0.0.0.0` — other machines reach `http://<host-lan-ip>:8080` / `8081` directly.

---

## Verifying it actually works

The interesting property is **non-interleaving** under concurrent inserts. To see it:

1. Open two tabs connected to the same `DOC_ID`.
2. In tab A, type `AAAAA` quickly at position 0.
3. At the same time, in tab B, type `BBBBB` quickly at position 0.

After both stop typing, both tabs converge to the same string — and that string is either `AAAAABBBBB` or `BBBBBAAAAA`, **never** `ABABABABAB`. That's Fugue doing its job.

You can also inspect the server's view directly:

```bash
curl http://localhost:8080/snapshot?doc=demo
```

returns `{"doc":"demo","text":"..."}` with the current materialised text.

---

## Running the test suite

```bash
go test ./... -count=1            # all unit tests, no race detector (CGO required for -race)
go test ./pkg/crdt/... -v         # CRDT-only, verbose
go test ./... -coverprofile=c.out && go tool cover -html=c.out
```

CI runs `go vet`, `golangci-lint`, the full test suite, and a `docker build` on every PR.
