package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type Socket struct {
	id                 string
	connectionInfo     *ConnectionInfo
	connection         SocketConnection
	interceptorsMx     sync.Mutex
	interceptors       map[string]chan *InboundMessage
	associatedValuesMx sync.Mutex
	associatedValues   map[string]any
	roomsMx            sync.RWMutex
	rooms              map[string]*Room
	roomManager        *RoomManager
	closeMu            sync.Mutex
	closed             bool
	closeStatus        Status
	closeStatusSource  CloseSource
	closeReason        string
	ctx                context.Context
	cancelCtx          context.CancelFunc
}

var _ context.Context = &Socket{}

func NewSocket(info *ConnectionInfo, conn SocketConnection) *Socket {
	s := &Socket{
		id:               uuid.NewString(),
		connectionInfo:   info,
		connection:       conn,
		interceptors:     map[string]chan *InboundMessage{},
		associatedValues: map[string]any{},
		rooms:            map[string]*Room{},
	}

	s.ctx, s.cancelCtx = context.WithCancel(context.Background())

	return s
}

func (s *Socket) ID() string {
	return s.id
}

func (s *Socket) ConnectionInfo() *ConnectionInfo {
	return s.connectionInfo
}

func (s *Socket) Headers() http.Header {
	if s.connectionInfo != nil && s.connectionInfo.Headers != nil {
		return s.connectionInfo.Headers
	}

	return http.Header{}
}

// QueryParam returns the value of a query parameter from the WebSocket connection URL
func (s *Socket) QueryParam(key string) string {
	if s.connectionInfo != nil && s.connectionInfo.Query != nil {
		return s.connectionInfo.Query[key]
	}

	return ""
}

// QueryParams returns all query parameters from the WebSocket connection URL
func (s *Socket) QueryParams() map[string]string {
	if s.connectionInfo != nil && s.connectionInfo.Query != nil {
		return s.connectionInfo.Query
	}

	return make(map[string]string)
}

func (s *Socket) RemoteAddr() string {
	if s.connectionInfo != nil {
		return s.connectionInfo.RemoteAddr
	}

	return ""
}

func (s *Socket) Close(status Status, reason string, source CloseSource) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}

	s.closed = true
	s.closeStatus = status
	s.closeReason = reason
	s.closeStatusSource = source
	s.cancelCtx()
}

func (s *Socket) IsClosed() bool {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	return s.closed
}

func (s *Socket) Send(messageType MessageType, data []byte) error {
	return s.connection.Write(s.ctx, &SocketMessage{
		Type: messageType,
		Data: data,
	})
}

func (s *Socket) Set(key string, value any) {
	s.associatedValuesMx.Lock()
	defer s.associatedValuesMx.Unlock()
	s.associatedValues[key] = value
}

func (s *Socket) Get(key string) (any, bool) {
	s.associatedValuesMx.Lock()
	defer s.associatedValuesMx.Unlock()
	v, ok := s.associatedValues[key]
	return v, ok
}

func (s *Socket) MustGet(key string) any {
	s.associatedValuesMx.Lock()
	defer s.associatedValuesMx.Unlock()
	v, ok := s.associatedValues[key]
	if !ok {
		panic(fmt.Sprintf("key %s not found", key))
	}

	return v
}

func (s *Socket) Delete(key string) {
	s.associatedValuesMx.Lock()
	defer s.associatedValuesMx.Unlock()
	delete(s.associatedValues, key)
}

func (s *Socket) HandleNextMessageWithNode(node *HandlerNode) bool {
	msg, err := s.connection.Read(s)
	if err != nil {
		closeStatus := websocket.CloseStatus(err)
		if closeStatus != -1 {
			s.Close(Status(closeStatus), "", ClientCloseSource)
			return false
		}
		if errors.Is(err, context.Canceled) {
			return false
		}
		panic(fmt.Errorf("error reading socket message: %w", err))
	}

	go func() {
		inboundMsg := inboundMessageFromPool()
		inboundMsg.RawData = msg.RawData
		inboundMsg.Data = msg.Data
		inboundMsg.Meta = msg.Meta
		ctx := NewContextWithNodeAndMessageType(s, inboundMsg, node, msg.Type)
		ctx.Next()
		ctx.free()
	}()

	return true
}

func (s *Socket) HandleOpen(node *HandlerNode) {
	openCtx := NewContextWithNode(s, inboundMessageFromPool(), node)
	openCtx.Next()
	openCtx.free()
}

func (s *Socket) HandleClose(node *HandlerNode) {
	closeCtx := NewContextWithNode(s, inboundMessageFromPool(), node)
	closeCtx.Next()
	closeCtx.free()
}

func (s *Socket) GetInterceptor(id string) (chan *InboundMessage, bool) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()
	interceptorChan, ok := s.interceptors[id]
	return interceptorChan, ok
}

func (s *Socket) AddInterceptor(id string, interceptorChan chan *InboundMessage) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()
	s.interceptors[id] = interceptorChan
}

func (s *Socket) RemoveInterceptor(id string) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()
	delete(s.interceptors, id)
}

func (s *Socket) Deadline() (time.Time, bool) {
	return s.ctx.Deadline()
}

func (s *Socket) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *Socket) Err() error {
	return s.ctx.Err()
}

func (s *Socket) Value(key any) any {
	return s.ctx.Value(key)
}

func (s *Socket) SetRoomManager(rm *RoomManager) {
	s.roomManager = rm
}

func (s *Socket) Join(roomName string) {
	if s.roomManager == nil {
		return
	}

	s.roomsMx.Lock()
	defer s.roomsMx.Unlock()
	room := s.roomManager.Room(roomName)
	s.rooms[roomName] = room
	room.addSocket(s)
}

func (s *Socket) Leave(roomName string) {
	s.roomsMx.Lock()
	room, exists := s.rooms[roomName]
	if exists {
		delete(s.rooms, roomName)
	}

	s.roomsMx.Unlock()
	if exists && room != nil {
		room.removeSocket(s)
	}
}

func (s *Socket) Rooms() []string {
	s.roomsMx.RLock()
	defer s.roomsMx.RUnlock()
	rooms := make([]string, 0, len(s.rooms))
	for name := range s.rooms {
		rooms = append(rooms, name)
	}

	return rooms
}

func (s *Socket) leaveAllRooms() {
	s.roomsMx.Lock()
	rooms := s.rooms
	s.rooms = make(map[string]*Room)
	s.roomsMx.Unlock()
	for _, room := range rooms {
		if room != nil {
			room.removeSocket(s)
		}
	}
}
