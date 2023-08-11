package slogx

import (
	"context"
	"log/slog"
)

// CtxHandler is used as a default logger.
// It applies for applications only (not for libraries).
//
// Usually we used logger stored in ctx. We had to extract it first
// so it took us one extra line in every function:
//
//	log := structlog.FromContext(ctx, nil)
//	log.Info("some message",...)
//
// With CtxHandler we minimise it.
// In main we should set CtxHandler as a default for a global logger:
//
//	slogx.SetDefaultCtxHandler(fallback, opts)
//
// By convention we do not change Default logger after.
// Now we log anywhere in code with just one line:
//
//	slog.InfoContext(ctx, "some message",...))
//
// We recommend not to add attributes/groups on global logger.
// All Attributes and Groups you should apply on handler, stored in ctx:
//
//	handler = handler.WithGroup("g")
//	handler = handler.WithAttrs([]slog.Attr{slog.Int("key", 3)})
//	ctx = slogx.NewContext(ctx, handler)
//	slog.InfoContext(ctx, "message")
//
// Spawning a new logger using With or WithGrop will cause these settings
// to be applied after the settings of handler stored in ctx:
//
//	log := slog.With(slog.Int("top", 20))
//
//	handler = handler.WithAttrs([]slog.Attr{slog.Int("top", 10)})
//	ctx = slogx.NewContext(ctx, handler)
//	log.InfoContext(ctx, "list")
//
// Output:
//
//	... level=INFO msg=list top=20
//
// By convention such logger must not be carried by stack neither in ctx nor in parameters.
//
// CtxHandler optionally reports !BADCTX with ctx as a value if there is no handler in it.
type CtxHandler struct {
	ignoreBADCTX bool
	handler      slog.Handler
}

// CtxHandlerOption is an option for a CtxHandler.
type ctxHandlerOption func(*CtxHandler)

// SetDefaultCtxHandler sets a CtxHandler as a default logger.
// It applies given options. If opts is nil, the default options are used.
func SetDefaultCtxHandler(fallback slog.Handler, opts ...ctxHandlerOption) {
	panic("TODO")
}

// Enabled works as (slog.Handler).Enabled.
// It uses handler returned by FromContext or fallback handler.
func (h *CtxHandler) Enabled(ctx context.Context, l slog.Level) bool {
	panic("TODO")
}

// Handle works as (slog.Handler).Handler.
// It uses handler returned by FromContext or fallback handler.
// Optionally add !BADCTX attr if FromContext returns nil.
func (h *CtxHandler) Handle(ctx context.Context, r slog.Record) error {
	panic("TODO")
}

// WithAttrs works exactly like (slog.Handler).WithAttrs.
func (h *CtxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	panic("TODO")
}

// WithGroup works exactly like (slog.Handler).WithGroup.
func (h *CtxHandler) WithGroup(name string) slog.Handler {
	panic("TODO")
}

// LaxCtxHandler is an option for disable adding !BADCTX attr.
func LaxCtxHandler() ctxHandlerOption {
	panic("TODO")
}
