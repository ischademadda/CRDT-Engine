# ADR-0001: Programming language choice
*Status*: Accepted

## Context
Engine have to be based on language with  high-perfomance in multiple WebSocket-connections with minimal memory usage. The second goal is to reach maximum compatibility on different platforms and developer's stack.

## Decision
The Golang backend provides light-weight engine that can easily handle thousands of WebSocket-connections and process all CRDT data.

## Consequences
The pros and cons of decision:

**Pros:**
 - High performance
 - Light-weighted backend
 - Fast horizontal connections and data grow 

 **Cons:**
 - Implementing CRDT logic from scratch

