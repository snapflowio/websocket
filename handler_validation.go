package websocket

import "reflect"

func validateHandlers(handlers []any, allowedTypes ...string) error {
	if len(handlers) == 0 {
		return ErrNoHandlers
	}
	for _, handler := range handlers {
		if !isValidHandler(handler, allowedTypes...) {
			return &InvalidHandlerError{
				Expected: allowedTypes,
				Got:      reflect.TypeOf(handler).String(),
			}
		}
	}
	return nil
}

func isValidHandler(handler any, allowedTypes ...string) bool {
	for _, allowedType := range allowedTypes {
		switch allowedType {
		case "Handler":
			if _, ok := handler.(Handler); ok {
				return true
			}
		case "OpenHandler":
			if _, ok := handler.(OpenHandler); ok {
				return true
			}
		case "CloseHandler":
			if _, ok := handler.(CloseHandler); ok {
				return true
			}
		case "HandlerFunc":
			if _, ok := handler.(HandlerFunc); ok {
				return true
			}
		case "func(*Context)":
			if _, ok := handler.(func(*Context)); ok {
				return true
			}
		}
	}
	return false
}

func validateNormalHandlers(handlers []any) error {
	return validateHandlers(handlers, "Handler", "HandlerFunc", "func(*Context)")
}

func validateOpenHandlers(handlers []any) error {
	return validateHandlers(handlers, "OpenHandler", "HandlerFunc", "func(*Context)")
}

func validateCloseHandlers(handlers []any) error {
	return validateHandlers(handlers, "CloseHandler", "HandlerFunc", "func(*Context)")
}
