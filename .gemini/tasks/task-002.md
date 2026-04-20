# TASK-002: Refactor `send` to be Non-Blocking

## Problem
Currently `send` calls `WriteJSON` directly. If multiple clients are targeted (broadcast), a single slow client blocks the whole loop.

## Implementation
- Modify `send` to iterate over clients and push to their `MessageChan` instead of writing to the socket.
- The `writePump` will be the ONLY place where `WriteMessage` or `WriteJSON` is called.

## Status
- [ ] Started
