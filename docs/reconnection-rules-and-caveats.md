# WebSocket Reconnection Rules, Caveats & Architecture Guide

A definitive architectural guide for handling offline drops, network transitions, and seamless reconnection in real-time WebSocket systems without buffer bloat, packet storms, or state desynchronization.

---

## Navigation & Related Guides
- [📖 **Docs Hub**](file:///home/lluie/ws/docs/README.md)
- [🚀 **Getting Started**](file:///home/lluie/ws/docs/getting-started.md)
- [🔄 **Offline Catch-Up Protocol**](file:///home/lluie/ws/docs/offline-reconnect-sync.md)
- [🔗 **Action Chaining System**](file:///home/lluie/ws/docs/action-chaining.md)
- [🔁 **Request-Reply Pattern**](file:///home/lluie/ws/docs/request-reply.md)
- [📚 **API Reference**](file:///home/lluie/ws/docs/api-reference.md)
- [🎮 **Interactive Demo Showcase**](file:///home/lluie/ws/docs/demo-showcase.md)

---

## 1. Executive Summary & The Core Mental Model

When engineers build real-time applications (chat, notifications, collaborative tools, live dashboards), a common mistake is trying to make the **WebSocket connection act as a stateful database and offline message broker**.

This leads to memory leaks, reconnect packet storms, out-of-order race conditions, and duplicated data.

### The Golden Rule of WebSocket Architecture:
> **WebSockets are for real-time notification deltas ("pokes"). The data model and APIs are the sources of state.**

```
+─────────────────────────────────────────────────────────────────────────────+
|                         THE IDEAL SEPARATION OF CONCERNS                    |
+─────────────────────────────────────────────────────────────────────────────+
|                                                                             |
|  1. WEBSOCKET LAYER (Lightweight Real-Time Transport)                       |
|     - Low-latency event streaming ("user X posted in #engineering").        |
|     - Instant transient pings ("typing", "read_receipt", "peer_calling").   |
|     - Does NOT hoard gigabytes of offline history.                         |
|                                                                             |
|  2. DATA MODEL LAYER (Source of Truth)                                      |
|     - Every message has state: ID, timestamp, room/DM, readBy/isRead.       |
|     - Persisted in database or bounded server memory.                       |
|                                                                             |
|  3. INBOX OVERVIEW & LAZY-LOAD (Bandwidth & Reconnect Efficiency)           |
|     - Reconnect does NOT blast 1,000 messages across 20 rooms.              |
|     - Reconnect fetches lightweight INBOX SUMMARY: unread counts & badges.  |
|     - Message history is lazily loaded ONLY when user opens that chat.      |
|                                                                             |
+─────────────────────────────────────────────────────────────────────────────+
```

---

## 2. The 5 Cardinal Rules of Reconnection

### Rule 1: Never Put a Heavy "Offline Storage Engine" in the Socket Client
- **What to do**: The client socket should only maintain a short-lived **outbound FIFO buffer** (e.g. [`outboundQueue`](file:///home/lluie/ws/client/ws.ts#L26)) to hold actions typed by the user while the socket is physically transitioning between states (e.g. elevator, subway cell switch).
- **Why**: Trying to store historical messages, unread flags, and multi-room sync machines inside the socket transport library couples network drivers to application domain logic.

### Rule 2: State Belongs to the Data, Not the Connection
- Attach delivery and read metadata directly to the message record:
  ```json
  {
    "id": "msg-109283-userA",
    "room": "engineering",
    "senderId": "userA",
    "content": { "text": "Deploying build #42" },
    "timestamp": 1776602400000,
    "recalled": false,
    "readBy": [
      { "userId": "userA", "readAt": "10:00:00" },
      { "userId": "userB", "readAt": "10:00:15" }
    ]
  }
  ```
- Whether a user is online or offline, the server records the message with the sender's receipt. When another user views it, they send `message:read`, updating the receipt list.

### Rule 3: Use the "Inbox Summary" Pattern (Badges First, Messages Later)
When a user reconnects after 2 hours offline:
- **WRONG**: Replaying 500 missed messages across 10 channels down the WebSocket connection.
- **RIGHT**: The client requests an **Inbox Summary**:
  ```json
  {
    "inbox": [
      { "roomId": "general", "unreadCount": 0, "lastMessage": "Sounds great!" },
      { "roomId": "engineering", "unreadCount": 5, "lastMessage": "Build completed" },
      { "partnerId": "user-sarah", "unreadCount": 2, "lastMessage": "Can you review PR?" }
    ]
  }
  ```
- The user's sidebar instantly lights up with unread badges (`[5]` on `#engineering`, `[2]` on Sarah).

### Rule 4: Lazy Load History on Open
- When the user clicks into `#engineering`, the client initiates a focused catch-up for that room only (`room:messages:get` or `fetch('/api/rooms/engineering/messages')`).
- If the user never clicks on Sarah's DM during this session, Sarah's message history is never transmitted over the wire, saving battery, bandwidth, and CPU.

### Rule 5: Zero-Duplicate ID-Indexed Merging
- Network reconnection inevitably causes edge-case overlaps (e.g., an incoming message was pushed right before the disconnect, or an optimistic message was rendered locally).
- Always reconcile inbound lists using a `Map<string, Message>` keyed by unique message UUID. Never blindly `push()` or `concat()` arrays.

---

## 3. Critical Caveats & Anti-Patterns to Avoid

### Caveat 1: The Reconnection Packet Storm
- **Scenario**: A momentary server restart or regional ISP blip disconnects 10,000 active clients simultaneously.
- **Failure Mode**: All 10,000 clients reconnect in the exact same second, and each client's socket sends 10 requests requesting full histories of all rooms. The server crashes under a 100,000-request avalanche.
- **Remedy**:
  1. Add **jittered exponential backoff** to reconnection intervals:
     ```typescript
     const delay = Math.min(maxInterval, baseInterval * 1.5 ** attempt) + Math.random() * 500;
     ```
  2. Request **only** the inbox badge counts upon reconnection.

### Caveat 2: The "Latest Snapshot Only" Naive Reconnect
- **Failure Mode**: Reconnecting simply requests `GET /latest_message`. If 3 messages were sent while offline, messages 1 and 2 are permanently lost, breaking conversations and corrupting user context.
- **Remedy**: The active room fetch returns the slice of unread messages (or bounded recent window, e.g. latest 50 messages), which are merged into existing local history by UUID.

### Caveat 3: Race Conditions on In-Flight Outbound Messages
- **Failure Mode**: User hits "Send". The socket drops mid-flight. Did the server receive it or not?
- **Remedy**:
  - Outbound messages carry a client-generated UUID `id: "msg-" + Date.now() + "-" + clientId`.
  - If the client re-sends the buffered message upon reconnecting, the server checks its recent ID index and discards the duplicate if it was already stored.

### Caveat 4: Optimistic UI Stalls
- **Failure Mode**: UI blocks user typing until the server acknowledges with a 200 OK.
- **Remedy**: Render the message immediately in the UI with a pending state (`status: 'sending'`), buffer it in `outboundQueue`, and update to `status: 'sent'` once the socket drains.

---

## 4. End-to-End Implementation Blueprint

### Step 1: Server Data Model & Inbox Computation (Go)

```go
// demo/main.go
type InboxSummaryItem struct {
    ID          string `json:"id"`
    Type        string `json:"type"` // "room" or "dm"
    Name        string `json:"name"`
    UnreadCount int    `json:"unreadCount"`
    LastText    string `json:"lastText"`
    LastTime    string `json:"lastTime"`
    LastTs      int64  `json:"lastTs"`
}

// WS Action Handler: Calculate inbox badges for reconnecting or active client
ws.On("inbox:summary", func(m *ws.Message, client *ws.Client) string {
    uid := client.Id()
    var items []InboxSummaryItem

    // 1. Compute room unread counts based on ReadBy slice
    roomMessagesMu.RLock()
    for room, msgs := range roomMessages {
        unread := 0
        var lastMsg *StoredMessage
        for _, sm := range msgs {
            if !sm.Recalled {
                lastMsg = sm
                if sm.SenderID != uid {
                    hasRead := false
                    for _, rb := range sm.ReadBy {
                        if rb.UserID == uid {
                            hasRead = true
                            break
                        }
                    }
                    if !hasRead {
                        unread++
                    }
                }
            }
        }
        item := InboxSummaryItem{
            ID:          room,
            Type:        "room",
            Name:        "#" + room,
            UnreadCount: unread,
        }
        if lastMsg != nil {
            item.LastText = formatContentText(lastMsg.Content)
            item.LastTime = lastMsg.Time
            item.LastTs = lastMsg.Timestamp
        }
        items = append(items, item)
    }
    roomMessagesMu.RUnlock()

    client.Send(ws.Message{
        Title:   "inbox:summary",
        Payload: items,
    })
    return ""
})
```

---

### Step 2: Client Lifecycle & Reconnection Hook (TypeScript / Svelte)

```typescript
// demo/ui/src/routes/+page.svelte
let inboxUnreadMap = $state<Record<string, number>>({});

wsInstance.on('reconnect', () => {
    status = 'connected';
    addLog('sys', 'Reconnected - Syncing inbox summaries');

    // 1. Re-authenticate / claim user identity
    if (myProfile) {
        performLogin(myProfile.username, myProfile.avatar, myProfile.status);
    }

    // 2. Fetch Inbox Summary: gets unread badges for all rooms & DMs
    wsInstance?.sendRaw('inbox:summary');

    // 3. Re-join only the currently active room & fetch its history
    if (currentRoom) {
        wsInstance?.sendRaw('join', currentRoom, currentRoom);
        fetchRoomHistory(currentRoom);
    }
});

// Register Inbox Summary listener
wsInstance.use('inbox:summary', (msg: Message) => {
    if (Array.isArray(msg.payload)) {
        for (const item of msg.payload) {
            inboxUnreadMap[item.id] = item.unreadCount;
        }
    }
    return '';
});
```

---

### Step 3: Lazy-Loading and Marking as Read on View

When the user selects a room or opens a chat:
```typescript
function selectRoom(roomName: string) {
    currentRoom = roomName;
    wsInstance?.sendRaw('join', roomName, roomName);

    // 1. Lazy load messages for this room
    fetchRoomHistory(roomName);

    // 2. Reset local unread counter for this room
    inboxUnreadMap[roomName] = 0;

    // 3. Mark all unread messages as read to notify server & peers
    const unread = messages.filter((m) => m.room === roomName && !hasRead(m));
    for (const m of unread) {
        markMessageAsRead(m.id, roomName);
    }
}
```

---

## 5. Architectural Comparison Matrix

| Attribute | Naive "Latest Only" | Bloated WS Offline Engine | GinWS Ideal Pattern |
| :--- | :--- | :--- | :--- |
| **Transport Role** | Socket does everything | Socket acts as offline buffer | Socket is lightweight real-time pipe |
| **Reconnect Traffic** | 1 snapshot message | 500+ messages flooded across all rooms | **Lightweight summary (unread badges)** |
| **Memory Overhead** | Low (data lost) | Very high (duplicate ring buffers in socket layer) | **Low & bounded** (server data layer only) |
| **Bandwidth Usage** | Minimal | Wasteful (downloads chats user never opens) | **Optimal** (lazy-loads active chat only) |
| **Read Receipts** | Not supported | Fragile | **Native per-message `readBy` array** |
| **Message Recall** | Not supported | Desynchronizes | **Server state mutation reflected in history** |

---

## 6. Verification in the Interactive Demo

To see this pattern operating live:
1. Run the demo server: `go run ./demo/main.go`
2. Run the Svelte client: `pnpm --dir demo/ui dev`
3. Navigate to `http://localhost:5173`.
4. Open two browser windows with different user identities (`Neo` and `Sarah`).
5. In Window 1 (`Sarah`), click **`⚡ Test Offline Sync`** in the room chat header.
6. In Window 2 (`Neo`), switch to `#engineering` and post 3 messages.
7. Notice:
   - Window 1 reconnects after 2.5s.
   - The `#engineering` tab in Window 1 displays an unread badge indicator: `[ 3 ]`.
   - Window 1 did not have to download all messages until clicking into `#engineering`.
   - Clicking `#engineering` lazily loads the 3 messages and dispatches seen receipts back to Window 2.

---

## 7. Further Reading & Documentation Links
- [📖 **Main Documentation Hub**](file:///home/lluie/ws/docs/README.md)
- [🔄 **Offline Catch-Up Protocol Details**](file:///home/lluie/ws/docs/offline-reconnect-sync.md)
- [🚀 **Getting Started Guide**](file:///home/lluie/ws/docs/getting-started.md)
- [🔗 **Action Chaining Pipeline**](file:///home/lluie/ws/docs/action-chaining.md)
- [🔁 **Request-Reply Pattern**](file:///home/lluie/ws/docs/request-reply.md)
- [📚 **Go & TypeScript API Reference**](file:///home/lluie/ws/docs/api-reference.md)
- [🎮 **Demo Application Showcase**](file:///home/lluie/ws/docs/demo-showcase.md)
