package slogx

import (
	"context"
	"log/slog"
)

const KeyBadCtx = "!BADCTX"

// CtxHandler is an slog.Handler provided for use through ctx.
// It optionally reports !BADCTX if there is no handler in ctx.
type CtxHandler struct {
	ignoreBADCTX bool
	handler      slog.Handler
}

// CtxHandlerOption are options for a CtxHandler.
type ctxHandlerOption func(*CtxHandler)

// NewCtxHandler creates a CtxHandler from fallback, using the given options.
// If opts is nil, the default options are used.
func NewCtxHandler(fallback slog.Handler, opts ...ctxHandlerOption) *CtxHandler {
	ctxHandler := &CtxHandler{
		handler: fallback,
	}
	for _, opt := range opts {
		opt(ctxHandler)
	}
	return ctxHandler
}

// Enabled works as (slog.Handler).Enabled.
// It uses handler returned by FromContext if exists or fallback handler.
func (h *CtxHandler) Enabled(ctx context.Context, l slog.Level) bool {
	handler := FromContext(ctx)
	if handler != nil {
		return handler.Enabled(ctx, l)
	}
	return h.handler.Enabled(ctx, l)
}

// Handle works as (slog.Handler).Handler.
// It uses handler returned by FromContext if exists or fallback handler.
// Optionally add !BADCTX attr if FromContext returns nil.
func (h *CtxHandler) Handle(ctx context.Context, r slog.Record) error {
	handler := FromContext(ctx)
	if handler == nil {
		handler = h.handler
		if !h.ignoreBADCTX {
			handler = handler.WithAttrs([]slog.Attr{{Key: KeyBadCtx, Value: slog.StringValue("missing handler")}})
		}
	}
	// TODO realisation.
	return nil
}

// WithAttrs works exactly like (slog.Handler).WithAttrs.
func (h *CtxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewCtxHandler(h.handler.WithAttrs(attrs))
}

// WithGroup works exactly like (slog.Handler).WithGroup.
func (h *CtxHandler) WithGroup(name string) slog.Handler {
	return NewCtxHandler(h.handler.WithGroup(name))
}

// LaxCtxHandler is an option for disable adding !BADCTX attr.
func LaxCtxHandler() ctxHandlerOption {
	return func(ctxHandler *CtxHandler) {
		ctxHandler.ignoreBADCTX = true
	}
}
