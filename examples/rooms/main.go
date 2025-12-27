package main

import (
	"log"
	"net/http"

	websocket "github.com/snapflowio/websocket"
	"github.com/snapflowio/websocket/middleware/json"
)

type JoinRoomMessage struct {
	Room string `json:"room"`
}

type ChatMessage struct {
	Room    string `json:"room"`
	Message string `json:"message"`
}

type DirectMessage struct {
	SocketID string `json:"socketId"`
	Message  string `json:"message"`
}

func main() {
	server := websocket.NewServer()
	if err := server.Use(json.Middleware()); err != nil {
		log.Fatal(err)
	}

	if err := server.UseOpen(func(ctx *websocket.Context) {
		log.Printf("Socket %s connected from %s", ctx.SocketID(), ctx.ConnectionInfo().RemoteAddr)
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.UseClose(func(ctx *websocket.Context) {
		log.Printf("Socket %s disconnected", ctx.SocketID())
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("join", func(ctx *websocket.Context) {
		var msg JoinRoomMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			log.Printf("Failed to unmarshal join message: %v", err)
			return
		}

		ctx.Join(msg.Room)
		log.Printf("Socket %s joined room: %s", ctx.SocketID(), msg.Room)
		ctx.To(msg.Room).Emit(map[string]string{
			"event":    "user_joined",
			"socketId": ctx.SocketID(),
		})

		ctx.Reply(map[string]any{
			"status": "joined",
			"room":   msg.Room,
		})
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("leave", func(ctx *websocket.Context) {
		var msg JoinRoomMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			log.Printf("Failed to unmarshal leave message: %v", err)
			return
		}

		ctx.Leave(msg.Room)
		log.Printf("Socket %s left room: %s", ctx.SocketID(), msg.Room)
		ctx.Reply(map[string]string{
			"status": "left",
			"room":   msg.Room,
		})
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("chat", func(ctx *websocket.Context) {
		var msg ChatMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			log.Printf("Failed to unmarshal chat message: %v", err)
			return
		}

		log.Printf("Broadcasting to room %s: %s", msg.Room, msg.Message)
		count := ctx.To(msg.Room).Emit(map[string]string{
			"message": msg.Message,
			"from":    ctx.SocketID(),
		})

		log.Printf("Sent to %d sockets in room %s", count, msg.Room)
		ctx.Reply(map[string]string{
			"status": "sent",
		})

	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("broadcast", func(ctx *websocket.Context) {
		var msg ChatMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			log.Printf("Failed to unmarshal broadcast message: %v", err)
			return
		}

		count := ctx.Broadcast(map[string]string{
			"message": msg.Message,
			"from":    ctx.SocketID(),
		})

		log.Printf("Broadcast sent to %d sockets", count)
		ctx.Reply(map[string]string{
			"status": "broadcast_sent",
		})
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("broadcast_except_me", func(ctx *websocket.Context) {
		var msg ChatMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return
		}

		count := ctx.BroadcastExceptMe(map[string]string{
			"message": msg.Message,
			"from":    ctx.SocketID(),
		})

		log.Printf("Broadcast sent to %d other sockets", count)
		ctx.Reply(map[string]string{
			"status": "broadcast_sent",
		})
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("direct_message", func(ctx *websocket.Context) {
		var msg DirectMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			log.Printf("Failed to unmarshal direct message: %v", err)
			return
		}

		err := ctx.EmitTo(msg.SocketID, map[string]string{
			"message": msg.Message,
			"from":    ctx.SocketID(),
		})

		if err != nil {
			log.Printf("Failed to send direct message: %v", err)
			ctx.Reply(map[string]string{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		ctx.Reply(map[string]string{
			"status": "sent",
		})
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("multi_room_chat", func(ctx *websocket.Context) {
		var msg struct {
			Rooms   []string `json:"rooms"`
			Message string   `json:"message"`
		}

		if err := ctx.Unmarshal(&msg); err != nil {
			log.Printf("Failed to unmarshal multi-room message: %v", err)
			return
		}

		count := ctx.ToRooms(msg.Rooms...).Emit(map[string]string{
			"message": msg.Message,
			"from":    ctx.SocketID(),
		})

		log.Printf("Multi-room broadcast sent to %d sockets in rooms %v", count, msg.Rooms)
		ctx.Reply(map[string]string{
			"status": "sent",
		})
	}); err != nil {
		log.Fatal(err)
	}

	log.Println("Server starting on :8081")
	http.Handle("/ws", server)
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
