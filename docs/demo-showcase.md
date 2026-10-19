# Interactive Demo Showcase Guide

The GinWS repository includes a comprehensive, production-grade interactive showcase application in [`demo/`](file:///home/lluie/ws/demo/) that demonstrates every core capability of the library:
- **Go Backend Server**: [`demo/main.go`](file:///home/lluie/ws/demo/main.go) (port `5000`)
- **SvelteKit Modern Frontend**: [`demo/ui/`](file:///home/lluie/ws/demo/ui/) (port `5173`)

---

## 1. Quick Start: How to Run the Demo

### Terminal 1: Launch Go Backend Server

```bash
cd demo
go run main.go
```
- Server URL: `http://localhost:5000`
- WebSocket endpoint: `ws://localhost:5000/ws`
- Auth verification endpoint: `http://localhost:5000/check`

### Terminal 2: Launch SvelteKit Frontend UI

```bash
cd demo/ui
npm install # or pnpm install
npm run dev
```
- Web Application URL: `http://localhost:5173`

> **Pro Tip**: Open `http://localhost:5173` in two different browser windows (e.g. Chrome and Chrome Incognito) to interact between two independent user personas in real-time.

---

## 2. Feature-by-Feature Tour

### 1. Persona Selection & Identity Memory

Upon opening the demo, you can pick a persona or create a custom identity:
- **Pre-seeded Personas**: Neo (⚡), Sarah (👩‍💻 Lead Dev), Alex (🧙‍♂️ Architect), AI Bot (🤖).
- **Custom Profile**: Choose a custom display name, emoji avatar, and status tagline.
- **Identity Memory**: Stored in `localStorage` (`STORAGE_ACTIVE_IDENTITY_KEY`), so refreshing or reconnecting immediately restores your session and presence automatically without re-entering details.

---

### 2. Real-Time Multi-Room Channels & Protected Groups

The channel system demonstrates GinWS room routing and authorization approval workflows:
- **Channel Switcher**: Seamlessly toggle between `#general`, `#dev`, `#random`, and `#public-wall`.
- **Channel Creation**: Create public rooms or **Protected Groups** (`isProtected: true`).
- **Join Permission Workflow**:
  1. An unauthorized user attempts to enter a protected group $\rightarrow$ triggers `room:request_access`.
  2. The group creator receives a real-time badge: `🔔 X Join Request(s)`.
  3. Clicking the badge opens the **Owner Review Modal**.
  4. Upon clicking **Approve**, the server emits `room:approve_access`, adds the user to the room's access list, and grants immediate entry.

---

### 3. Action Chaining Visualizer

Navigate to the **Action Chaining** panel to inspect multi-step server pipelines:
- **Pipeline Stages**: `order:create` $\rightarrow$ `order:validate` $\rightarrow$ `order:finalize`.
- **Live Latency & State Inspection**:
  1. `order:create`: Ingests raw payload and attaches ISO timestamps.
  2. `order:validate`: Verifies inventory levels and authorization checks.
  3. `order:finalize`: Assigns order ID (`ORD-7749`), confirms status, and replies back to the client.
- The UI renders each step's progression and payload state transformation in real-time with round-trip millisecond latency metrics.

---

### 4. Modern Request-Reply Workbench

Test typed `await ws.request()` calls interactively:
- Select from pre-configured actions (`ping`, `echo`, `slow_echo`, `order:create`) or type custom action names.
- Customize the JSON payload and timeout duration (ms).
- **Timeout & Error Testing**: Send `slow_echo` with a 500ms timeout to watch the Promise automatically reject with a descriptive timeout error without stalling the WebSocket connection.

---

### 5. Offline Simulator & History Recovery

Experience the 7-tier offline catch-up protocol with a single click:
1. In the chat header, click **`⚡ Test Offline Sync`** (title: *"Simulate dropping offline for 2.5s and automatically catching up on all messages"*).
2. The connection badge changes to `⏳ Reconnecting...`.
3. Type and send a chat message while disconnected — notice it does **not** fail; it is buffered in `outboundQueue`.
4. Concurrently send a message from a second browser window.
5. After 2.5 seconds, the client reconnects automatically.
6. The client automatically flushes your queued message and issues `room:messages:get`, reconciling all missed messages chronologically without duplicates!

---

### 6. Facebook-Style Read Receipts & Seen Avatars

- **Floating Seen Avatars**: As you view incoming messages, mini avatar chips float beneath each message bubble showing who has read up to that point.
- **Real-Time Read Receipts**: Viewing a message triggers `message:read`, broadcasting `message:read_receipt` to all room members.
- **Message Recall (Unsend)**: Hover over your sent messages and click **Recall**. The message is immediately updated for all room members to `[This message was recalled]`, suppressing attachments while preserving chat history order.

---

### 7. Multiplayer Tic-Tac-Toe Game (Action Chained Game Loop)

Play real-time Tic-Tac-Toe powered by Go action chaining:
- **Game Loop Pipeline**: `game:move` $\rightarrow$ `game:validate` $\rightarrow$ `game:evaluate`.
- **Live Board Sync**: State synchronized across all connected players.
- **Scoreboard**: Live tracking for Player X, Player O, and Draws.
- **Automated AI Opponent**: When playing alone as Player X, the server simulates an AI opponent for Player O after a 600ms thinking delay (`🤖 AI Opponent is replying...`).
- **Board Reset**: Emits `game:reset` to clear the 3x3 board and reset turn to "X".

---

### 8. Direct Messaging & Peer Directory

- **Sidebar Peer Directory**: Real-time list of all connected users with online presence indicators.
- **1-on-1 Direct Messaging**: Private conversations with unread notification badges.
- **File & Photo Drag-and-Drop**: Drag images or documents directly into DM or room chat feeds.
- **Interactive Bot Personas**: Sending a DM to `sys-bot`, `user-sarah`, or `user-alex` triggers intelligent automated responses with simulated typing latency and "✓ Seen" read receipts.

---

### 9. Photo Sharing & 2-Way WebRTC Video Calling

- **Camera Viewfinder**: Real-time webcam viewfinder (with an automated canvas fallback video stream if camera hardware or permissions are unavailable).
- **Snapshot Sharing (`photo:share`)**: Capture a photo snapshot and broadcast it instantly to active channels.
- **2-Way WebRTC Video Calling**:
  - Full WebRTC signaling conducted over GinWS (`signal:call`, `signal:answer`, `signal:ice`, `signal:hangup`).
  - Incoming call notification modal with Accept and Decline actions.

---

### 10. Public Broadcast Wall

- **Global Bulletin Banner**: A persistent announcement strip displayed at the top of the chat view.
- **Instant Shouts**: Broadcast announcements to all connected users across all rooms via `wall:post`.
- **Feed Hydration**: Reconnected clients receive historical posts via `wall:get`.

---

### 11. Diagnostic Live Event Stream & Latency Monitor

- **Event Inspector**: Real-time stream of all WebSocket events categorized into:
  - System (`sys`)
  - Inbound (`in`)
  - Outbound (`out`)
  - Error (`err`)
- **Expandable Payloads**: Click any log entry to inspect the raw JSON message payload.
- **Heartbeat Monitor**: Measures round-trip ping/pong latency continuously.
