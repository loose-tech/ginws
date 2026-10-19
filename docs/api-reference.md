# Pure API Reference

Complete API specification for the **Go backend** (`ginws`) and the **TypeScript client** (`client/`).

---

## Navigation & Related Guides
- [📖 **Docs Hub**](file:///home/lluie/ws/docs/README.md)
- [🚀 **Getting Started**](file:///home/lluie/ws/docs/getting-started.md)
- [🔗 **Action Chaining System**](file:///home/lluie/ws/docs/action-chaining.md)
- [🔁 **Request-Reply Pattern**](file:///home/lluie/ws/docs/request-reply.md)
- [🔄 **Offline Catch-Up Protocol**](file:///home/lluie/ws/docs/offline-reconnect-sync.md)
- [📜 **Reconnection Rules & Caveats**](file:///home/lluie/ws/docs/reconnection-rules-and-caveats.md)
- [🎮 **Interactive Demo Showcase**](file:///home/lluie/ws/docs/demo-showcase.md)

---

## Part 1: Go Backend (`ginws`)

Package import:
```go
import ws "github.com/kainkent69/ginws"
```

### 1. Sentinel Errors

Defined in [`model.go`](file:///home/lluie/ws/model.go#L13-L19):

| Variable | Description |
| :--- | :--- |
| `ws.ErrFailedToProcess` | Returned when an inbound message title has no registered action handler on the Hub. |
| `ws.ErrFaildToProcess` | Deprecated alias for `ErrFailedToProcess`, retained for backward compatibility. |
| `ws.ErrNoClient` | Returned when attempting to dispatch to a client ID that is not connected. |
| `ws.ErrDeadline` | Returned when action processing exceeds the specified message duration (`context.DeadlineExceeded`). |
| `ws.ErrMaxChainDepth` | Returned when an action chain exceeds 50 iterations, preventing infinite loops. |

---

### 2. Struct: [`ws.Message`](file:///home/lluie/ws/model.go#L22-L42)

Represents messages exchanged across the WebSocket wire.

```go
type Message struct {
    Encoding   string        `json:"encoding"`
    Payload    any           `json:"payload"`
    Id         string        `json:"id"`
    Room       string        `json:"room"`
    Title      string        `json:"title"`
    DurationMs time.Duration `json:"duration"`
    Broadcast  string        `json:"broadcast"`
    Reply      string        `json:"reply"`
    Data       any           `json:"data,omitempty"`
}
```

#### Fields

| Field | Go Type | JSON Key | Description |
| :--- | :--- | :--- | :--- |
| `Encoding` | `string` | `"encoding"` | Character encoding format (defaults to `"utf8"`). |
| `Payload` | `any` | `"payload"` | Application payload data (string, struct, map, array, etc.). |
| `Id` | `string` | `"id"` | Target client ID or sender identity ID. |
| `Room` | `string` | `"room"` | Target room identifier for join/leave operations or room broadcasts. |
| `Title` | `string` | `"title"` | Message action name (e.g. `"ping"`, `"chat"`, `"order:create"`). |
| `DurationMs` | `time.Duration` | `"duration"` | Action timeout duration in milliseconds. In JSON, clients provide milliseconds as an integer (e.g. `3000`). GinWS wraps action processing in `context.WithTimeout(ctx, DurationMs * time.Millisecond)`. |
| `Broadcast` | `string` | `"broadcast"` | Client ID to exclude from broadcast (prevents echoing back to sender). |
| `Reply` | `string` | `"reply"` | Unique correlation identifier for Request-Reply workflows. Echoed back in responses. |
| `Data` | `any` | `"data,omitempty"` | Internal pipeline state mutated and passed across chained actions. Omitted from wire if empty/nil. |

---

### 3. Struct: [`ws.Client`](file:///home/lluie/ws/model.go#L45-L54)

Represents an active, connected WebSocket client session.

```go
type Client struct {
    Handshaker
    Conn        *websocket.Conn
    Joined      []string
    Credential  []string
    MessageChan chan Message
    // unexported synchronization fields
}
```

#### Properties

| Property | Type | Description |
| :--- | :--- | :--- |
| `Handshaker` | `Handshaker` | Embedded authentication and identity provider. Provides `.Id()` and `.Claims()`. |
| `Conn` | `*websocket.Conn` | Underlying Gorilla WebSocket connection instance. |
| `Joined` | `[]string` | Slice of room names that the client is currently subscribed to. |
| `Credential` | `[]string` | Optional client credential tokens or scope tags. |
| `MessageChan` | `chan Message` | Buffered output channel (capacity: 256) consumed by the client's `writePump`. |

#### Methods

- **`Send(msg Message) bool`**:
  Enqueues `msg` into the client's `MessageChan` for transmission by the dedicated `writePump` goroutine.
  - Thread-safe via internal mutex.
  - Returns `true` if enqueued successfully.
  - Returns `false` if client is closed or buffer (256 slots) is full (logs warning and drops frame to protect hub from slow consumers).
- **`Close()`**:
  Gracefully terminates the client connection: sets internal closed flag, closes `MessageChan`, and closes `Conn`. Safe for concurrent invocation.
- **`JoinRoom(name string) *Room`**:
  Subscribes the client to room `name`, creating the room if it does not yet exist.
- **`LeaveRoom(name string)`**:
  Unsubscribes the client from room `name`.
- **`Id() string`**:
  Returns the unique client identity string (delegated to embedded `Handshaker.Id()`).

---

### 4. Struct: [`ws.Room`](file:///home/lluie/ws/model.go#L106-L110)

Represents a named grouping of clients for targeted broadcasting.

```go
type Room struct {
    Name    string
    Window  map[string]*Client
    // unexported sync.RWMutex
}
```

- **`Name`**: Unique room identifier (e.g. `"general"`, `"room-99"`).
- **`Window`**: Map of active client IDs to their `*Client` references. Access is guarded by internal RWMutex.

---

### 5. Type: [`ws.Action`](file:///home/lluie/ws/model.go#L113)

```go
type Action func(m *Message, client *Client) string
```

The handler function signature for all action routes.
- **Parameters**:
  - `m *Message`: Pointer to the incoming message frame. Handlers can read `m.Payload`, mutate `m.Data`, or inspect `m.Reply`.
  - `client *Client`: Reference to the sender client session.
- **Return Value**:
  - Non-empty `string`: Triggers immediate chained dispatch to the next named action handler.
  - Empty `string` (`""`): Concludes the action pipeline.

---

### 6. Struct: [`ws.Hub`](file:///home/lluie/ws/model.go#L116-L126)

The central orchestrator managing connected clients, room memberships, authentication, and action pipelines.

#### Constructor

- **`ws.NewHub() *Hub`**:
  Allocates and initializes a new isolated `Hub` instance with thread-safe client, room, and action registries.

#### Configuration Methods

- **`SetAuther(a Auther)`**:
  Sets the authentication handler used during the WebSocket HTTP upgrade handshake.
- **`SetUpgrader(u websocket.Upgrader)`**:
  Replaces the default Gorilla WebSocket upgrader (allows customizing buffers, origin checking, or compression).
- **`SetOnDisconnect(fn func(client *Client))`**:
  Registers a callback hook executed in a background goroutine whenever a client disconnects.

#### Routing & Registration Methods

- **`On(route string, action Action)`**:
  Registers an action handler for the specified message title string.
- **`GetAction(route string) (Action, bool)`**:
  Retrieves a registered action handler by title.
- **`Register(c *Client)`**:
  Registers a client into the hub's active client map.
- **`GetClient(id string) (*Client, bool)`**:
  Retrieves a connected client by its identity string.
- **`ClientCount() int`**:
  Returns the number of currently connected clients.

#### Room Management Methods

- **`JoinRoom(client *Client, name string) *Room`**:
  Adds a client to the specified room.
- **`LeaveRoom(client *Client, name string)`**:
  Removes a client from the specified room.

#### Dispatch & Processing Methods

- **`Send(msg Message)`**:
  Routes a message to its intended destination:
  - If `msg.Room != ""` $\rightarrow$ broadcasts to all clients in that room (skipping client ID `msg.Broadcast` if set).
  - Else if `msg.Id != ""` $\rightarrow$ unicasts to the specific client with matching ID.
  - Else $\rightarrow$ broadcasts to all clients on the hub (skipping `msg.Broadcast` if set).
- **`Broadcast(msg Message)`**:
  Convenience method that clears `Id` and `Room` and dispatches `msg` to all connected clients.
- **`ProcessMessage(client *Client, message *Message) error`**:
  Executes the registered action pipeline for `message`. If `message.DurationMs > 0`, enforces timeout via `context.WithTimeout`.
- **`WSHandler(auth bool) gin.HandlerFunc`**:
  Returns a Gin HTTP handler function that validates authentication (if `auth == true`), upgrades the connection to WebSocket, registers the client, and launches `readPump` and `writePump` routines.

---

### 7. Package-Level Functions (`DefaultHub`)

GinWS provides convenience functions operating on the global [`ws.DefaultHub`](file:///home/lluie/ws/model.go#L302) singleton:

```go
var DefaultHub = NewHub()
var AuthHandler Auther

func On(route string, action Action)
func JoinRoom(client *Client, name string) *Room
func Send(msg Message)
func ProcessMessage(client *Client, message *Message) error
func WSHandler(auth bool) gin.HandlerFunc
```

---

### 8. Interfaces: [`Handshaker`](file:///home/lluie/ws/types.go#L5-L12) & [`Auther`](file:///home/lluie/ws/types.go#L14-L16)

```go
type Handshaker interface {
    Claims() any
    HandShake() error
    Id() string
}

type Auther interface {
    Check(ctx *gin.Context) (Handshaker, error)
}
```

- **[`DefaultHandshaker`](file:///home/lluie/ws/types.go#L21-L39)**:
  Built-in implementation of `Handshaker` returned for anonymous connections:
  - `ws.NewDefaultHandshaker(id string) *DefaultHandshaker`
  - `.Id()` returns `id`.
  - `.Claims()` and `.HandShake()` return `nil`.

---

## Part 2: TypeScript Client (`client/`)

Import path:
```typescript
import { Ws } from './client/ws.js';
import { Queue } from './client/queue.js';
import { gen12p3 } from './client/helper.js';
import type {
  Message,
  ActionFunc,
  RequestOptions,
  WsEventType,
  ReplyHandler,
  WsOptions,
} from './client/types.js';
```

---

### 1. Class: `Ws` ([`client/ws.ts`](file:///home/lluie/ws/client/ws.ts#L13-L356))

High-level WebSocket connection manager supporting automatic reconnection, offline buffering, typed request-reply, and action routing.

#### Constructor

```typescript
constructor(url: string | URL, protocolsOrOptions?: string | string[] | WsOptions)
```

- **`url`**: The target WebSocket server URL (e.g. `"ws://localhost:5000/ws"`).
- **`protocolsOrOptions`**: Either a protocol string/array or a [`WsOptions`](#5-interface-wsoptions) object.

#### Public Properties

| Property | Type | Description |
| :--- | :--- | :--- |
| `url` | `string` | Target WebSocket connection URL. |
| `initialize` | `boolean` | `true` if the socket has successfully connected at least once. |
| `reconnectInterval` | `number` | Interval in ms between reconnect attempts (default: `2000`). |
| `maxReconnectAttempts` | `number` | Maximum reconnection attempts (default: `Infinity`). |
| `autoReconnect` | `boolean` | Whether to automatically reconnect on unexpected close (default: `true`). |
| `attempts` | `number` | Current count of consecutive reconnect attempts. |
| `socket` | `WebSocket` | Underlying native browser/Node `WebSocket` instance. |
| `actionMap` | `Map<string, ActionFunc>` | Registered named client action handlers. |
| `replyMap` | `Map<string, ReplyHandler>` | Pending request-reply Promise handlers keyed by correlation ID. |

#### Core Methods

##### `request<T = any>(title: string, payload?: any, options?: RequestOptions): Promise<Message & { payload: T }>`
Dispatches a request frame with an auto-generated unique correlation ID (`reply`) and returns a Promise that awaits the server's reply.
- Automatically rejects if the server returns `title: "error"` or if the request times out.
- `options.timeout`: Expiration timeout in milliseconds.
- `options.room`: Target room context.

##### `send(title: string, payload?: any, room?: string): void`
Fire-and-forget message transmission. If the socket is not in `WebSocket.OPEN` state, automatically buffers the message in `outboundQueue` to transmit upon reconnection.

##### `use(title: string, action: ActionFunc): void`
Registers a client-side action handler. If `action` returns a string, chains execution to the next named handler (subject to client-side `maxDepth = 50`).

##### `on(event: WsEventType, handler: (data?: any) => void): () => void`
Registers an event listener for lifecycle events:
- Events: `"open"`, `"close"`, `"error"`, `"message"`, `"reconnect"`, `"reconnecting"`.
- Returns an unsubscribe function.

##### `off(event: WsEventType, handler?: (data?: any) => void): void`
Removes a specific event listener or all listeners for an event type.

##### `once(event: WsEventType, handler: (data?: any) => void): () => void`
Registers a one-time event listener that unsubscribes itself after first execution.

##### `fire(event: WsEventType, data?: any): void`
Manually triggers all registered listeners for a given event type.

##### `closeSafe(): void`
Intentionally and cleanly terminates the connection:
- Sets `isIntentionalClose = true` and `autoReconnect = false`.
- Clears pending reconnect timers.
- Closes the underlying socket.
- Clears `outboundQueue`.
- Rejects all pending request-reply Promises with `"WebSocket closed intentionally"`.

##### `sendObj(msg: Message, reply?: ActionFunc, expiredCb?: ReplyHandler["expiredAction"]): Promise<any>`
Sends a pre-structured `Message` frame with optional callback-based reply and timeout expiration handler.

##### `sendRaw(title: string, room?: string | null, data?: any, duration?: number, reply?: ActionFunc, expiredCb?: ReplyHandler["expiredAction"]): Promise<any>`
Backward-compatible raw sender supporting positional parameters.

##### `sendTo(room: string, title: string, data?: any, duration?: number, reply?: ActionFunc, expiredCb?: ReplyHandler["expiredAction"]): Promise<any>`
Convenience wrapper around `sendRaw` targeting a specific room.

##### `run(title: string, msg: Message): void`
Internal dispatcher called upon incoming message frame. Routes to `replyMap` if `msg.reply` matches, and/or to `actionMap` if `title` matches.

---

### 2. Class: [`Queue<T>`](file:///home/lluie/ws/client/queue.ts#L7-L61)

A generic, thread-safe FIFO queue utility designed for managing offline action queues and sequential packet streams.

#### Constructor

```typescript
const queue = new Queue<T>();
```

#### Methods & Accessors

- **`enqueue(data: T, id?: string | number): QueueItem<T>`**:
  Appends an item to the tail of the queue. Returns the created [`QueueItem<T>`](#7-interface-queueitemt). If `id` is omitted, generates a unique numeric timestamp identifier.
- **`dequeue(): QueueItem<T> | undefined`**:
  Removes and returns the item at the head of the queue, or `undefined` if empty.
- **`peek(): QueueItem<T> | undefined`**:
  Returns the head item without removing it, or `undefined` if empty.
- **`length: number`** (getter):
  Returns the current number of queued elements.
- **`isEmpty(): boolean`**:
  Returns `true` if `length === 0`.
- **`clear(): void`**:
  Discards all items currently in the queue.
- **`flush(): QueueItem<T>[]`**:
  Drains and returns all items currently in the queue, resetting length to 0.
- **`processEach(handler: (item: T) => Promise<void> | void): Promise<void>`**:
  Sequentially drains the queue by invoking `handler` for each item. Guarantees single-worker concurrency via an internal `processing` guard.

---

### 3. Interface: `Message` ([`client/types.ts`](file:///home/lluie/ws/client/types.ts#L1-L11))

```typescript
export interface Message {
  id?: string | undefined;
  room?: string | undefined;
  title: string;
  duration?: number | undefined;
  payload?: any;
  reply?: string | undefined;
  broadcast?: string | undefined;
  data?: any;
  encoding?: string | undefined;
}
```

---

### 4. Type: `ActionFunc` ([`client/types.ts`](file:///home/lluie/ws/client/types.ts#L13))

```typescript
export type ActionFunc = (
  msg: Message
) => string | undefined | void | Promise<string | undefined | void>;
```

---

### 5. Interface: `RequestOptions` ([`client/types.ts`](file:///home/lluie/ws/client/types.ts#L15-L19))

```typescript
export interface RequestOptions {
  room?: string | undefined;
  timeout?: number | undefined;
  onExpire?: (() => void) | undefined;
}
```

---

### 6. Type: `WsEventType` ([`client/types.ts`](file:///home/lluie/ws/client/types.ts#L21))

```typescript
export type WsEventType =
  | "open"
  | "close"
  | "error"
  | "message"
  | "reconnect"
  | "reconnecting";
```

---

### 7. Interface: `ReplyHandler` ([`client/types.ts`](file:///home/lluie/ws/client/types.ts#L23-L30))

```typescript
export interface ReplyHandler {
  key: string;
  action?: ActionFunc | undefined;
  resolve: (value: any) => void;
  reject: (reason?: any) => void;
  expiration: number;
  expiredAction?: ((reply: ReplyHandler, done: () => void) => void) | undefined;
}
```

---

### 8. Interface: `WsOptions` ([`client/types.ts`](file:///home/lluie/ws/client/types.ts#L32-L37))

```typescript
export interface WsOptions {
  protocols?: string | string[] | undefined;
  reconnectInterval?: number | undefined;
  maxReconnectAttempts?: number | undefined;
  autoReconnect?: boolean | undefined;
}
```

---

### 9. Interface: `QueueItem<T>` ([`client/queue.ts`](file:///home/lluie/ws/client/queue.ts#L1-L5))

```typescript
export interface QueueItem<T = any> {
  id: string | number;
  data: T;
  timestamp: number;
}
```

---

### 10. Helper Functions ([`client/helper.ts`](file:///home/lluie/ws/client/helper.ts))

- **`gen12p3(): string`**:
  Generates a high-entropy collision-resistant correlation ID composed of two random alphanumeric parts joined by a hyphen: `XXXXXXXXXXXX-YYY` (12 chars + 3 chars) using the 65-character alphabet `0-9A-Za-z!@-`.
