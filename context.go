package slogx

import (
	"context"
	"log/slog"
)

type contextKey int

const contextKeyLog contextKey = 0

// NewContext returns a new Context that carries value log.
func NewContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKeyLog, log)
}

// FromContext returns the Logger value stored in ctx if exists or nil.
func FromContext(ctx context.Context) *slog.Logger {
	log, _ := ctx.Value(contextKeyLog).(*slog.Logger)
	return log
}
