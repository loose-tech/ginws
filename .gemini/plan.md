# GinWS Improvement Plan

## Objective
Stabilize the `ginws` library by fixing concurrency bugs, protocol violations, and resource leaks.

## Phase 1: Backend Stabilization
1. **Thread Safety:** Mutex protection for `Clients` and `Rooms`.
2. **Write Sync:** Consolidate all writes into `writePump`.
3. **Lifecycle:** Implement `unregister` on disconnect.
4. **Timeouts:** Fix Micro/Milli unit bug.

## Phase 2: Client Reliability
1. **Queue Fix:** Resolve race conditions in `Queue.ts`.
2. **Type Sync:** Align `Message` interfaces.
3. **Cleanup:** Ensure listener removal on reconnect.
