# Changelog

All notable changes to the **GinWS** project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- **Reconnection Rules & Architecture Guide** ([`docs/reconnection-rules-and-caveats.md`](file:///home/lluie/ws/docs/reconnection-rules-and-caveats.md)):
  - Definitive architectural documentation detailing the core mental model: WebSockets as real-time transport deltas rather than offline storage engines.
  - The 5 Cardinal Rules of Reconnection (state on data, inbox summary pattern, lazy loading on open, and zero-duplicate UUID Map reconciliation).
  - Comprehensive breakdown of critical caveats: reconnection packet storms, naive single-state replacement, in-flight race conditions, and optimistic UI stalls.
- **Server-Side Inbox Summary Protocol** ([`demo/main.go`](file:///home/lluie/ws/demo/main.go#L1470-L1605)):
  - Registered `inbox:summary` action handler computing per-user unread counts, last message snippets, and timestamps across all active channels and direct message threads.
  - Prevents packet storms by transmitting lightweight conversation summaries upon reconnect instead of blasting full histories across all rooms.
- **Sidebar Unread Badge Indicators** ([`demo/ui/src/routes/+page.svelte`](file:///home/lluie/ws/demo/ui/src/routes/+page.svelte)):
  - Reactive `roomUnread` tracking displaying badge counters (`[ 2 ]`, `[ 5 ]`) next to inactive channel items in the sidebar.
  - Automatic badge reset when selecting or opening a channel or direct message thread.
  - Re-synchronized on both initial socket `open` and subsequent `reconnect` events.
- **Documentation Navigation Network** ([`docs/`](file:///home/lluie/ws/docs/)):
  - Unified header and footer navigation bars cross-linking all 8 guides:
    - [📖 Main Docs Hub](file:///home/lluie/ws/docs/README.md)
    - [🚀 Getting Started](file:///home/lluie/ws/docs/getting-started.md)
    - [🔗 Action Chaining System](file:///home/lluie/ws/docs/action-chaining.md)
    - [🔁 Request-Reply Pattern](file:///home/lluie/ws/docs/request-reply.md)
    - [🔄 Offline Catch-Up Protocol](file:///home/lluie/ws/docs/offline-reconnect-sync.md)
    - [📜 Reconnection Rules & Caveats](file:///home/lluie/ws/docs/reconnection-rules-and-caveats.md)
    - [📚 Pure API Reference](file:///home/lluie/ws/docs/api-reference.md)
    - [🎮 Demo Showcase Guide](file:///home/lluie/ws/docs/demo-showcase.md)
- **Unified Public Location & Channels**:
  - Direct integration of the Public Wall into the main workspace via the `#public-wall` channel and a persistent top broadcast banner.
  - Bidirectional mirroring between `wall:post` broadcasts and `#public-wall` room messages.
- **Interactive Direct Messaging & Peer Simulation**:
  - Interactive persona bots (`sys-bot`, `user-sarah`, `user-alex`) with automated smart replies and read receipt dispatch.
  - Direct 1-on-1 WebRTC video calling with in-app signaling (`signal:call`, `signal:answer`, `signal:ice`, `signal:hangup`).
- **Mini-Game AI Opponent via Action Chaining**:
  - Multi-step game loop: `game:move` $\rightarrow$ `game:validate` $\rightarrow$ `game:evaluate`.
  - Automated Player O AI opponent response with turn locking and thinking state indicator.
- **Multimedia Drag & Drop Input**:
  - Drag-and-drop zone supporting images, audio files, and videos with live upload preview chips.
- **Browser Persona Persistence**:
  - LocalStorage caching of chosen identity, avatar, and custom status with optional auto-login on startup.
- **Protected Groups & Permission Requests**:
  - Private group creation with access approval modals (`room:request_access` and `room:approve_access`).
- **Facebook-Style Seen Indicators**:
  - Per-message `ReadBy` slice with avatar chips rendered under each message bubble showing who has read up to that point.
- **Message Recall (Unsend)**:
  - Dynamic message retraction and broadcast redaction (`message:recall` $\rightarrow$ `message:recalled`).

### Changed
- Reconnection lifecycle in client now synchronizes inbox badge summaries first, followed by lazy loading of the active room's messages.
- Message history retrieval bounded to 200 items per room with automatic pruning in memory ring buffers.

### Fixed
- Eliminated duplicate messages upon reconnection using UUID-indexed Map reconciliation.
- Resolved race conditions between in-flight outbound messages and socket dropouts using the FIFO `outboundQueue`.

---

## [1.0.0] - 2026-10-15

### Added
- **Core Go WebSocket Engine (`ginws`)**:
  - High-performance WebSocket Hub integrating with the Gin framework (`ws.WSHandler`, `ws.AuthHandler`).
  - Strict Gorilla WebSocket concurrency isolation with dedicated per-client write pumps (256-buffered channels) preventing socket corruption and isolating slow clients.
  - Non-blocking 16MB message read pumps with pong heartbeat synchronization (54s ping ticker).
- **TypeScript Client Library (`client/`)**:
  - Type-safe `Ws` class supporting auto-reconnect, exponential backoff, and event listeners (`open`, `close`, `error`, `reconnect`, `reconnecting`, `message`).
  - FIFO `outboundQueue` to buffer actions while disconnected and automatically flush on socket open.
  - Standalone generic `Queue<T>` utility.
- **Request-Reply Pattern**:
  - `await ws.request(title, payload, { timeout })` promise method with unique correlation reply IDs.
- **Server-Side Action Chaining**:
  - Handlers route to subsequent actions by returning their string name, with `m.Data` mutation and cycle/depth protection.
- **Room Management & Broadcast Targeting**:
  - Native room join and leave protocols with targeted broadcasting (`ws.Send`) and sender exclusion (`Broadcast: client.Id()`).
