package websocket

import (
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"
)

type Server struct {
	firstHandlerNode      *HandlerNode
	lastHandlerNode       *HandlerNode
	firstOpenHandlerNode  *HandlerNode
	lastOpenHandlerNode   *HandlerNode
	firstCloseHandlerNode *HandlerNode
	lastCloseHandlerNode  *HandlerNode
	origins               []string
	logger                *logrus.Logger
	roomManager           *RoomManager
}

var _ http.Handler = &Server{}
var _ Handler = &Server{}
var _ OpenHandler = &Server{}
var _ CloseHandler = &Server{}

func NewServer() *Server {
	logger := logrus.New()
	return &Server{
		logger:      logger,
		roomManager: NewRoomManager(logger),
	}
}

func (s *Server) SetLogger(logger *logrus.Logger) {
	s.logger = logger
	if s.roomManager != nil {
		s.roomManager.logger = logger
	}
}

func (s *Server) SetOrigins(origins []string) {
	s.origins = origins
}

func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if s.isWebsocketUpgradeRequest(req) {
		s.handleWebsocketConnection(res, req)
		return
	}

	res.WriteHeader(400)

	if _, err := res.Write([]byte("Bad Request. Expected websocket upgrade request")); err != nil {
		s.logger.WithError(err).Error("failed to write error response")
	}
}

type ConnectionInfo struct {
	RemoteAddr string
	Headers    http.Header
	Query      map[string]string // Query parameters from the connection URL
}

func (s *Server) HandleConnection(info *ConnectionInfo, connection SocketConnection) {
	socket := NewSocket(info, connection)
	socket.SetRoomManager(s.roomManager)
	socket.HandleOpen(s.firstOpenHandlerNode)
	for socket.HandleNextMessageWithNode(s.firstHandlerNode) {
	}

	socket.HandleClose(s.firstCloseHandlerNode)
	socket.leaveAllRooms()
	socket.closeMu.Lock()
	defer socket.closeMu.Unlock()
	if err := connection.Close(socket.closeStatus, socket.closeReason); err != nil {
		s.logger.WithError(err).Error("failed to close connection")
	}
}

func (s *Server) Handle(ctx *Context) {
	subCtx := NewSubContextWithNode(ctx, s.firstHandlerNode)
	subCtx.Next()
	subCtx.free()
	if subCtx.currentHandlerNode != nil {
		ctx.Next()
	}
}

func (s *Server) HandleOpen(ctx *Context) {
	subCtx := NewSubContextWithNode(ctx, s.firstOpenHandlerNode)
	subCtx.Next()
	subCtx.free()
	if subCtx.currentHandlerNode != nil {
		ctx.Next()
	}
}

func (s *Server) HandleClose(ctx *Context) {
	subCtx := NewSubContextWithNode(ctx, s.firstCloseHandlerNode)
	subCtx.Next()
	subCtx.free()
	if subCtx.currentHandlerNode != nil {
		ctx.Next()
	}
}

func (s *Server) Use(handlers ...any) error {
	event := WildcardDeep
	if len(handlers) != 0 {
		if customEvent, ok := handlers[0].(string); ok {
			if !strings.HasSuffix(customEvent, WildcardDeep) && !strings.HasSuffix(customEvent, WildcardSingle) {
				customEvent += "." + WildcardDeep
			}
			event = customEvent
			handlers = handlers[1:]
		}
	}

	return s.on(event, handlers...)
}

func (s *Server) UseOpen(handlers ...any) error {
	if err := validateOpenHandlers(handlers); err != nil {
		return err
	}

	nextHandlerNode := &HandlerNode{
		BindType: OpenBindType,
		Handlers: handlers,
	}

	if s.firstOpenHandlerNode == nil {
		s.firstOpenHandlerNode = nextHandlerNode
		s.lastOpenHandlerNode = nextHandlerNode
	} else {
		s.lastOpenHandlerNode.Next = nextHandlerNode
		s.lastOpenHandlerNode = nextHandlerNode
	}

	return nil
}

func (s *Server) UseClose(handlers ...any) error {
	if err := validateCloseHandlers(handlers); err != nil {
		return err
	}

	nextHandlerNode := &HandlerNode{
		BindType: CloseBindType,
		Handlers: handlers,
	}

	if s.firstCloseHandlerNode == nil {
		s.firstCloseHandlerNode = nextHandlerNode
		s.lastCloseHandlerNode = nextHandlerNode
	} else {
		s.lastCloseHandlerNode.Next = nextHandlerNode
		s.lastCloseHandlerNode = nextHandlerNode
	}
	return nil
}

func (s *Server) On(event string, handlers ...any) error {
	return s.on(event, handlers...)
}

func (s *Server) on(event string, handlers ...any) error {
	if err := validateNormalHandlers(handlers); err != nil {
		return err
	}

	pattern, err := NewPattern(event)
	if err != nil {
		return &InvalidPatternError{
			Pattern: event,
			Reason:  err,
		}
	}

	for _, handler := range handlers {
		if handlerWithOpen, ok := handler.(OpenHandler); ok {
			if err := s.UseOpen(handlerWithOpen); err != nil {
				return err
			}
		}
		if handlerWithClose, ok := handler.(CloseHandler); ok {
			if err := s.UseClose(handlerWithClose); err != nil {
				return err
			}
		}
	}

	nextHandlerNode := &HandlerNode{
		BindType: NormalBindType,
		Pattern:  pattern,
		Handlers: handlers,
	}

	if s.firstHandlerNode == nil {
		s.firstHandlerNode = nextHandlerNode
		s.lastHandlerNode = nextHandlerNode
	} else {
		s.lastHandlerNode.Next = nextHandlerNode
		s.lastHandlerNode = nextHandlerNode
	}

	return nil
}

func (s *Server) isWebsocketUpgradeRequest(req *http.Request) bool {
	return req.Header.Get("Upgrade") == "websocket"
}

func (s *Server) handleWebsocketConnection(res http.ResponseWriter, req *http.Request) {
	origins := s.origins
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	conn, err := websocket.Accept(res, req, &websocket.AcceptOptions{
		OriginPatterns: origins,
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to accept websocket connection")
		if conn != nil {
			if closeErr := conn.Close(websocket.StatusInternalError, "failed to accept websocket connection"); closeErr != nil {
				s.logger.WithError(closeErr).Error("failed to close connection after accept error")
			}
		}

		return
	}

	// Extract query parameters
	queryParams := make(map[string]string)
	for key, values := range req.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0] // Take first value if multiple
		}
	}

	info := &ConnectionInfo{
		RemoteAddr: req.RemoteAddr,
		Headers:    req.Header,
		Query:      queryParams,
	}

	socket := NewSocket(info, NewWebSocketConnection(conn))
	socket.SetRoomManager(s.roomManager)
	socket.HandleOpen(s.firstOpenHandlerNode)
	for socket.HandleNextMessageWithNode(s.firstHandlerNode) {
	}

	socket.HandleClose(s.firstCloseHandlerNode)
	socket.leaveAllRooms()
	socket.closeMu.Lock()
	closeStatus := socket.closeStatus
	closeReason := socket.closeReason
	socket.closeMu.Unlock()
	if err := conn.Close(closeStatus, closeReason); err != nil {
		s.logger.WithError(err).Error("failed to close websocket connection")
	}
}
