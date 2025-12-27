package websocket

import "sync"

type InboundMessage struct {
	hasSetID    bool
	hasSetEvent bool
	ID          string
	Event       string
	RawData     []byte
	Data        []byte
	Meta        map[string]any
}

var inboundMessagePool = sync.Pool{
	New: func() any {
		return &InboundMessage{}
	},
}

func inboundMessageFromPool() *InboundMessage {
	msg := inboundMessagePool.Get().(*InboundMessage)
	msg.hasSetID = false
	msg.hasSetEvent = false
	msg.ID = ""
	msg.Event = ""
	msg.RawData = nil
	msg.Data = nil
	msg.Meta = nil
	return msg
}
func (m *InboundMessage) free() {
	inboundMessagePool.Put(m)
}

type OutboundMessage struct {
	ID   string
	Data any
}
