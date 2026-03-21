# CRDT-Engine

A lightweight, high-performance Conflict-free Replicated Data Type (CRDT) engine written in pure Go. 

This project provides a universal data synchronization core designed for real-time collaborative applications. It specifically addresses common CRDT bottlenecks such as text interleaving anomalies, excessive memory consumption, and single-thread blocking, making it suitable for high-load, cloud-native backend environments.

## Key Features

* **Fugue Algorithm**: Implements the Fugue algorithm for sequence CRDTs to mathematically guarantee "maximal non-interleaving" during concurrent text edits.
* **Epoch-based Garbage Collection**: Uses Vector Clocks to track the global state across the cluster. Once an deletion operation is acknowledged by all clients, background goroutines physically remove the tombstones from memory, preventing Server Out-Of-Memory (OOM) issues.
* **Memory Optimization**: Employs Run-Length Encoding (RLE) and B-trees to group sequential user inputs into contiguous memory blocks (Go slices), drastically reducing garbage collector overhead compared to single-character node structs.
* **Horizontal Scalability**: Designed for high concurrency. Uses Fan-In/Worker Pool patterns for WebSocket connections and Redis Pub/Sub for syncing compact binary deltas across multiple server instances.
* **Type-Safe API**: Leverages Go Generics to provide a strict, interface-driven API (`Accept interfaces, return structs`), allowing easy integration with custom business logic structures.

## Requirements

* Go 1.26.1 or higher

## Project Structure

The repository follows the Standard Go Project Layout:

* `/pkg/crdt`: The core engine library and CRDT mathematical primitives.
* `/cmd`: Main applications, including the runnable demo server.
* `/internal`: Private application logic and adapters.
* `/docs`: Architectural documentation. Includes Architecture Decision Records (`/adr`) and C4 model diagrams.

## Current status: Documentation and Architecture Development

* C4 Context & Container
* Main ADRs 

## Getting Started

*(Add standard installation instructions here once the module is published)*
```bash
go get github.com/ischademadda/CRDT-Engine