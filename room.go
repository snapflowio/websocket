package websocket

import (
	"github.com/sirupsen/logrus"
	"sync"
)

type Room struct {
	name    string
	sockets map[*Socket]bool
	mu      sync.RWMutex
	manager *RoomManager
}
type RoomManager struct {
	rooms  map[string]*Room
	mu     sync.RWMutex
	logger *logrus.Logger
}

func NewRoomManager(logger *logrus.Logger) *RoomManager {
	if logger == nil {
		logger = logrus.New()
	}
	return &RoomManager{
		rooms:  make(map[string]*Room),
		logger: logger,
	}
}
func (rm *RoomManager) Room(name string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if room, exists := rm.rooms[name]; exists {
		return room
	}
	room := &Room{
		name:    name,
		sockets: make(map[*Socket]bool),
		manager: rm,
	}
	rm.rooms[name] = room
	return room
}
func (rm *RoomManager) GetRoom(name string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.rooms[name]
}
func (rm *RoomManager) DeleteRoom(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if room, exists := rm.rooms[name]; exists {
		room.RemoveAll()
		delete(rm.rooms, name)
	}
}
func (rm *RoomManager) Rooms() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	names := make([]string, 0, len(rm.rooms))
	for name := range rm.rooms {
		names = append(names, name)
	}
	return names
}
func (rm *RoomManager) GetAllSockets() []*Socket {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	socketMap := make(map[*Socket]bool)
	for _, room := range rm.rooms {
		room.mu.RLock()
		for socket := range room.sockets {
			socketMap[socket] = true
		}
		room.mu.RUnlock()
	}
	sockets := make([]*Socket, 0, len(socketMap))
	for socket := range socketMap {
		sockets = append(sockets, socket)
	}
	return sockets
}
func (rm *RoomManager) GetSocketByID(id string) *Socket {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	for _, room := range rm.rooms {
		room.mu.RLock()
		for socket := range room.sockets {
			if socket.ID() == id {
				room.mu.RUnlock()
				return socket
			}
		}
		room.mu.RUnlock()
	}
	return nil
}
func (r *Room) Name() string {
	return r.name
}
func (r *Room) addSocket(socket *Socket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sockets[socket] = true
	if r.manager.logger != nil {
		r.manager.logger.WithFields(logrus.Fields{
			"room":     r.name,
			"socketId": socket.ID(),
		}).Debug("Socket joined room")
	}
}
func (r *Room) removeSocket(socket *Socket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sockets, socket)
	if r.manager.logger != nil {
		r.manager.logger.WithFields(logrus.Fields{
			"room":     r.name,
			"socketId": socket.ID(),
		}).Debug("Socket left room")
	}
}
func (r *Room) Join(socket *Socket) {
	socket.roomsMx.Lock()
	defer socket.roomsMx.Unlock()
	socket.rooms[r.name] = r
	r.addSocket(socket)
}
func (r *Room) Leave(socket *Socket) {
	socket.roomsMx.Lock()
	defer socket.roomsMx.Unlock()
	delete(socket.rooms, r.name)
	r.removeSocket(socket)
}
func (r *Room) RemoveAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sockets = make(map[*Socket]bool)
}
func (r *Room) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sockets)
}
func (r *Room) Has(socket *Socket) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sockets[socket]
}
func (r *Room) Sockets() []*Socket {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sockets := make([]*Socket, 0, len(r.sockets))
	for socket := range r.sockets {
		sockets = append(sockets, socket)
	}
	return sockets
}
func (r *Room) Emit(event string, data any, marshaller func(*OutboundMessage) ([]byte, error), messageType MessageType, exclude ...*Socket) int {
	if marshaller == nil {
		if r.manager.logger != nil {
			r.manager.logger.Error("No marshaller provided for room emit")
		}
		return 0
	}
	msgBuf, err := marshaller(&OutboundMessage{Data: data})
	if err != nil {
		if r.manager.logger != nil {
			r.manager.logger.WithError(err).Error("Failed to marshal message for room emit")
		}
		return 0
	}
	sockets := r.Sockets()
	excludeMap := make(map[*Socket]bool, len(exclude))
	for _, s := range exclude {
		excludeMap[s] = true
	}
	sent := 0
	for _, socket := range sockets {
		if excludeMap[socket] {
			continue
		}
		if err := socket.Send(messageType, msgBuf); err != nil {
			if r.manager.logger != nil {
				r.manager.logger.WithError(err).WithField("socketId", socket.ID()).Warn("Failed to send to socket in room")
			}
			continue
		}
		sent++
	}
	return sent
}
func (r *Room) Broadcast(data []byte, messageType MessageType, exclude ...*Socket) int {
	sockets := r.Sockets()
	excludeMap := make(map[*Socket]bool, len(exclude))
	for _, s := range exclude {
		excludeMap[s] = true
	}
	sent := 0
	for _, socket := range sockets {
		if excludeMap[socket] {
			continue
		}
		if err := socket.Send(messageType, data); err != nil {
			if r.manager.logger != nil {
				r.manager.logger.WithError(err).WithField("socketId", socket.ID()).Warn("Failed to send to socket in room")
			}
			continue
		}
		sent++
	}
	return sent
}
func (c *Context) Join(roomName string) {
	if c.socket == nil {
		return
	}
	c.socket.Join(roomName)
}
func (c *Context) Leave(roomName string) {
	if c.socket == nil {
		return
	}
	c.socket.Leave(roomName)
}
func (c *Context) Room(roomName string) *Room {
	if c.socket == nil || c.socket.roomManager == nil {
		return nil
	}
	return c.socket.roomManager.GetRoom(roomName)
}
func (c *Context) To(roomName string) *RoomEmitter {
	emitter := &RoomEmitter{
		ctx:         c,
		exclude:     []*Socket{},
		messageType: c.messageType,
	}
	if c.socket != nil {
		emitter.exclude = []*Socket{c.socket}
		if c.socket.roomManager != nil {
			emitter.room = c.socket.roomManager.GetRoom(roomName)
		}
	}
	return emitter
}

