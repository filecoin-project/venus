package logging

import (
	"context"

	logging "github.com/ipfs/go-log"
	"go.uber.org/zap"
)

type contextKey string

var ctxKey contextKey = "logger"

type (
	EventLogger  = logging.ZapEventLogger
	TaggedLogger = zap.SugaredLogger
)

var New = logging.Logger

func ContextWithLogger(parent context.Context, l *TaggedLogger) context.Context {
	return context.WithValue(parent, ctxKey, l)
}

func LoggerFromContext(ctx context.Context, fallback *EventLogger) *TaggedLogger {
	val := ctx.Value(ctxKey)
	if val != nil {
		l, ok := val.(*TaggedLogger)
		if ok && l != nil {
			return l
		}
	}

	return &fallback.SugaredLogger
}
