package websocket

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
)

func (c *Context) Next() {
	defer c.tryUpdateParent()
	isCloseHandler := c.currentHandlerNode != nil && c.currentHandlerNode.BindType == CloseBindType
	if c.Error != nil || (!isCloseHandler && c.socket.IsClosed()) {
		return
	}
	if c.message.hasSetID {
		interceptorChan, ok := c.socket.GetInterceptor(c.message.ID)
		if ok {
			interceptorChan <- c.message
			return
		}
		c.message.hasSetID = false
	}
	if c.message.hasSetEvent {
		if c.currentHandlerNodeMatches && !c.currentHandlerNode.tryMatch(c) {
			c.currentHandlerNode = c.currentHandlerNode.Next
			c.currentHandlerNodeMatches = false
			c.currentHandlerIndex = 0
			c.currentHandler = nil
		}
		c.message.hasSetEvent = false
	}
	for c.currentHandlerNode != nil {
		if !c.currentHandlerNodeMatches {
			for c.currentHandlerNode != nil {
				if c.currentHandlerNode.tryMatch(c) {
					c.currentHandlerNodeMatches = true
					break
				}
				c.currentHandlerNode = c.currentHandlerNode.Next
			}
			if !c.currentHandlerNodeMatches {
				break
			}
		}
		if c.currentHandlerIndex < len(c.currentHandlerNode.Handlers) {
			c.currentHandler = c.currentHandlerNode.Handlers[c.currentHandlerIndex]
			c.currentHandlerIndex++
			break
		}
		c.currentHandlerNode = c.currentHandlerNode.Next
		c.currentHandlerNodeMatches = false
		c.currentHandlerIndex = 0
		c.currentHandler = nil
	}
	if c.currentHandler == nil {
		return
	}
	bindType := c.currentHandlerNode.BindType
	if currentHandler, ok := c.currentHandler.(OpenHandler); ok && bindType == OpenBindType {
		execWithCtxRecovery(c, func() {
			currentHandler.HandleOpen(c)
		})
	} else if currentHandler, ok := c.currentHandler.(CloseHandler); ok && bindType == CloseBindType {
		execWithCtxRecovery(c, func() {
			currentHandler.HandleClose(c)
		})
	} else if currentHandler, ok := c.currentHandler.(Handler); ok && bindType == NormalBindType {
		execWithCtxRecovery(c, func() {
			currentHandler.Handle(c)
		})
	} else if currentHandler, ok := c.currentHandler.(HandlerFunc); ok {
		execWithCtxRecovery(c, func() {
			currentHandler(c)
		})
	} else if currentHandler, ok := c.currentHandler.(func(*Context)); ok {
		execWithCtxRecovery(c, func() {
			currentHandler(c)
		})
	} else {
		panic(fmt.Sprintf("Unknown handler type: %s", reflect.TypeOf(c.currentHandler)))
	}
	if (bindType == OpenBindType && !c.socket.IsClosed()) || bindType == CloseBindType {
		c.Next()
	}
	c.currentHandlerNode = nil
	c.currentHandlerNodeMatches = false
	c.currentHandlerIndex = 0
	c.currentHandler = nil
}
func execWithCtxRecovery(ctx *Context, fn func()) {
	defer func() {
		if maybeErr := recover(); maybeErr != nil {
			if err, ok := maybeErr.(error); ok {
				ctx.Error = err
			} else {
				ctx.Error = fmt.Errorf("%s", maybeErr)
			}
			stack := string(debug.Stack())
			stackLines := strings.Split(stack, "\n")
			ctx.ErrorStack = strings.Join(stackLines[6:], "\n")
		}
	}()
	fn()
}
