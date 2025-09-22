package slogx

import (
	"context"
	"log/slog"
)

type contextKey int

const (
	contextKeyHandler contextKey = iota
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
