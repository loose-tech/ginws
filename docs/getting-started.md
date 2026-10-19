# Getting Started with GinWS

This guide walks you through integrating **GinWS** into your Go backend and pairing it with the TypeScript client.

---

## 1. Installation

GinWS is designed to be added as a **Git submodule** directly into your project. This keeps your Go backend models, protocol definitions, and TypeScript client in strict lockstep across frontend and backend repositories.

### Step 1: Add as Git Submodule

From your repository root:

```bash
git submodule add -b main https://github.com/kainkent69/ginws ginws
```

### Step 2: Configure Go Workspace or `go.mod`

If using a Go workspace (`go.work`):

```bash
go work use ./ginws
```

Or configure your `go.mod` to reference the local submodule path:

```go
require github.com/kainkent69/ginws v0.0.0

replace github.com/kainkent69/ginws => ./ginws
```

### Step 3: TypeScript Client Path

The client source is located in `ginws/client`. You can import the TypeScript source directly if your bundler (Vite, Rollup, SvelteKit, Webpack) supports TypeScript path aliases or relative imports:

```typescript
// Import Ws class and types directly
import { Ws } from '../ginws/client/ws.js';
import type { Message, ActionFunc, RequestOptions } from '../ginws/client/types.js';
```

Alternatively, you can compile the client with `pnpm build` inside `ginws/client` to emit the ECMAScript modules in `ginws/client/dist/`.

---

## 2. Go Backend Setup

### Minimal Example (Using `DefaultHub`)

GinWS provides a package-level singleton `DefaultHub` for rapid prototyping:

```go
package main

import (
	"github.com/gin-gonic/gin"
	ws "github.com/kainkent69/ginws"
)

func main() {
	r := gin.Default()

	// Register an action handler
	ws.On("ping", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "pong",
			Payload: "pong response from server",
			Reply:   m.Reply, // Echo reply ID if client used request-reply
		})
		return "" // Chain finished
	})

	// Mount the WebSocket endpoint (false disables auth requirement)
	r.GET("/ws", ws.WSHandler(false))

	r.Run(":5000")
}
```

### Production Example (Isolated Hub & Custom Authentication)

