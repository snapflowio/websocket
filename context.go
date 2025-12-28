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

const DefaultRequestTimeout = 5 * time.Second

var ErrContextFreed = errors.New("context cannot be used after handler returns - handlers must block until all operations complete")

type Context struct {
	parentContext             *Context
	socket                    *Socket
	message                   *InboundMessage
	messageType               websocket.MessageType
	messageUnmarshaler        func(message *InboundMessage, into any) error
	messageMarshaller         func(message *OutboundMessage) ([]byte, error)
	currentHandlerNode        *HandlerNode
	currentHandlerNodeMatches bool
	associatedValues          map[string]any
	currentHandlerIndex       int
	currentHandler            any
	ctx                       context.Context
	cancelCtx                 context.CancelFunc
	Error                     error
	ErrorStack                string
}

var _ context.Context = &Context{}

func NewContext(sender *Socket, message *InboundMessage, handlers ...any) *Context {
	return NewContextWithNode(sender, message, &HandlerNode{Handlers: handlers})
}

func NewContextWithNode(socket *Socket, message *InboundMessage, firstHandlerNode *HandlerNode) *Context {
	return NewContextWithNodeAndMessageType(socket, message, firstHandlerNode, websocket.MessageText)
}

func NewContextWithNodeAndMessageType(socket *Socket, message *InboundMessage, firstHandlerNode *HandlerNode, messageType websocket.MessageType) *Context {
	ctx := contextFromPool()
	ctx.ctx, ctx.cancelCtx = context.WithCancel(socket)
	ctx.socket = socket
	ctx.message = message
	ctx.messageType = messageType
	if message.ID == "" {
		message.ID = uuid.NewString()
	}

	ctx.currentHandlerNode = firstHandlerNode
	return ctx
}

func NewSubContextWithNode(ctx *Context, firstHandlerNode *HandlerNode) *Context {
	subCtx := contextFromPool()
	subCtx.ctx, subCtx.cancelCtx = context.WithCancel(ctx)
	subCtx.parentContext = ctx
	subCtx.socket = ctx.socket
	subMsg := inboundMessageFromPool()
	subMsg.hasSetID = ctx.message.hasSetID
	subMsg.hasSetEvent = ctx.message.hasSetEvent
	subMsg.ID = ctx.message.ID
	subMsg.Event = ctx.message.Event
	subMsg.RawData = ctx.message.RawData
	subMsg.Data = ctx.message.Data
	subMsg.Meta = ctx.message.Meta
	subCtx.message = subMsg
	subCtx.messageType = ctx.messageType
	subCtx.Error = ctx.Error
	subCtx.ErrorStack = ctx.ErrorStack
	subCtx.messageUnmarshaler = ctx.messageUnmarshaler
	subCtx.messageMarshaller = ctx.messageMarshaller
	for k, v := range ctx.associatedValues {
		subCtx.associatedValues[k] = v
	}
	subCtx.currentHandlerNode = firstHandlerNode
	return subCtx
}

var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			associatedValues: map[string]any{},
		}
	},
}

func contextFromPool() *Context {
	return contextPool.Get().(*Context)
}

func (c *Context) free() {
	if c.message != nil {
		c.message.free()
	}

	c.cancelCtx()
	c.parentContext = nil
	c.socket = nil
	c.message = nil
	c.messageType = 0
	c.Error = nil
	c.ErrorStack = ""
	c.messageUnmarshaler = nil
	c.messageMarshaller = nil
	c.currentHandlerNode = nil
	c.currentHandlerNodeMatches = false
	c.currentHandlerIndex = 0
	c.currentHandler = nil

	for k := range c.associatedValues {
		delete(c.associatedValues, k)
	}

	contextPool.Put(c)
}

func (c *Context) tryUpdateParent() {
	if c.parentContext == nil {
		return
	}

	c.parentContext.Error = c.Error
	c.parentContext.ErrorStack = c.ErrorStack

	for k, v := range c.associatedValues {
		c.parentContext.associatedValues[k] = v
	}
}

