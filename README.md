# GinWS

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org/)
[![Gin Framework](https://img.shields.io/badge/Gin-v1.9+-008ECF?style=flat&logo=gin)](https://gin-gonic.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**GinWS** is a high-performance, enterprise-grade WebSocket engine for the [Gin](https://github.com/gin-gonic/gin) Go framework, accompanied by a type-safe TypeScript client.

It eliminates the traditional headaches of real-time development: ad-hoc event maps, brittle callback pyramids, unhandled offline packet drops, and concurrent write collisions on raw WebSocket sockets.

---

## Key Highlights

- ⚡ **Modern Request-Reply (`await ws.request()`)**: Await typed server responses directly in TypeScript with automatic correlation IDs, customizable timeouts, and error propagation.
- 🔗 **Server-Side Action Chaining**: Handlers route to subsequent actions simply by returning their name (`order:create` $\rightarrow$ `order:validate` $\rightarrow$ `order:finalize`), with state accumulation and built-in loop protection.
- 🔄 **Offline Catch-Up & Reconnection Sync**: Reconnecting does **not** just pull the single latest message. It replays queued client packets, reconciles missed room history chronologically, and updates Facebook-style seen indicators.
- 🛡️ **Single-Writer Goroutine Safety**: Strict Gorilla WebSocket concurrency isolation with dedicated per-client write pumps (256-buffered channels) preventing socket corruption and isolating slow clients.
- 👥 **Room Management & Broadcast Targeting**: Built-in room join/leave protocols, targeted broadcasts, and sender exclusion (`Broadcast: client.Id()`).
- 🎮 **Full-Stack Demo Application**: Production-grade showcase featuring multi-room chat, WebRTC video calling, a multiplayer game loop with an automated AI opponent, and live action chaining visualizers.

---

## Architecture

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

## Detailed Documentation Guides

For comprehensive deep dives, refer to our specialized guides in [`docs/`](file:///home/lluie/ws/docs/):

| Guide | Description |
| :--- | :--- |
| 📖 **[Getting Started](file:///home/lluie/ws/docs/getting-started.md)** | Submodule installation, Go workspace setup, Gin integration, and client initialization. |
| 🔗 **[Action Chaining System](file:///home/lluie/ws/docs/action-chaining.md)** | Composing sequential pipelines, mutating `m.Data`, cycle detection, and game/order pipelines. |
| ⚡ **[Request-Reply Pattern](file:///home/lluie/ws/docs/request-reply.md)** | `await ws.request()`, correlation reply IDs, timeouts, and error handling. |
| 🔄 **[Offline Catch-Up & Reconnect Sync](file:///home/lluie/ws/docs/offline-reconnect-sync.md)** | Ring buffers, client outbound queue, message deduplication, and seen indicators. |
| 📚 **[Pure API Reference](file:///home/lluie/ws/docs/api-reference.md)** | Complete specification for Go structs (`Hub`, `Client`, `Message`) and TypeScript types (`Ws`, `Queue`). |
| 🚀 **[Interactive Demo Showcase](file:///home/lluie/ws/docs/demo-showcase.md)** | Tour of the full-stack demo in `demo/` and `demo/ui/` (Chat, WebRTC, Tic-Tac-Toe with AI). |

---

## Quick Start (Go Backend)

```go
package main

import (
	"github.com/gin-gonic/gin"
	ws "github.com/kainkent69/ginws"
)

func main() {
	r := gin.Default()

	// 1. Simple request-reply action
	ws.On("ping", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "pong",
			Payload: "pong response",
			Reply:   m.Reply, // Correlates with client's awaiting Promise
		})
		return "" // Chain finished
	})

	// 2. Multi-step Action Chaining pipeline
	ws.On("order:create", func(m *ws.Message, client *ws.Client) string {
		m.Data = map[string]any{"orderId": "ORD-123", "verified": true}
		return "order:finalize" // Chain to next named action
	})

	ws.On("order:finalize", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "order:confirmed",
			Payload: m.Data,
			Reply:   m.Reply,
		})
		return ""
	})

	// 3. Mount Gin WebSocket endpoint
	r.GET("/ws", ws.WSHandler(false))

	r.Run(":5000")
}
```

---

## Quick Start (TypeScript Client)

Import directly from the submodule path or compiled dist:

```typescript
import { Ws } from './ginws/client/ws.js';

// 1. Initialize client with auto-reconnection
const ws = new Ws('ws://localhost:5000/ws', {
  reconnectInterval: 2000,
  autoReconnect: true,
});

// 2. Await server response with modern async/await
async function testRequest() {
  try {
    const res = await ws.request<{ orderId: string }>(
      'order:create',
      { item: 'Laptop' },
      { timeout: 3000 }
    );
    console.log('Order created:', res.payload.orderId);
  } catch (err: any) {
    console.error('Request failed:', err.message);
  }
}

// 3. Listen for broadcast events
ws.use('announcement', (msg) => {
  console.log('Received announcement:', msg.payload);
});
```

---

## Running the Interactive Demo

Experience all features (chat channels, persona selection, WebRTC calls, Tic-Tac-Toe with AI, and action chaining visualizers) in the full-stack demo:

```bash
# Terminal 1: Run Go Backend
cd demo
go run main.go

# Terminal 2: Run SvelteKit Frontend
cd demo/ui
npm install
npm run dev
```

Then visit **`http://localhost:5173`** in your browser.

---

## Release History
For version history and detailed change notes, see [**CHANGELOG.md**](file:///home/lluie/ws/CHANGELOG.md).
