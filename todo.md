# GinWS Project Roadmap & Access Guide (`todo.md`)

## 1. Quick Access & How to Run

### Backend Server (Gin + GinWS)
```bash
cd demo
go run main.go
# Server listens on: http://localhost:5000
# WebSocket endpoint: ws://localhost:5000/ws
# Auth verification: http://localhost:5000/check
```

### Modern Frontend Client UI (`demo/ui/` - SvelteKit)
```bash
cd demo/ui
npm run dev
# Web interface available at: http://localhost:5173
```

---

## 2. Core Goals & Implementation Checklist

### A. Real-Time Multi-Room Chat System ("Chant System")
- [x] Room join and leave actions (`title: "join"`, `title: "leave"`) with server acknowledgment.
- [x] Room-targeted message broadcasting ([`ws.Send`](file:///home/lluie/ws/demo/main.go#L70-L80)).
- [x] Multi-channel room switcher (`#general`, `#dev`, `#random`, and custom rooms).
- [x] Active room presence badge and live message timeline with timestamps and sender IDs.
- [x] Self vs. other message distinction and auto-scrolling message container.

### B. Request-Reply & Action Chaining System
- [x] Modern direct async/await request method ([`ws.request()`](file:///home/lluie/ws/client/ws.ts#L187-L194)).
- [x] Backward-compatible callback pattern ([`ws.sendRaw()`](file:///home/lluie/ws/client/ws.ts#L250-L270)).
- [x] Timeout handling and error reporting with rejection.
- [x] 3-Step Named Action Chaining Visualizer:
  - Step 1: `order:create` $\rightarrow$ parses payload and timestamps
  - Step 2: `order:validate` $\rightarrow$ verifies inventory and auth
  - Step 3: `order:finalize` $\rightarrow$ assigns order ID and replies to client
- [x] Round-trip latency display (ms) and raw payload inspector.

### C. Real-Time Client System & Dashboard UI (`demo/ui`)
- [x] Live WebSocket connection status badge (Connected, Reconnecting, Disconnected).
- [x] Client identity display (`user-with-id=X` from handshake).
- [x] Auto-reconnect toggle and custom server URL input.
- [x] Live heartbeat ping/pong latency monitor.
- [x] Real-time diagnostic event stream (filtering inbound, outbound, errors, and system events).
- [x] Disconnect / Reconnect / Reset controls.

### D. User Login & Identity ([`demo/main.go`](file:///home/lluie/ws/demo/main.go#L148-L185))
- [x] Sign-in modal with display name input and emoji avatar picker.
- [x] Server-side user profile registration via `auth:login`.
- [x] Profile persistence and active session header badge.

### E. Online Peer Discovery & Direct Messaging (DM)
- [x] Real-time sidebar listing all connected peers with avatars and usernames.
- [x] Real-time presence tracking with online/offline disconnect detection ([`model.go`](file:///home/lluie/ws/model.go#L169-L198)).
- [x] Private 1-on-1 direct messaging (`title: "dm"`) targeted to any user ID.
- [x] In-app conversation feed with sender identification.

### F. The Public Broadcast Wall ([`demo/main.go`](file:///home/lluie/ws/demo/main.go#L240-L290))
- [x] Live community bulletin board that broadcasts shouts/announcements to all clients.
- [x] Post composer with instant broadcast via `wall:post`.
- [x] Historical posts hydration for newly connected clients via `wall:get`.

### G. Video Call & Photo-to-Photo ("phot to phot")
- [x] In-browser camera viewfinder (`getUserMedia`).
- [x] Snapshot Photo Capture with instant room sharing (`photo:share`).
- [x] 2-Way WebRTC Video Call with GinWS signaling (`signal:call`, `signal:answer`, `signal:ice`, `signal:hangup`).
- [x] Incoming call notification modal with Accept and Decline actions.

### H. Real-Time Multiplayer Mini-Game (Tic-Tac-Toe via Action Chaining)
- [x] Interactive 3x3 game arena synchronized across all connected players.
- [x] Multi-step backend pipeline running the game loop:
  `game:move` ➔ `game:validate` ➔ `game:evaluate`
- [x] Live scoreboard (Player X, Player O, Draws) and board reset.

### I. Persona Selection, Identity Memory & Protected Groups
- [x] Remembered persona selection screen ("Who Do You Want to Be Today?") with local storage persistence.
- [x] Custom status/title and avatar selector.
- [x] Channel & Protected Group creation with settings (Topic, Icon, Privacy).
- [x] Group join permission flow (`room:request_access` ➔ Owner Review Modal ➔ `room:approve_access`).

### J. Offline Message Catch-Up, History Fetch & Reconnect Sync ("Catch when offline")
- [x] Server-side in-memory ring-buffer message history per room and DM conversation.
- [x] Reconnect & room-switch sync (`room:messages:get` / `room:messages:history`) to automatically fetch all missed messages after being offline or disconnected.
- [x] Seamless client message reconciliation avoiding duplicates while restoring chronological order.

### K. Message Recall & Unsend System ("Recallable in a room")
- [x] Sender message recall action (`message:recall`).
- [x] Server-side message redaction and broadcast (`message:recalled`).
- [x] UI update displaying "[This message was recalled]" with recall indicator badge and attachment suppression.

### L. Facebook-Style Read Indicators & Last Read Tracker ("Facebook like read indicator")
- [x] Automatic read receipt tracking when client views incoming room or DM messages (`message:read`).
- [x] Server-side read receipt registry (`readBy: []ReadReceipt`) preserving user ID, username, avatar, and read timestamp.
- [x] Real-time read receipt broadcasting (`message:read_receipt`).
- [x] Facebook Messenger-style mini avatar chips under each message bubble showing who has seen up to that point.

### M. Unified Whole-App Architecture, Direct Messaging/Calling & Mini-Game AI Opponent
- [x] **Unified Public Location & Channels:**
  - Integrated the Public Wall directly into the main workspace via the `#public-wall` channel and a top broadcast shout banner.
  - Bidirectional mirroring between `wall:post` shouts and `#public-wall` room messages.
- [x] **Interactive Direct Messaging & Peer Calling:**
  - Online peer directory always seeded with interactive personas (`sys-bot`, `user-sarah`, `user-alex`).
  - Automated intelligent response and "✓ Seen" read receipt dispatch when sending DMs to demo peers.
  - Direct 1-on-1 WebRTC calling accessible from DM headers, sidebar user rows, and the Photo & Video Place.
  - Unread DM counters on nav tabs, sidebar peers, and DM lists with interactive incoming toast alerts.
- [x] **Mini-Game Automated Opponent (Action Chaining Pipeline):**
  - Integrated an automated AI move response for Player O in `game:evaluate` (600ms latency simulation).
  - UI thinking indicator (`🤖 AI Opponent is replying...`) and turn-based move locking for Player X.
- [x] **Photo & Video Hub:**
  - Camera preview with automatic high-fidelity simulated stream fallback for environments without webcams.
  - One-click snapshot broadcast to active rooms and public feeds.
  - Peer call launcher chips for instant video calls.

---

## 3. Architecture Overview

```
[SvelteKit UI: demo/ui] (Port 5173)
        │
        │ WebSocket (ws://localhost:5000/ws)
        ▼
[GinWS Server: demo/main.go] (Port 5000)
   ├── Handshake / Auth: ws.AuthHandler
   ├── User Registry: auth:login, users:list, presence, personas (sys-bot, user-sarah, user-alex)
   ├── Rooms & Groups: rooms:get, room:create, room:request_access, room:approve_access
   ├── Message Memory & Catch-up:
   │     ├── room:messages:get ➔ room:messages:history (offline recovery)
   │     ├── message:recall ➔ message:recalled (unsend / recall engine)
   │     └── message:read ➔ message:read_receipt (Facebook-style read tracking)
   ├── Direct Messaging: dm ➔ dm_sent, automated peer replies & read receipts
   ├── Unified Broadcast Wall: wall:post, wall:get, bidirectional sync with #public-wall
   ├── WebRTC Signaling: signal:call, signal:answer, signal:ice, photo:share
   ├── Game Engine: game:move ➔ game:validate ➔ game:evaluate ➔ automated AI opponent response
   └── Gorilla WebSocket Single-Writer Pump
```