For production applications with multi-tenant isolation, custom authentication, or custom WebSocket upgrader settings, instantiate an explicit [`Hub`](file:///home/lluie/ws/model.go#L116-L126):

```go
package main

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	ws "github.com/kainkent69/ginws"
)

// 1. Implement ws.Handshaker to represent verified identity
type CustomUserSession struct {
	userId   string
	username string
}

func (s *CustomUserSession) Claims() any      { return s.username }
func (s *CustomUserSession) HandShake() error { return nil }
func (s *CustomUserSession) Id() string       { return s.userId }

// 2. Implement ws.Auther to extract credentials from Gin Context
type TokenAuther struct{}

func (a *TokenAuther) Check(ctx *gin.Context) (ws.Handshaker, error) {
	token := ctx.Query("token")
	if token == "" {
		token = ctx.GetHeader("Authorization")
	}
	if token != "secret-token-123" {
		return nil, errors.New("invalid or missing token")
	}
	return &CustomUserSession{
		userId:   "usr_9981",
		username: "Sarah",
	}, nil
}

func main() {
	r := gin.Default()

	// Create an independent Hub instance
	hub := ws.NewHub()

	// Configure authentication provider
	hub.SetAuther(&TokenAuther{})

	// Optional: Custom gorilla/websocket Upgrader
	hub.SetUpgrader(websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
		CheckOrigin: func(r *http.Request) bool {
			// Enforce your trusted origins
			return true
		},
	})

	// Optional: Disconnect hook
	hub.SetOnDisconnect(func(client *ws.Client) {
		println("Client disconnected:", client.Id())
	})

	// Register business actions on the hub
	hub.On("greet", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "greet:reply",
			Payload: "Hello, " + client.Id(),
			Reply:   m.Reply,
		})
		return ""
	})

	// Mount WebSocket handler with authentication enabled (auth = true)
	r.GET("/ws", hub.WSHandler(true))

	r.Run(":5000")
}
```

---

## 3. Gorilla WebSocket Concurrency Safety

A primary benefit of GinWS is eliminating concurrent write panics. Gorilla WebSocket connections **do not allow concurrent writers**. 

In GinWS:
- Every client has an isolated `writePump()` goroutine that is the sole owner of socket writes.
- Calls to `client.Send(msg)` from any goroutine simply enqueue the message into a thread-safe 256-slot channel (`MessageChan`).
- If a client's network hangs or stalls, GinWS drops buffer overruns with a warning log rather than blocking the hub or other connected clients.

---

## 4. TypeScript Client Usage

### Instantiation & Connection Lifecycle

The [`Ws`](file:///home/lluie/ws/client/ws.ts#L13-L356) class manages the native `WebSocket` object, handles reconnects automatically, queues offline messages, and correlates incoming replies.

```typescript
import { Ws } from './client/ws.js';

// Instantiate client
const ws = new Ws('ws://localhost:5000/ws', {
  reconnectInterval: 2000,    // Time between reconnect attempts (ms)
  maxReconnectAttempts: 10,   // Max retries before stopping (default: Infinity)
  autoReconnect: true,        // Automatically reconnect on abnormal close
});

// Event: Connection opened (or re-opened)
ws.on('open', () => {
  console.log('Connected to GinWS server');
});

// Event: Reconnect triggered (fired on successful reconnection)
ws.on('reconnect', (instance) => {
  console.log('Successfully reconnected! Resynchronizing state...');
});

// Event: In progress reconnection attempt
ws.on('reconnecting', ({ attempt }) => {
  console.warn(`Connection lost. Attempting reconnection #${attempt}...`);
});

// Event: Connection closed
ws.on('close', () => {
  console.log('Connection closed');
});

// Event: WebSocket error
ws.on('error', (err) => {
  console.error('WebSocket encountered error:', err);
});

// Clean termination:
// ws.closeSafe();
```

---

## 5. Sending & Receiving Messages

### A. Fire-and-Forget (`ws.send`)

Use [`ws.send()`](file:///home/lluie/ws/client/ws.ts#L196-L203) when you do not need a reply from the server:

```typescript
// Send notification to server
ws.send('notify', { timestamp: Date.now() });

// Send message targeted to a specific room
ws.send('chat', { text: 'Hello everyone!' }, 'lobby');
```

### B. Listening for Messages (`ws.use`)

Register action listeners on the client using [`ws.use(title, handler)`](file:///home/lluie/ws/client/ws.ts#L183-L185):

```typescript
ws.use('announcement', (msg) => {
  console.log('Broadcast received:', msg.payload);
});
```

### C. Built-in Room Operations (`join` / `leave`)

The GinWS Go server provides built-in handlers for `"join"` and `"leave"`:

```typescript
// Join the room using request-reply to await acknowledgment
const reply = await ws.request('join', 'dev-room', { room: 'dev-room' });
console.log('Successfully joined room:', reply.room);

// Leave room
await ws.request('leave', 'dev-room', { room: 'dev-room' });
```

---

## 6. Next Steps

- Learn how to structure multi-step workflows in **[Action Chaining System](file:///home/lluie/ws/docs/action-chaining.md)**.
- Explore synchronous-style calling in **[Request-Reply Pattern](file:///home/lluie/ws/docs/request-reply.md)**.
- Implement reliable message delivery in **[Offline Catch-Up & Reconnect Sync](file:///home/lluie/ws/docs/offline-reconnect-sync.md)**.
- Review full type and method definitions in **[Pure API Reference](file:///home/lluie/ws/docs/api-reference.md)**.
- Explore all live UI features in **[Interactive Demo Showcase](file:///home/lluie/ws/docs/demo-showcase.md)**.
