// Package integration holds end-to-end tests that wire multiple internal
// packages together — repository, use-case, websocket, redis, worker — the
// way cmd/demo-app does at runtime.
//
// These tests don't exercise the HTTP layer (no real WebSocket), but they do
// exercise everything below it through real goroutines, real channels, and a
// real (in-memory) Redis Pub/Sub via miniredis. The point is to catch
// regressions in *wiring* — the kind of bugs that pass every per-package unit
// test but break when the layers meet.
package integration
