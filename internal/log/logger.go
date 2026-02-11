package log

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
)

type contextKey string



var CorrelatedIDKey contextKey = "correlation_id"
const LoggerKeyForContext contextKey = "logger"
type Logger struct {
	*slog.Logger
}

func NewLoggerWithJSONOutput() *Logger {
	return &Logger{
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (l *Logger) WithCorrelationID(ctx context.Context) *Logger {
	id := GetOrGenerateCorrelationID(ctx)

	return &Logger{
		Logger: l.Logger.With(string(CorrelatedIDKey), id),
	}
}

func GetOrGenerateCorrelationID(ctx context.Context) string {
	if id := ctx.Value(CorrelatedIDKey); id != nil {
		if s, ok := id.(string); ok {
			return s
		}
	}

	id := GenerateCorrelationID()

	return id
}

func GenerateCorrelationID() string {
	return uuid.New().String()
}





func GetLoggerInstanceFromContext(ctx context.Context, fallbackLogger *Logger) *Logger {
	if ctx != nil {
		if logger := ctx.Value(LoggerKeyForContext); logger != nil {
			if l, ok := logger.(*Logger); ok {
				return l
			}
		}


		if fallbackLogger != nil {
			return fallbackLogger.WithCorrelationID(ctx)
		}
		return NewLoggerWithJSONOutput().WithCorrelationID(ctx)
	}


	if fallbackLogger != nil {
		return fallbackLogger
	}

	return NewLoggerWithJSONOutput()
}