func (c *Context) SetOnSocket(key string, value any) {
	if c.socket == nil {
		return
	}

	c.socket.Set(key, value)
}

func (c *Context) GetFromSocket(key string) (any, bool) {
	if c.socket == nil {
		return nil, false
	}

	return c.socket.Get(key)
}

func (c *Context) MustGetFromSocket(key string) any {
	if c.socket == nil {
		panic(ErrContextFreed)
	}

	return c.socket.MustGet(key)
}

func (c *Context) DeleteFromSocket(key string) {
	if c.socket == nil {
		return
	}

	c.socket.Delete(key)
}

func (c *Context) Set(key string, value any) {
	c.associatedValues[key] = value
}

func (c *Context) Get(key string) (any, bool) {
	v, ok := c.associatedValues[key]
	return v, ok
}

func (c *Context) MustGet(key string) any {
	v, ok := c.associatedValues[key]
	if !ok {
		panic("key not found")
	}

	return v
}

func (c *Context) Delete(key string) {
	delete(c.associatedValues, key)
}

func (c *Context) SocketID() string {
	if c.socket == nil {
		return ""
	}

	return c.socket.id
}

func (c *Context) ConnectionInfo() *ConnectionInfo {
	if c.socket == nil {
		return nil
	}

	return c.socket.ConnectionInfo()
}

func (c *Context) MessageID() string {
	return c.message.ID
}

func (c *Context) RawData() []byte {
	return c.message.RawData
}

func (c *Context) Data() []byte {
	return c.message.Data
}

func (c *Context) Event() string {
	return c.message.Event
}

func (c *Context) MessageType() MessageType {
	return MessageType(c.messageType)
}

func (c *Context) Headers() http.Header {
	if c.socket == nil {
		return nil
	}

	return c.socket.Headers()
}

func (c *Context) RemoteAddr() string {
	if c.socket == nil {
		return ""
	}

	return c.socket.RemoteAddr()
}

func (c *Context) Unmarshal(into any) error {
	return c.unmarshalInboundMessage(c.message, into)
}

func (c *Context) SetMessageUnmarshaler(unmarshaler func(message *InboundMessage, into any) error) {
	c.messageUnmarshaler = unmarshaler
}

func (c *Context) SetMessageMarshaller(marshaller func(message *OutboundMessage) ([]byte, error)) {
	c.messageMarshaller = marshaller
}

func (c *Context) SetMessageID(id string) {
	c.message.ID = id
	c.message.hasSetID = true
}

func (c *Context) SetMessageEvent(event string) {
	c.message.Event = event
	c.message.hasSetEvent = true
}

func (c *Context) SetMessageRawData(rawData []byte) {
	c.message.RawData = rawData
}

func (c *Context) SetMessageData(data []byte) {
	c.message.Data = data
}

func (c *Context) SetMessageMeta(meta map[string]any) {
	c.message.Meta = meta
}

func (c *Context) Meta(key string) (any, bool) {
	if c.message == nil || c.message.Meta == nil {
		return nil, false
	}

	value, ok := c.message.Meta[key]
	return value, ok
}

func (c *Context) Send(data any) error {
	if c.socket == nil {
		return ErrContextFreed
	}

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		Data: data,
	})

	if err != nil {
		return err
	}

	return c.socket.Send(c.messageType, msgBuf)
}

func (c *Context) SendEvent(event string, data any) error {
	if c.socket == nil {
		return ErrContextFreed
	}

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		Event: event,
		Data:  data,
	})

	if err != nil {
		return err
	}

	return c.socket.Send(c.messageType, msgBuf)
}

func (c *Context) Reply(data any) error {
	if c.socket == nil {
		return ErrContextFreed
	}

	if c.MessageID() == "" {
		return errors.New("cannot reply to a message without an ID")
	}

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:   c.MessageID(),
		Data: data,
	})

	if err != nil {
		return err
	}

	return c.socket.Send(c.messageType, msgBuf)
}