type RoomEmitter struct {
	room        *Room
	rooms       []string
	ctx         *Context
	exclude     []*Socket
	messageType MessageType
}

func (re *RoomEmitter) Except(sockets ...*Socket) *RoomEmitter {
	re.exclude = append(re.exclude, sockets...)
	return re
}
func (re *RoomEmitter) Emit(data any) int {
	if re.ctx == nil {
		return 0
	}
	if len(re.rooms) > 0 {
		return re.emitToMultipleRooms(data)
	}
	if re.room == nil {
		return 0
	}
	return re.room.Emit("", data, re.ctx.messageMarshaller, re.messageType, re.exclude...)
}
func (re *RoomEmitter) emitToMultipleRooms(data any) int {
	if re.ctx.socket == nil || re.ctx.socket.roomManager == nil {
		return 0
	}
	socketMap := make(map[*Socket]bool)
	for _, roomName := range re.rooms {
		room := re.ctx.socket.roomManager.GetRoom(roomName)
		if room != nil {
			room.mu.RLock()
			for socket := range room.sockets {
				socketMap[socket] = true
			}
			room.mu.RUnlock()
		}
	}
	excludeMap := make(map[*Socket]bool)
	for _, socket := range re.exclude {
		excludeMap[socket] = true
	}
	msgBytes := re.ctx.mustMarshal(data)
	sent := 0
	for socket := range socketMap {
		if excludeMap[socket] {
			continue
		}
		if err := socket.Send(re.messageType, msgBytes); err == nil {
			sent++
		}
	}
	return sent
}
func (c *Context) Broadcast(data any) int {
	if c.socket == nil || c.socket.roomManager == nil {
		return 0
	}
	allSockets := c.socket.roomManager.GetAllSockets()
	sent := 0
	for _, socket := range allSockets {
		if err := socket.Send(c.messageType, c.mustMarshal(data)); err == nil {
			sent++
		}
	}
	return sent
}
func (c *Context) BroadcastExceptMe(data any) int {
	if c.socket == nil || c.socket.roomManager == nil {
		return 0
	}
	allSockets := c.socket.roomManager.GetAllSockets()
	sent := 0
	msgBytes := c.mustMarshal(data)
	for _, socket := range allSockets {
		if socket == c.socket {
			continue
		}
		if err := socket.Send(c.messageType, msgBytes); err == nil {
			sent++
		}
	}
	return sent
}
func (c *Context) EmitTo(socketID string, data any) error {
	if c.socket == nil || c.socket.roomManager == nil {
		return nil
	}
	targetSocket := c.socket.roomManager.GetSocketByID(socketID)
	if targetSocket == nil {
		return nil
	}
	msgBytes := c.mustMarshal(data)
	return targetSocket.Send(c.messageType, msgBytes)
}
func (c *Context) ToRooms(roomNames ...string) *RoomEmitter {
	emitter := &RoomEmitter{
		ctx:         c,
		rooms:       []string{},
		exclude:     []*Socket{},
		messageType: c.messageType,
	}
	if c.socket != nil {
		emitter.exclude = []*Socket{c.socket}
		emitter.rooms = roomNames
	}
	return emitter
}
func (c *Context) mustMarshal(data any) []byte {
	msg := &OutboundMessage{Data: data}
	bytes, err := c.messageMarshaller(msg)
	if err != nil {
		return []byte{}
	}
	return bytes
}
func (s *Server) Rooms() *RoomManager {
	return s.roomManager
}
