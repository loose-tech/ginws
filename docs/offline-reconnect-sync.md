# Offline Catch-Up, Reconnection Synchronization & History Architecture

A common pitfall in real-time WebSocket systems is naive reconnection logic. In standard implementations:
- Disconnection causes in-flight outbound messages to fail or be dropped silently.
- Inbound messages broadcast while the user is offline are lost forever.
- Or, worse, naive systems simply retrieve the single "latest" state upon reconnecting, wiping out conversation context, missed events, and intermediate messages.

**GinWS** rejects the "latest snapshot only" anti-pattern. Instead, it implements a comprehensive **Offline Catch-Up, Outbound Queuing, and Timeline Reconciliation Protocol**. Reconnecting does **not** merely retrieve the single latest state; it restores the complete conversational history, guarantees delivery of offline user actions, eliminates duplicates, and resynchronizes Facebook-style read indicators and message recall states.

---

## 1. Architectural Philosophy: Why "Latest Only" Fails

```
❌ NAIVE RECONNECT: "LATEST SNAPSHOT ONLY" (ANTI-PATTERN)
═══════════════════════════════════════════════════════════════════════
[Client Drops] ──► [3 Messages Sent by Peers] ──► [Client Reconnects]
                                                         │
                                                         ▼
                                            Fetches only "Message 3".
                                            Messages 1 & 2 LOST FOREVER.
                                            Unsent offline inputs ERRORED.

✅ GINWS RECONNECT: FULL TIMELINE RECONCILIATION & OUTBOUND QUEUING
═══════════════════════════════════════════════════════════════════════
[Client Drops] ──► Outbound actions buffered in outboundQueue.
                ──► Peers post messages (server appends to ring buffer).
                ──► [Client Reconnects]
                          │
                          ├─► 1. ws.flushOutboundQueue() delivers queued user inputs.
                          ├─► 2. Client issues "room:messages:get".
                          ├─► 3. Server returns full message history.
                          ├─► 4. Client reconciles with Map<string, Message> deduplication.
                          └─► 5. Chronological sort & dispatch of missed read receipts.
```

When building interactive applications, chat channels, financial transactions, or collaborative tools, losing intermediate messages breaks application integrity:
1. **Missed Conversational Context**: If a team discusses a bug and posts a code patch followed by "approved", fetching only the word "approved" hides the patch and the context.
2. **Offline Typing & Action Loss**: Users often hit "Send" in subways or elevators with transient connection dips. Failing their inputs immediately causes frustration.
3. **Broken Read Receipts & Presence**: Without historical reconciliation, you cannot compute whether a user has read messages 1 through 10 or only message 10.

---

## 2. Tier 1: Client-Side Outbound Queuing

