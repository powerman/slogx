package slogx

import (
	"context"
	"log/slog"
)

const KeyBadCtx = "!BADCTX"

// CtxHandler is used as a default logger.
// It applies for applications only (not for libraries).
//
// Usually we used logger stored in ctx. We had to extract it first.
// It took us one extra line in every function:
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
//	log.InfoContext(slogx.NewContext(ctx, handler), "list")
//
// Output:
//
//	... level=INFO msg=list top=20
//
// By convention such logger must not be carried by stack neither in ctx nor in parameters.
//
// CtxHandler optionally reports !BADCTX with ctx as a value if there is no handler in it.
type CtxHandler struct {
	handler      slog.Handler
	opList       []data
	ignoreBADCTX bool
}

// Data is used for store operations with logger.
// It may be group to add or attributes. It also keeps an order.
// Applying Handle method all data will be added to a handler at last.
type data struct {
	group string
	attrs []slog.Attr
}

// CtxHandlerOption is an option for a CtxHandler.
type ctxHandlerOption func(*CtxHandler)

// SetDefaultCtxHandler sets a CtxHandler as a default logger.
// It applies given options. If opts is nil, the default options are used.
func SetDefaultCtxHandler(fallback slog.Handler, opts ...ctxHandlerOption) {
	const size = 64 << 10
	ctxHandler := &CtxHandler{
		handler: fallback,
		opList:  make([]data, size),
	}
	for _, opt := range opts {
		opt(ctxHandler)
	}
	slog.SetDefault(slog.New(ctxHandler))
}

// Enabled works as (slog.Handler).Enabled.
// It useys handler returned by FromContext or fallback handler.
func (h *CtxHandler) Enabled(ctx context.Context, l slog.Level) bool {
	handler := FromContext(ctx)
	if handler == nil {
		handler = h.handler
	}
	return handler.Enabled(ctx, l)
}

// Handle works as (slog.Handler).Handler.
// It uses handler returned by FromContext or fallback handler.
// Optionally add !BADCTX attr if FromContext returns nil.
func (h *CtxHandler) Handle(ctx context.Context, r slog.Record) error {
	handler := FromContext(ctx)
	if handler == nil {
		handler = h.handler
		if !h.ignoreBADCTX {
			handler = handler.WithAttrs([]slog.Attr{slog.Any(KeyBadCtx, ctx)})
		}
	}
	for _, op := range h.opList {
		if len(op.group) > 0 {
			handler = handler.WithGroup(op.group)
		} else {
			handler = handler.WithAttrs(op.attrs)
		}
	}
	return handler.Handle(ctx, r)
}

// WithAttrs works exactly like (slog.Handler).WithAttrs.
func (h *CtxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	ctxHandler := h.clone()
	ctxHandler.opList = append(ctxHandler.opList, data{
		attrs: attrs,
	})
	return ctxHandler
}

// WithGroup works exactly like (slog.Handler).WithGroup.
func (h *CtxHandler) WithGroup(name string) slog.Handler {
	if len(name) == 0 {
		return h
	}
	ctxHandler := h.clone()
	ctxHandler.opList = append(ctxHandler.opList, data{
		group: name,
	})
	return ctxHandler
}

// LaxCtxHandler is an option for disable adding !BADCTX attr.
func LaxCtxHandler() ctxHandlerOption {
	return func(ctxHandler *CtxHandler) {
		ctxHandler.ignoreBADCTX = true
	}
}

func (h *CtxHandler) clone() *CtxHandler {
	return &CtxHandler{
		handler:      h.handler,
		opList:       h.opList,
		ignoreBADCTX: h.ignoreBADCTX,
	}
}