func (c *Context) ReplyEvent(event string, data any) error {
	if c.socket == nil {
		return ErrContextFreed
	}

	if c.MessageID() == "" {
		return errors.New("cannot reply to a message without an ID")
	}

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:    c.MessageID(),
		Event: event,
		Data:  data,
	})

	if err != nil {
		return err
	}

	return c.socket.Send(c.messageType, msgBuf)
}

func (c *Context) Request(data any) (any, error) {
	return c.RequestWithTimeout(data, DefaultRequestTimeout)
}

func (c *Context) RequestWithTimeout(data any, timeout time.Duration) (any, error) {
	if c.socket == nil {
		return nil, ErrContextFreed
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.RequestWithContext(ctx, data)
}
func (c *Context) RequestWithContext(ctx context.Context, data any) (any, error) {
	if c.socket == nil {
		return nil, ErrContextFreed
	}

	id := uuid.NewString()
	responseMessageChan := make(chan *InboundMessage, 1)
	defer close(responseMessageChan)
	c.socket.AddInterceptor(id, responseMessageChan)
	defer c.socket.RemoveInterceptor(id)
	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:   id,
		Data: data,
	})

	if err != nil {
		return nil, err
	}

	if err := c.socket.Send(c.messageType, msgBuf); err != nil {
		return nil, err
	}

	select {
	case responseMessage := <-responseMessageChan:
		data := responseMessage.Data
		responseMessage.free()
		return data, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
	}
}

func (c *Context) RequestInto(data any, into any) error {
	return c.RequestIntoWithTimeout(data, into, DefaultRequestTimeout)
}

func (c *Context) RequestIntoWithTimeout(data any, into any, timeout time.Duration) error {
	if c.socket == nil {
		return ErrContextFreed
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.RequestIntoWithContext(ctx, data, into)
}
func (c *Context) RequestIntoWithContext(ctx context.Context, data any, into any) error {
	if c.socket == nil {
		return ErrContextFreed
	}

	id := uuid.NewString()
	responseMessageChan := make(chan *InboundMessage, 1)
	defer close(responseMessageChan)
	c.socket.AddInterceptor(id, responseMessageChan)
	defer c.socket.RemoveInterceptor(id)

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:   id,
		Data: data,
	})

	if err != nil {
		return err
	}

	if err := c.socket.Send(c.messageType, msgBuf); err != nil {
		return err
	}

	select {
	case responseMessage := <-responseMessageChan:
		err := c.unmarshalInboundMessage(responseMessage, into)
		responseMessage.free()
		return err
	case <-ctx.Done():
		return fmt.Errorf("request cancelled: %w", ctx.Err())
	}
}

func (c *Context) Close() {
	if c.socket == nil {
		return
	}

	c.socket.Close(StatusNormalClosure, "", ServerCloseSource)
}
func (c *Context) CloseWithStatus(status Status, reason string) {
	if c.socket == nil {
		return
	}

	c.socket.Close(status, reason, ServerCloseSource)
}

func (c *Context) CloseStatus() (Status, string, CloseSource) {
	if c.socket == nil {
		return 0, "", 0
	}

	c.socket.closeMu.Lock()
	defer c.socket.closeMu.Unlock()
	return c.socket.closeStatus, c.socket.closeReason, c.socket.closeStatusSource
}

func (c *Context) unmarshalInboundMessage(message *InboundMessage, into any) error {
	if c.messageUnmarshaler == nil {
		return errors.New("no message unmarshaller set. use SetMessageUnmarshaler or add message parser middleware")
	}

	return c.messageUnmarshaler(message, into)
}

func (c *Context) marshallOutboundMessage(message *OutboundMessage) ([]byte, error) {
	if c.messageMarshaller == nil {
		return nil, errors.New("no message marshaller set. use SetMessageMarshaller() or add data encoder middleware")
	}

	return c.messageMarshaller(message)
}

func (c *Context) Deadline() (time.Time, bool) {
	return c.ctx.Deadline()
}

func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *Context) Err() error {
	return c.ctx.Err()
}

func (c *Context) Value(v any) any {
	return c.ctx.Value(v)
}
