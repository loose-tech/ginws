# GinWS Documentation

Welcome to the comprehensive documentation for **GinWS** — an enterprise-grade, high-performance WebSocket toolkit for the [Gin](https://github.com/gin-gonic/gin) Go framework, accompanied by a type-safe TypeScript client.

GinWS solves the common architectural friction points in real-time applications: asynchronous request-reply correlations, decoupled sequential business logic via server-side action chaining, robust reconnect & offline message synchronization, and concurrent WebSocket pump safety.

---

## Documentation Map

| Guide | Description | Target Files |
| :--- | :--- | :--- |
| **[Getting Started](file:///home/lluie/ws/docs/getting-started.md)** | Submodule setup, Gin backend registration, TypeScript client installation, and connection lifecycle. | [`ws.go`](file:///home/lluie/ws/ws.go), [`client/ws.ts`](file:///home/lluie/ws/client/ws.ts) |
| **[Action Chaining System](file:///home/lluie/ws/docs/action-chaining.md)** | Deep dive into multi-step pipelines (`order:create` $\rightarrow$ `order:validate` $\rightarrow$ `order:finalize`, Tic-Tac-Toe `game:move` $\rightarrow$ `game:validate` $\rightarrow$ `game:evaluate`), `m.Data` mutation, and cycle protection. | [`message.go`](file:///home/lluie/ws/message.go), [`model.go`](file:///home/lluie/ws/model.go#L112-L126) |
| **[Request-Reply Pattern](file:///home/lluie/ws/docs/request-reply.md)** | Awaiting responses directly with `await ws.request()`, correlation reply IDs, timeouts, and error handling. | [`client/ws.ts`](file:///home/lluie/ws/client/ws.ts#L187-L249), [`model.go`](file:///home/lluie/ws/model.go#L21-L42) |
| **[Offline Catch-Up & Reconnect Sync](file:///home/lluie/ws/docs/offline-reconnect-sync.md)** | Message history architecture, client outbound buffering, reconnection reconciliation, Facebook-style read indicators, and message recall. | [`client/ws.ts`](file:///home/lluie/ws/client/ws.ts#L121-L136), [`demo/main.go`](file:///home/lluie/ws/demo/main.go#L118-L174) |
| **[Reconnection Rules, Caveats & Architecture](file:///home/lluie/ws/docs/reconnection-rules-and-caveats.md)** | Golden rules for offline recovery, anti-patterns to avoid, inbox summary unread badges, lazy-loading, and why heavy socket offline buffers fail. | [`docs/reconnection-rules-and-caveats.md`](file:///home/lluie/ws/docs/reconnection-rules-and-caveats.md), [`demo/main.go`](file:///home/lluie/ws/demo/main.go) |
| **[Pure API Reference](file:///home/lluie/ws/docs/api-reference.md)** | Full API specification for Go backend (`Hub`, `Client`, `Message`, `Room`, `Handshaker`, `Auther`) and TypeScript client (`Ws`, `Queue`, types). | [`model.go`](file:///home/lluie/ws/model.go), [`client/types.ts`](file:///home/lluie/ws/client/types.ts) |
| **[Interactive Demo Showcase](file:///home/lluie/ws/docs/demo-showcase.md)** | Guide to running and exploring the full-stack showcase in `demo/` (Go server) and `demo/ui/` (SvelteKit frontend). | [`demo/main.go`](file:///home/lluie/ws/demo/main.go), [`demo/ui/src/routes/+page.svelte`](file:///home/lluie/ws/demo/ui/src/routes/+page.svelte) |

---

## Architectural Overview

GinWS decouples network transport from business logic through a single-writer gorilla/websocket architecture, non-blocking per-client channels, and in-memory action dispatchers.

```
+─────────────────────────────────────────────────────────────────────────────+
|                             TypeScript Client                               |
|   ┌──────────────────────────────────────────────────────────────────────┐  |
|   |  Ws Instance (client/ws.ts)                                          |  |
|   |  - Outbound Queue (buffers offline messages)                         |  |
|   |  - Reply Map (Promise resolvers keyed by correlation reply ID)       |  |
|   |  - Client Action Router (middleware pipeline)                        |  |
|   └──────────────────────────────────────────────────────────────────────┘  |
+──────────────────────────────────────▲──────────────────────────────────────+
                                       │ WebSocket Frame (JSON)
                                       │ { title, reply, payload, room, ... }
+──────────────────────────────────────▼──────────────────────────────────────+
|                           Go GinWS Server (Hub)                             |
|                                                                             |
|   ┌──────────────────────────────────────────────────────────────────────┐  |
|   |  ws.WSHandler (Gin Middleware)                                       |  |
|   |  - Optional Auther / Handshaker verification                         |  |
|   |  - Gorilla WebSocket Upgrader (buffer size: 1024, origin check)      |  |
|   └──────────────────────────────────┬───────────────────────────────────┘  |
|                                      │ spawns per client                    |
|                ┌─────────────────────┴─────────────────────┐                |
|                ▼                                           ▼                |
|   ┌─────────────────────────┐                 ┌─────────────────────────┐   |
|   |  Client readPump()      |                 |  Client writePump()     |   |
|   |  - 16MB Read Limit      |                 |  - Single writer lock   |   |
|   |  - Pong heartbeat sync  |                 |  - 54s ping ticker      |   |
|   |  - Built-in join/leave  |                 |  - Non-blocking 256-msg |   |
|   |  - Async message spawn  |                 |    buffered channel     |   |
|   └────────────┬────────────┘                 └────────────▲────────────┘   |
|                │                                           │                |
|                ▼                                           │ c.Send(msg)    |
|   ┌────────────────────────────────────────────────────────┴────────────┐  |
|   |  Hub Message Processor & Action Chaining Pipeline                    |  |
|   |                                                                      |  |
|   |  action = h.GetAction(currentRoute)                                  |  |
|   |  nextRoute = action(msg, client)                                     |  |
|   |  while nextRoute != "":                                              |  |
|   |      currentRoute = nextRoute                                        |  |
|   |      (max depth: 50, timeout enforced via context.WithTimeout)       |  |
|   └──────────────────────────────────────────────────────────────────────┘  |
+─────────────────────────────────────────────────────────────────────────────+
```

---

## Core Tenets

1. **Safety First**: Gorilla WebSocket connections cannot have concurrent writers. GinWS isolates all socket writing to a dedicated `writePump` routine per client fed by a 256-slot channel, preventing race conditions or socket corruptions.
2. **Slow Client Isolation**: If a client's network hangs, GinWS drops buffer overruns with a warning log rather than stalling the Go hub or blocking sibling clients.
3. **Composable Logic**: Handlers do not need to invoke each other through brittle internal method calls. Handlers return the string name of the next action, enabling transparent state transformations and pipeline reuse.
4. **Resilient Reconnection**: Reconnecting clients do not merely pull the single latest state. The protocol coordinates full catch-up sync, message deduplication, and read receipt updates.

---

## Release History
For version history, detailed feature additions, and breaking change notices, consult the [**CHANGELOG**](file:///home/lluie/ws/CHANGELOG.md).
