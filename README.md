# Snapflow WebSocket

WebSocket framework for Go inspired by Socket.IO. Route messages by event name, organize connections into rooms, and broadcast to groups.

Works with any Go HTTP router via the standard `http.Handler` interface.

## Installation

```bash
go get github.com/snapflowio/websocket
```

## Quick Example

```go
package main

import (
    "log"
    "net/http"
    ws "github.com/snapflowio/websocket"
    "github.com/snapflowio/websocket/middleware/json"
)

func main() {
    server := ws.NewServer()
    server.Use(json.Middleware())

    server.UseOpen(func(ctx *ws.Context) {
        log.Printf("Connected: %s", ctx.SocketID())
    })

    server.On("join", func(ctx *ws.Context) {
        var data struct {
            Username string `json:"username"`
            Room     string `json:"room"`
        }
        ctx.Unmarshal(&data)

        ctx.SetOnSocket("username", data.Username)
        ctx.Join(data.Room)

        ctx.To(data.Room).Emit(map[string]string{
            "event":    "user_joined",
            "username": data.Username,
        })
    })

    server.On("chat", func(ctx *ws.Context) {
        var data struct {
            Room string `json:"room"`
            Text string `json:"text"`
        }
        ctx.Unmarshal(&data)

        username := ctx.MustGetFromSocket("username").(string)

        ctx.To(data.Room).Emit(map[string]interface{}{
            "event":    "message",
            "username": username,
            "text":     data.Text,
        })
    })

    http.Handle("/ws", server)
    log.Fatal(http.ListenAndServe(":8081", nil))
}
```

Client side:

```javascript
const ws = new WebSocket('ws://localhost:8081/ws');

ws.send(JSON.stringify({
    event: 'join',
    data: { username: 'Alice', room: 'general' }
}));

ws.send(JSON.stringify({
    event: 'chat',
    data: { room: 'general', text: 'Hello!' }
}));

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.event === 'message') {
        console.log(`${msg.data.username}: ${msg.data.text}`);
    }
};
```

## Event Routing

Route messages by event name:

```go
server.On("ping", func(ctx *ws.Context) {
    ctx.Reply(map[string]string{"event": "pong"})
})

// Wildcards
server.On("user.*", handleUser)        // Matches user.login, user.logout
server.On("game.**", handleGame)       // Matches game.start, game.player.move, etc
```

## Rooms

Group connections together:

```go
server.On("join_room", func(ctx *ws.Context) {
    ctx.Join("lobby")
    ctx.Join("notifications")
})

// Check membership
if ctx.InRoom("admin") {
    // admin stuff
}

// Get all rooms
rooms := ctx.Rooms()
```

Rooms are automatically cleaned up when connections close.

## Broadcasting

```go
// Everyone
ctx.Broadcast(data)

// Everyone except sender
ctx.BroadcastExceptMe(data)

// Specific room
ctx.To("lobby").Emit(data)

// Multiple rooms
ctx.ToRooms("room1", "room2").Emit(data)

// Room except sender
ctx.To("lobby").ExceptMe().Emit(data)

// Direct message by socket ID
ctx.EmitTo(socketID, data)
```

## Context Storage

Two types:

```go
// Per-message (cleared after handler)
ctx.Set("requestID", id)
value := ctx.Get("requestID")

// Per-connection (persists until disconnect)
ctx.SetOnSocket("userID", id)
value := ctx.GetFromSocket("userID")
```

Use socket storage for user data, auth state, etc. Use message storage for request-specific stuff.

## Middleware

```go
// Global
server.Use(func(ctx *ws.Context) {
    log.Printf("Event: %s", ctx.Event())
    ctx.Next()
})

// Pattern-specific
server.On("admin.*", func(ctx *ws.Context) {
    if !isAdmin(ctx) {
        ctx.Emit(map[string]string{"error": "unauthorized"})
        return
    }
    ctx.Next()
})
```

Built-in middleware:

```go
import (
    "github.com/snapflowio/websocket/middleware"
    "github.com/snapflowio/websocket/middleware/json"
)

server.Use(json.Middleware())
server.Use(middleware.Logger(logger))
server.Use(middleware.Recovery(logger))
server.Use(middleware.RateLimit(100, time.Second))
server.Use(middleware.RequestID())
server.Use(middleware.Timeout(30 * time.Second))
```

## Connection Lifecycle

```go
server.UseOpen(func(ctx *ws.Context) {
    // Connection established
    token := ctx.Headers().Get("Authorization")
    user := authenticate(token)
    ctx.SetOnSocket("user", user)
})

server.UseClose(func(ctx *ws.Context) {
    // Connection closing
    user := ctx.GetFromSocket("user")
    log.Printf("User %v disconnected", user)
})

// Close programmatically
ctx.Close()
ctx.CloseWithStatus(ws.StatusPolicyViolation, "banned")
```

## Request/Response

Server can request data from clients:

```go
server.On("delete", func(ctx *ws.Context) {
    var confirmation struct{ Confirmed bool }

    err := ctx.RequestInto(map[string]string{
        "event": "confirm_delete",
    }, &confirmation)

    if err != nil {
        ctx.Reply(map[string]string{"error": "timeout"})
        return
    }

    if confirmation.Confirmed {
        // delete stuff
    }
})

// Custom timeout
ctx.RequestIntoWithTimeout(data, &response, 30*time.Second)
```

## Message Format

Using JSON middleware, messages look like:

```json
{
    "event": "chat.message",
    "id": "optional-request-id",
    "data": {
        "text": "Hello",
        "room": "general"
    }
}
```

You can write custom middleware for other formats.

## Context Lifecycle

**Important:** Contexts are pooled and reused. If your handler needs to keep using the context, it must block:

```go
// BAD - context gets recycled
server.On("subscribe", func(ctx *ws.Context) {
    go func() {
        ctx.Emit(update) // WRONG - context already recycled
    }()
})

// GOOD - handler blocks
server.On("subscribe", func(ctx *ws.Context) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            ctx.Emit(update)
        case <-ctx.Done():
            return
        }
    }
})
```

## HTTP Routers

Works with any router:

```go
// net/http
http.Handle("/ws", server)

// gorilla/mux
mux.Handle("/ws", server)

// chi
r.Handle("/ws", server)

// gin
gin.Default().Any("/ws", gin.WrapH(server))
```

## Configuration

```go
server := ws.NewServer()

// Restrict origins
server.SetOrigins([]string{
    "https://myapp.com",
})

// Custom logger
server.SetLogger(logrus.New())
```

## License

This library is available under the MIT License.