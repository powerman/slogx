package slogx

import (
	"context"
	"log/slog"
)

type contextKey int

const (
	contextKeyLog contextKey = iota
	contextKeyHandler
)

// NewContextWithHandler returns a new Context that carries value handler.
func NewContextWithHandler(ctx context.Context, handler slog.Handler) context.Context {
	return context.WithValue(ctx, contextKeyHandler, handler)
}

// HandlerFromContext returns a Handler value stored in ctx if exists or nil.
func HandlerFromContext(ctx context.Context) slog.Handler {
	handler, _ := ctx.Value(contextKeyHandler).(slog.Handler)
	return handler
}

// NewContextWithLogger returns a new Context that carries value log.
func NewContextWithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKeyLog, log)
}

// LoggerFromContext returns a Logger value stored in ctx if exists or nil.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	log, _ := ctx.Value(contextKeyLog).(*slog.Logger)
	return log
}
