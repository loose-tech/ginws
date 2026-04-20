# TASK-001: Implement Thread Safety for Global Maps

## Problem
`Clients` and `Rooms` maps are accessed concurrently without synchronization.

## Implementation
- Add `sync.RWMutex` to `model.go`.
- Wrap map access in `JoinRoom`, `WSHandler`, and `send`.

## Status
- [ ] Started
