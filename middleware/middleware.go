package middleware

import (
	"fmt"
	"runtime/debug"
	"time"

	websocket "github.com/snapflowio/websocket"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func Set[V any](key string, value V) func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		ctx.Set(key, value)
		ctx.Next()
	}
}

func SetFunc[V any](key string, fn func() V) func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		ctx.Set(key, fn())
		ctx.Next()
	}
}

func SetFromContext[V any](key string, fn func(*websocket.Context) V) func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		ctx.Set(key, fn(ctx))
		ctx.Next()
	}
}

func SocketSet[V any](key string, value V) func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		ctx.SetOnSocket(key, value)
		ctx.Next()
	}
}

func SocketSetFunc[V any](key string, fn func() V) func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		ctx.SetOnSocket(key, fn())
		ctx.Next()
	}
}

func SocketSetFromContext[V any](key string, fn func(*websocket.Context) V) func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		ctx.SetOnSocket(key, fn(ctx))
		ctx.Next()
	}
}

func Logger(logger *logrus.Logger) func(*websocket.Context) {
	if logger == nil {
		logger = logrus.New()
	}
	return func(ctx *websocket.Context) {
		start := time.Now()
		event := ctx.Event()
		socketID := ctx.SocketID()

		ctx.Next()

		duration := time.Since(start)
		logger.WithFields(logrus.Fields{
			"event":    event,
			"socketId": socketID,
			"duration": duration,
		}).Debug("Event processed")
	}
}

func Recovery(logger *logrus.Logger) func(*websocket.Context) {
	if logger == nil {
		logger = logrus.New()
	}
	return func(ctx *websocket.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.WithFields(logrus.Fields{
					"panic":   fmt.Sprintf("%v", r),
					"stack":   stack,
					"event":   ctx.Event(),
					"socketId": ctx.SocketID(),
				}).Error("Panic recovered in handler")

				ctx.Error = fmt.Errorf("internal server error")
			}
		}()
		ctx.Next()
	}
}

func RequestID() func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		requestID := uuid.New().String()
		ctx.Set("requestID", requestID)
		ctx.Next()
	}
}

func Timeout(duration time.Duration) func(*websocket.Context) {
	return func(ctx *websocket.Context) {
		done := make(chan struct{})
		go func() {
			ctx.Next()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(duration):
			ctx.Error = fmt.Errorf("handler timeout after %v", duration)
		}
	}
}

func CORS(allowedOrigins []string) func(*websocket.Context) {
	originMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		originMap[origin] = true
	}

	return func(ctx *websocket.Context) {
		connInfo := ctx.ConnectionInfo()
		if connInfo != nil {
			origin := connInfo.Headers.Get("Origin")
			if len(originMap) > 0 && !originMap[origin] && !originMap["*"] {
				ctx.Error = fmt.Errorf("origin not allowed: %s", origin)
				return
			}
		}
		ctx.Next()
	}
}

func RateLimit(maxRequests int, window time.Duration) func(*websocket.Context) {
	type rateLimitData struct {
		count     int
		resetTime time.Time
	}

	return func(ctx *websocket.Context) {
		socketID := ctx.SocketID()
		key := "ratelimit:" + socketID

		var data rateLimitData
		if val, ok := ctx.GetFromSocket(key); ok {
			data = val.(rateLimitData)
		}

		now := time.Now()
		if now.After(data.resetTime) {
			data.count = 0
			data.resetTime = now.Add(window)
		}

		data.count++
		if data.count > maxRequests {
			ctx.Error = fmt.Errorf("rate limit exceeded")
			return
		}

		ctx.SetOnSocket(key, data)
		ctx.Next()
	}
}
