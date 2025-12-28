package websocket

import (
	"errors"
	"fmt"
)

var (
	ErrNoHandlers      = errors.New("no handlers provided")
	ErrInvalidPattern  = errors.New("invalid event pattern")
	ErrInvalidHandler  = errors.New("invalid handler type")
	ErrNoMarshaller    = errors.New("no message marshaller set")
	ErrNoUnmarshaller  = errors.New("no message unmarshaller set")
	ErrNoMessageID     = errors.New("cannot reply to message without ID")
	ErrInvalidRoomName = errors.New("invalid room name")
	ErrInvalidSocketID = errors.New("invalid socket ID")
	ErrSocketNotFound  = errors.New("socket not found")
	ErrRoomNotFound    = errors.New("room not found")
	ErrNoRoomManager   = errors.New("room manager not initialized")
)

type InvalidHandlerError struct {
	Expected []string
	Got      string
}

func (e *InvalidHandlerError) Error() string {
	return fmt.Sprintf("invalid handler type: expected one of %v, got %s", e.Expected, e.Got)
}

type InvalidPatternError struct {
	Pattern string
	Reason  error
}

func (e *InvalidPatternError) Error() string {
	return fmt.Sprintf("invalid event pattern %q: %v", e.Pattern, e.Reason)
}

func (e *InvalidPatternError) Unwrap() error {
	return e.Reason
}
