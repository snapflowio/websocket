package websocket

type Handler interface {
	Handle(ctx *Context)
}
type OpenHandler interface {
	HandleOpen(ctx *Context)
}
type CloseHandler interface {
	HandleClose(ctx *Context)
}
type HandlerFunc func(ctx *Context)
type BindType int

const (
	NormalBindType BindType = iota
	OpenBindType
	CloseBindType
)

type HandlerNode struct {
	BindType BindType
	Pattern  *Pattern
	Handlers []any
	Next     *HandlerNode
}

func (n *HandlerNode) tryMatch(ctx *Context) bool {
	if n.Pattern == nil {
		return true
	}
	return n.Pattern.Match(ctx.Event())
}
