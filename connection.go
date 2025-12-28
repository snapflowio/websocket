package websocket

import (
	"context"

	"github.com/coder/websocket"
)

type MessageType = websocket.MessageType

const (
	MessageText   MessageType = websocket.MessageText
	MessageBinary MessageType = websocket.MessageBinary
)

type SocketMessage struct {
	Type    MessageType
	RawData []byte
	Data    []byte
	Meta    map[string]any
}
type SocketConnection interface {
	Read(ctx context.Context) (*SocketMessage, error)
	Write(ctx context.Context, msg *SocketMessage) error
	Close(status Status, reason string) error
}
type WebSocketConnection struct {
	conn *websocket.Conn
}

var _ SocketConnection = &WebSocketConnection{}

func NewWebSocketConnection(conn *websocket.Conn) *WebSocketConnection {
	return &WebSocketConnection{
		conn: conn,
	}
}

func (c *WebSocketConnection) Read(ctx context.Context) (*SocketMessage, error) {
	messageType, data, err := c.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	return &SocketMessage{
		Type:    messageType,
		RawData: data,
	}, nil
}

func (c *WebSocketConnection) Write(ctx context.Context, msg *SocketMessage) error {
	return c.conn.Write(ctx, msg.Type, msg.Data)
}

func (c *WebSocketConnection) Close(status Status, reason string) error {
	return c.conn.Close(websocket.StatusCode(status), reason)
}
