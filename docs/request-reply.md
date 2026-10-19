# The Request-Reply Pattern

Raw WebSockets are inherently asynchronous and unidirectional. When a client sends a message frame, there is no native mechanism to `await` the corresponding server response. Traditionally, developers have had to juggle manual correlation maps, maintain brittle callback dictionaries, or maintain a dual architecture (HTTP REST for request-response queries, and WebSockets solely for server push).

**GinWS** natively solves this through its **Request-Reply Pattern**. With GinWS, you invoke `await ws.request(...)` just like a strongly-typed HTTP API call, while benefiting from the ultra-low latency, persistent connection, and multiplexing of WebSockets.

---

## Navigation & Related Guides
- [📖 **Docs Hub**](file:///home/lluie/ws/docs/README.md)
- [🚀 **Getting Started**](file:///home/lluie/ws/docs/getting-started.md)
- [🔗 **Action Chaining System**](file:///home/lluie/ws/docs/action-chaining.md)
- [🔄 **Offline Catch-Up Protocol**](file:///home/lluie/ws/docs/offline-reconnect-sync.md)
- [📜 **Reconnection Rules & Caveats**](file:///home/lluie/ws/docs/reconnection-rules-and-caveats.md)
- [📚 **API Reference**](file:///home/lluie/ws/docs/api-reference.md)
- [🎮 **Interactive Demo Showcase**](file:///home/lluie/ws/docs/demo-showcase.md)

---

## 1. Sequence & Protocol Architecture

```
TypeScript Client                                               Go Server (GinWS)
       │                                                                │
       │  1. await ws.request<OrderResp>("order:create", data, { timeout: 3000 })
       │     - Generates unique correlation ID: "a7B9x2Y1z0P4-9jK"      │
       │     - Registers Promise handlers in this.replyMap              │
       │     - Starts 3000ms client timeout timer                       │
       ├───────────────────────────────────────────────────────────────►│
       │     Frame: { title: "order:create", reply: "a7B9x...", ... }   │
       │                                                                │ 2. Process message
       │                                                                │    (or execute multi-step
       │                                                                │     Action Chain)
       │                                                                │
       │                                                                │ 3. Send response echoing
       │                                                                │    m.Reply correlation ID
       │◄───────────────────────────────────────────────────────────────┤
       │     Frame: { title: "order:completed", reply: "a7B9x...", ... }│
       │                                                                │
       │  4. Match msg.reply in replyMap                                │
       │     - Clears timeout timer                                     │
       │     - Deletes entry from replyMap                              │
       │     - Resolves Promise with typed payload                      │
       ▼                                                                ▼
```

### Correlation IDs via [`gen12p3()`](file:///home/lluie/ws/client/helper.ts#L17)

To ensure zero collision risk across concurrent requests from thousands of clients, GinWS uses [`gen12p3()`](file:///home/lluie/ws/client/helper.ts#L17):
- Format: `XXXXXXXXXXXX-YYY` (12 characters + hyphen + 3 characters).
- Alphabet: `0-9A-Za-z!@-` (65 characters).
- Total permutations: $65^{15} \approx 1.5 \times 10^{27}$, ensuring absolute uniqueness per request without needing coordination or UUID overhead.

---

## 2. TypeScript Client Usage

### Modern `async/await` Syntax: [`ws.request()`](file:///home/lluie/ws/client/ws.ts#L188-L193)

```typescript
import { Ws } from './client/ws.js';

const ws = new Ws('ws://localhost:5000/ws');

interface OrderResponse {
  orderId: string;
  status: string;
  total: number;
}

async function submitOrder() {
  try {
    // Send request and await typed response
    const response = await ws.request<OrderResponse>(
      'order:create',
      { item: 'Ergonomic Desk', qty: 1 },
      { timeout: 3000 } // Reject if server does not respond within 3000ms
    );

    console.log('Order ID:', response.payload.orderId);
    console.log('Status:', response.payload.status);
  } catch (err: any) {
    // Catches timeout errors or server rejection errors
    console.error('Order failed:', err.message);
  }
}
```

### Request Options ([`RequestOptions`](file:///home/lluie/ws/client/types.ts#L15-L19))

```typescript
export interface RequestOptions {
  room?: string | undefined;     // Route the request in the context of a room
  timeout?: number | undefined;  // Timeout duration in milliseconds
  onExpire?: (() => void) | undefined; // Custom timeout callback (optional)
}
```

### Room-Scoped Requests

To target a request to a specific room channel:

```typescript
const historyResponse = await ws.request(
  'room:messages:get',
  { limit: 50 },
  { room: 'dev-team', timeout: 5000 }
);

console.log('Room messages:', historyResponse.payload.messages);
```

---

## 3. Timeout Handling & Memory Safety

GinWS actively prevents memory leaks from unanswered requests:

1. When `ws.request()` is invoked with a timeout (e.g. `timeout: 3000`), a timer is started.
2. If the server does not reply before 3000ms:
   - The timer fires.
   - The entry is immediately deleted from `this.replyMap`.
   - The Promise rejects with:
     ```
     Error: WebSocket request 'order:create' timed out after 3000ms
     ```
3. **Late-Arriving Reply Protection**: If the server finishes slow processing and sends a response *after* the client timeout, the client's `run()` method inspects `this.replyMap`. Because the ID was already removed, the late frame is safely ignored without throwing unhandled rejection errors.

---

## 4. Server Error Propagation

If the Go backend fails during processing (e.g. unknown action handler, exceeded chain depth, or execution failure), GinWS automatically dispatches a reply frame with `title: "error"`:

```json
{
  "title": "error",
  "reply": "a7B9x2Y1z0P4-9jK",
  "payload": {
    "error": "maximum action chain depth exceeded"
  }
}
```

In [`client/ws.ts`](file:///home/lluie/ws/client/ws.ts#L311-L314), the client intercepts `msg.title === "error"` and **automatically rejects** the Promise:

```typescript
if (msg.title === "error") {
  const errMsg = msg.payload?.error || "Server returned an error";
  replyHandler.reject(new Error(errMsg));
} else {
  replyHandler.resolve(msg);
}
```

This ensures `try ... catch` blocks on the client receive actionable Go error strings seamlessly:

```typescript
try {
  await ws.request('invalid:route', {});
} catch (err: any) {
  console.error(err.message); // Logs: "failed to process"
}
```

---

## 5. Go Backend Implementation

### Standard Handler Reply

In any Go action handler, echo back `m.Reply` when sending the response:

```go
ws.On("calculate:tax", func(m *ws.Message, client *ws.Client) string {
    amount, _ := m.Payload.(float64)
    tax := amount * 0.0825

    client.Send(ws.Message{
        Title:   "calculate:tax:result",
        Payload: map[string]any{"tax": tax, "total": amount + tax},
        Reply:   m.Reply, // REQUIRED: Correlates with client's awaiting Promise
    })

    return "" // Pipeline complete
})
```

### Replying at the End of an Action Chain

When using multi-step action chaining, `m.Reply` is preserved throughout the pipeline. Only the **terminal handler** in the chain needs to invoke `client.Send()`:

```go
// Step 1
ws.On("chain:start", func(m *ws.Message, c *ws.Client) string {
    m.Data = "step-1"
    return "chain:step2"
})

// Step 2
ws.On("chain:step2", func(m *ws.Message, c *ws.Client) string {
    m.Data = m.Data.(string) + " -> step-2"
    return "chain:complete"
})

// Terminal Step: Replies back to the client
ws.On("chain:complete", func(m *ws.Message, c *ws.Client) string {
    c.Send(ws.Message{
        Title:   "chain:done",
        Payload: m.Data,
        Reply:   m.Reply, // Preserved from step 1!
    })
    return ""
})
```

---

## 6. Built-in Room Operations with Request-Reply

GinWS Go server provides built-in request-reply acknowledgments for room joining and leaving:

### Joining a Room with Acknowledgment

```typescript
// Await server room membership confirmation
const joinResp = await ws.request('join', 'engineering', { room: 'engineering' });
console.log(joinResp.payload); 
// Output: { room: "engineering", status: "joined" }
```

### Leaving a Room with Acknowledgment

```typescript
const leaveResp = await ws.request('leave', 'engineering', { room: 'engineering' });
console.log(leaveResp.payload);
// Output: { room: "engineering", status: "left" }
```

---

## 7. Lower-Level Sending Methods

In addition to `ws.request()`, the `Ws` class provides specialized sending methods:

### `ws.sendRaw()` (Positional Signature with Custom Callbacks)

```typescript
ws.sendRaw(
  'ping',          // title
  null,            // room
  { ping: true },  // payload
  2000,            // duration (timeout in ms)
  (msg) => {
    // Optional custom reply callback
    console.log('Received pong callback:', msg.payload);
    return '';
  },
  (replyHandler, done) => {
    // Optional custom expiration callback
    console.warn('Ping expired!');
    done();
  }
);
```

### `ws.sendTo()` (Room-Targeted Helper)

```typescript
ws.sendTo('lobby', 'room:ping', { test: true }, 2000);
```

### `ws.send()` (Fire-and-Forget)

When no response is expected, use `ws.send()` to avoid allocating correlation IDs and timers:

```typescript
ws.send('notify:typing', { user: 'Sarah' }, 'general');
```
