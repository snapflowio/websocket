package json

import (
	"encoding/json"
	"errors"
	websocket "github.com/snapflowio/websocket"
)

func Middleware() func(ctx *websocket.Context) {
	return func(ctx *websocket.Context) {
		headers := ctx.Headers()
		secWebSocketProtocol := headers.Get("Sec-WebSocket-Protocol")
		if secWebSocketProtocol != "" && secWebSocketProtocol != "json" {
			ctx.Error = errors.New("Unsupported WebSocket Subprotocol: " + secWebSocketProtocol)
			return
		}
		var messageData struct {
			ID    string          `json:"id"`
			Event string          `json:"event"`
			Meta  map[string]any  `json:"meta"`
			Data  json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(ctx.RawData(), &messageData); err != nil {
			if secWebSocketProtocol == "" {
				ctx.Next()
				return
			}
			ctx.Error = err
			return
		}
		if messageData.ID != "" {
			ctx.SetMessageID(messageData.ID)
		}
		if messageData.Event != "" {
			ctx.SetMessageEvent(messageData.Event)
		}
		if messageData.Meta != nil {
			ctx.SetMessageMeta(messageData.Meta)
		}
		if messageData.Data != nil {
			ctx.SetMessageData(messageData.Data)
		}
		ctx.SetMessageUnmarshaler(func(message *websocket.InboundMessage, into any) error {
			return json.Unmarshal(message.Data, into)
		})
		ctx.SetMessageMarshaller(func(message *websocket.OutboundMessage) ([]byte, error) {
			switch v := message.Data.(type) {
			case []FieldError:
				message.Data = M{
					"error":  "Validation error",
					"fields": genFieldsField(v),
				}
			case FieldError:
				message.Data = M{
					"error":  "Validation error",
					"fields": genFieldsField([]FieldError{v}),
				}
			case Error:
				message.Data = M{"error": string(v)}
			case string:
				message.Data = M{"message": v}
			}
			envelope := map[string]any{}
			if message.ID != "" {
				envelope["id"] = message.ID
			}
			if message.Data != nil {
				envelope["data"] = message.Data
			}
			return json.Marshal(envelope)
		})
		ctx.Next()
	}
}
