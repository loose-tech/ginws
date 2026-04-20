# GinWS

Powerful WebSocket tooling for Gin with a built-in request-reply TypeScript client.

## Features
- **Request-Reply Pattern**: Await server responses directly in TypeScript.
- **Action Chaining**: Go handlers can route to next actions by name.
- **Independent Queues**: One slow client won't block the entire server.
- **Submodule Friendly**: Designed to be embedded directly in your project.

## Installation (Git Submodule)

Add GinWS as a submodule to keep your backend and frontend source in sync:

```bash
# 1. Add the submodule
git submodule add -b main https://github.com/kainkent69/ginws <path>

# 2. Add to your Go workspace
go work use <path>

# 3. Configure go.mod
# require github.com/kainkent69/ginws v0.0.0
# replace github.com/kainkent69/ginws => <path>
```

## Quick Start (Go)

```go
import ws "github.com/kainkent69/ginws"

func main() {
    ws.On("ping", func(m *ws.Message, client ws.Client) string {
        client.Send(ws.Message{Title: "pong", Reply: m.Reply})
        return "" 
    })
    
    // ... setup gin and r.GET("/ws", ws.WSHandler(true))
}
```

## Client (TypeScript)

Client files are stored in `<path>/client`.

### Usage
Import the `Ws` class from the submodule path:
```typescript
import { Ws } from '<path>/client/ws';

```
