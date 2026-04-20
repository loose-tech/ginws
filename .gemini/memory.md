# Project Memory - GinWS

## Project Overview
`ginws` is a Gin-based WebSocket library with a TypeScript client. It supports request-reply cycles, room broadcasting, and action chaining.

## Distribution Preference
- **Git Submodules**: Users are instructed to add the repo as a submodule to get both Go and TS source.

## Key Findings (2026-04-20)
- **Critical Concurrency:** Maps (`Clients`, `Rooms`) lack mutex protection.
- **Slow Consumer Bug:** `send` loop blocks on sequential `WriteJSON` calls.
- **Protocol Violation:** Concurrent writes to same connection (send vs writePump).
- **Resource Leaks:** No cleanup on client disconnect.
- **Timeout Bug:** Backend uses `Microsecond` instead of `Millisecond`.
- **Client Race:** `Queue.ts` has race conditions in buffer resolution.

## Workflow
- Plan: `.gemini/plan.md`
- Tasks: `.gemini/tasks.md`
- History: `.gemini/tasks/`
- Memory: `.gemini/memory.md`