The TypeScript [`Ws`](file:///home/lluie/ws/client/ws.ts#L13-L356) client maintains an internal [`outboundQueue`](file:///home/lluie/ws/client/ws.ts#L26):

```typescript
// client/ws.ts
private outboundQueue: string[] = [];

private sendSafe(json: string): void {
  if (this.socket && this.socket.readyState === WebSocket.OPEN) {
    this.socket.send(json);
  } else {
    // Socket is connecting, closed, or reconnecting: safely buffer message
    this.outboundQueue.push(json);
  }
}
```

### Automatic Drain on Connection (`flushOutboundQueue`)

When the socket establishes or re-establishes an `OPEN` state, the client immediately flushes the queue in strict FIFO order:

```typescript
// client/ws.ts
private flushOutboundQueue() {
  while (this.outboundQueue.length > 0 && this.socket.readyState === WebSocket.OPEN) {
    const next = this.outboundQueue.shift();
    if (next) {
      this.socket.send(next);
    }
  }
}
```

This guarantees that:
- Messages sent while the user was offline are never dropped.
- Outbound messages are delivered in the exact order the user dispatched them.
- User code calling `ws.send()`, `ws.request()`, or `ws.sendRaw()` does not need to guard against connection state.

For specialized custom workflows, GinWS also provides the standalone generic [`Queue<T>`](file:///home/lluie/ws/client/queue.ts#L7-L61) class with `.enqueue()`, `.dequeue()`, `.flush()`, and async `.processEach()`.

---

## 3. Tier 2: Server-Side Ring Buffers & Historical Message Logs

The GinWS Go server maintains bounded, chronological in-memory ring buffers per room and per DM conversation ([`demo/main.go`](file:///home/lluie/ws/demo/main.go#L118-L174)):

```go
type ReadReceipt struct {
    UserID   string `json:"userId"`
    Username string `json:"username"`
    Avatar   string `json:"avatar"`
    ReadAt   string `json:"readAt"`
}

type StoredMessage struct {
    ID           string         `json:"id"`
    Room         string         `json:"room"`
    SenderID     string         `json:"senderId"`
    SenderName   string         `json:"senderName"`
    SenderAvatar string         `json:"senderAvatar"`
    Content      any            `json:"content"`
    Time         string         `json:"time"`
    Timestamp    int64          `json:"timestamp"`
    Recalled     bool           `json:"recalled"`
    ReadBy       []*ReadReceipt `json:"readBy"`
}

var (
    roomMessagesMu sync.RWMutex
    roomMessages   = make(map[string][]*StoredMessage)
)
```

Whenever any message is broadcast to a room via `chat`, the server appends it to `roomMessages[room]`:

```go
func appendRoomMessage(room string, msg *StoredMessage) {
    roomMessagesMu.Lock()
    defer roomMessagesMu.Unlock()

    const maxHistoryPerRoom = 200
    list := roomMessages[room]
    list = append(list, msg)

    // Bounded ring buffer: prune oldest messages to prevent memory exhaustion
    if len(list) > maxHistoryPerRoom {
        list = list[len(list)-maxHistoryPerRoom:]
    }
    roomMessages[room] = list
}
```

---

## 4. Tier 3: Reconnection Catch-Up Request Protocol

When a disconnected client reconnects, the client's `reconnect` listener fires:

```typescript
// demo/ui/src/routes/+page.svelte
wsInstance.on('reconnect', () => {
    status = 'connected';
    addLog('sys', 'Reconnected successfully');

    // 1. Re-query available rooms and permissions
    wsInstance?.sendRaw('rooms:get');

    // 2. Re-authenticate / restore user profile presence
    if (myProfile) {
        performLogin(myProfile.username, myProfile.avatar, myProfile.status);
    }

    // 3. Re-join active room and initiate full message history catch-up
    if (currentRoom) {
        wsInstance?.sendRaw('join', currentRoom, currentRoom);
        fetchRoomHistory(currentRoom);
    }
});

function fetchRoomHistory(roomName: string) {
    if (!wsInstance || status !== 'connected') return;
    wsInstance.sendRaw('room:messages:get', roomName, { room: roomName });
}
```

The server responds with the registered action handler:

```go
// demo/main.go
ws.On("room:messages:get", func(m *ws.Message, client *ws.Client) string {
    room := m.Room
    if room == "" {
        room = "general"
    }

    history := getRoomMessages(room)

    client.Send(ws.Message{
        Title: "room:messages:history",
        Room:  room,
        Payload: map[string]any{
            "room":     room,
            "messages": history,
        },
        Reply: m.Reply,
    })
    return ""
})
```

---

## 5. Tier 4: Client-Side Deduplication & Reconciliation Algorithm

A critical challenge when catching up is avoiding duplicate message rendering (e.g. if an optimistic local message was already displayed, or if a message arrived right before connection dropped).

GinWS resolves this through an **ID-indexed Map reconciliation algorithm**:

```typescript
// demo/ui/src/routes/+page.svelte
wsInstance.use('room:messages:history', (msg: Message) => {
    const { room, messages: incomingHistory } = msg.payload;

    // 1. Create a dictionary indexed by unique message ID
    const idMap = new Map<string, ChatMessage>();

    // 2. Ingest incoming historical messages from server
    if (Array.isArray(incomingHistory)) {
        for (const item of incomingHistory) {
            idMap.set(item.id, item);
        }
    }

    // 3. Preserve existing messages for the room (e.g. optimistic local sends)
    const existingRoomMessages = messages.filter((m) => m.room === room);
    for (const item of existingRoomMessages) {
        if (!idMap.has(item.id)) {
            idMap.set(item.id, item);
        } else {
            // Merge updated properties like readBy and recalled flag
            const serverItem = idMap.get(item.id)!;
            idMap.set(item.id, {
                ...item,
                ...serverItem,
                readBy: serverItem.readBy || item.readBy,
                recalled: serverItem.recalled !== undefined ? serverItem.recalled : item.recalled
            });
        }
    }

    // 4. Stable chronological sort based on server timestamp
    const mergedCurrent = Array.from(idMap.values()).sort(
        (a, b) => (a.timestamp || 0) - (b.timestamp || 0)
    );

    // 5. Update room messages in reactive state
    const otherRoomsMessages = messages.filter((m) => m.room !== room);
    messages = [...otherRoomsMessages, ...mergedCurrent];

    // 6. Automatically dispatch read receipts for caught-up messages
    for (const item of incomingHistory) {
        if (item.senderId !== myClientId && !hasRead(item)) {
            markMessageAsRead(item.id, room);
        }
    }
});
```

### Key Guarantees:
- **Zero Duplicates**: Even if the client already had messages 1 to 5, receiving history with messages 1 to 10 merges existing items cleanly by key.
- **Strict Chronological Order**: Out-of-order packet arrivals are normalized using `a.timestamp - b.timestamp`.
- **Merged Metadata**: Read receipts and recall status updates applied while offline are seamlessly reflected.

---

## 6. Tier 5: Local Storage Persistence Backup

To protect conversations against hard browser reloads or accidental tab closures, the client synchronizes local state with `localStorage`:

```typescript
// demo/ui/src/routes/+page.svelte
function saveMessagesBackup(items: ChatMessage[]) {
    try {
        if (typeof localStorage !== 'undefined') {
            // Persist the latest 100 messages locally
            localStorage.setItem('ginws_messages_cache', JSON.stringify(items.slice(-100)));
        }
    } catch (e) {
        console.warn('Failed to cache messages', e);
    }
}

function loadCachedMessagesBackup() {
    try {
        if (typeof localStorage !== 'undefined') {
            const cached = localStorage.getItem('ginws_messages_cache');
            if (cached) {
                const parsed = JSON.parse(cached);
                if (Array.isArray(parsed) && parsed.length > 0) {
                    messages = parsed;
                }
            }
        }
    } catch (e) {
        console.warn('Failed to load message backup', e);
    }
}
```

When a user opens the application, cached messages render instantly with zero blank-screen delay, followed immediately by background catch-up reconciliation with the Go server.

---

## 7. Tier 6: Facebook-Style Read Receipts & Seen Avatars

GinWS synchronizes message read states across the room timeline:

```
[ Alice: "Deploying v2.1 to production now." ]  (10:42 AM)
   └── Seen by: 👩‍💻 Sarah  🧙‍♂️ Alex  🤖 Bot
```

### Real-Time Flow:

1. **Viewing Inbound Messages**:
   When messages become visible, the client emits `message:read`:
   ```typescript
   wsInstance.sendRaw('message:read', currentRoom, {
       room: currentRoom,
       messageId: msg.id,
       userId: myClientId,
       username: myProfile.username,
       avatar: myProfile.avatar,
   });
   ```

2. **Server Updates Ring Buffer & Broadcasts**:
   The Go handler registers the receipt in memory and broadcasts `message:read_receipt`:
   ```go
   // demo/main.go
   ws.On("message:read", func(m *ws.Message, client *ws.Client) string {
       // Append ReadReceipt to stored message...
       ws.Send(ws.Message{
           Room:    room,
           Title:   "message:read_receipt",
           Payload: readData,
       })
       return ""
   })
   ```

3. **Catch-Up Synchronization**:
   When a disconnected client reconnects, the `room:messages:history` payload includes the populated `readBy` array for every stored message. Clients immediately render avatar chips under each message bubble without needing separate queries.

---

## 8. Tier 7: Message Recall (Unsend) Across History

GinWS supports retracting messages after they have been posted:

1. **Sender Requests Recall**:
   ```typescript
   wsInstance.sendRaw('message:recall', currentRoom, {
       room: currentRoom,
       messageId: targetMessageId,
   });
   ```

2. **Server Redaction in Ring Buffer**:
   ```go
   // demo/main.go
   ws.On("message:recall", func(m *ws.Message, client *ws.Client) string {
       // Locate message in roomMessages...
       msg.Recalled = true
       msg.Content = "[This message was recalled]"

       // Broadcast recall notice to all room members
       ws.Send(ws.Message{
           Room:    room,
           Title:   "message:recalled",
           Payload: map[string]any{"messageId": targetMessageId, "room": room},
       })
       return ""
   })
   ```

3. **Reconnection Integrity**:
   Because `msg.Recalled = true` is mutated directly in the server's ring buffer, a user reconnecting after a message was recalled will receive the redacted message in history rather than stale sensitive content.

---

## 9. Interactive Verification: The Offline Sync Simulator

The demo application ([`demo/ui/`](file:///home/lluie/ws/demo/ui/)) includes a dedicated simulator to experience this system live:

1. Open `http://localhost:5173` in your browser.
2. In the room chat header, locate the **`⚡ Test Offline Sync`** button.
3. Click the button:
   - The client forces a clean disconnect (`wsInstance.closeSafe()`).
   - The status badge turns amber: `⏳ Reconnecting...`.
4. While disconnected, type a new chat message and click **Send**.
   - Notice that the client does **not** crash or throw an error.
   - The message is buffered in `outboundQueue`.
5. Simultaneously, open a second browser window and send a message from a different user persona.
6. After 2.5 seconds, the simulator reconnects:
   - The client flushes its buffered message to the server.
   - The client requests `room:messages:get`.
   - Both your offline message and the peer's message appear in flawless chronological order.
   - Zero duplicated messages, zero lost messages.
