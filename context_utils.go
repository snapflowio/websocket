package websocket

func CtxSocket(ctx *Context) *Socket {
	return ctx.socket
}

func CtxFree(ctx *Context) {
	ctx.free()
}

func CtxMeta(ctx *Context) map[string]any {
	if ctx.message == nil {
		return nil
	}

	return ctx.message.Meta
}
