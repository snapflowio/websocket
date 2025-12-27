package main

import (
	"log"
	"net/http"

	websocket "github.com/snapflowio/websocket"
	"github.com/snapflowio/websocket/middleware/json"
)

type Message struct {
	Text string `json:"text"`
}

func main() {
	server := websocket.NewServer()
	if err := server.Use(json.Middleware()); err != nil {
		log.Fatal(err)
	}

	if err := server.On("message", func(ctx *websocket.Context) {
		var msg Message
		ctx.Unmarshal(&msg)
		log.Printf("Received: %s", msg.Text)
		ctx.Reply(Message{Text: "Hello from server!"})
	}); err != nil {
		log.Fatal(err)
	}

	if err := server.On("ping", func(ctx *websocket.Context) {
		ctx.Reply("pong")
	}); err != nil {
		log.Fatal(err)
	}

	log.Println("Server starting on :8081")
	http.Handle("/ws", server)
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
