package slogx

import (
	"context"
	"log/slog"
)

// A WrapHandlerConfig sets configuration for [WrapHandler].
//
// All fields are optional, but without any callbacks it won't be very useful.
//
// When ProxyWith is true, both WithAttrs and WithGroup calls are proxied to the next handler.
// This mode is useful for transparent proxies that just want to intercept
// Enabled and/or Handle calls and do not need to handle top-level attributes.
// In this mode, Enabled and Handle callbacks get nil [*GroupOrAttrs].
//
// When ProxyWith is false and ProxyWithAttrs is true,
// WithAttrs calls are proxied to the next handler until the first WithGroup call.
// This mode is useful for handlers that want to add top-level attributes
// but want to proxy initial WithAttrs calls to the next handler to keep possible optimizations
// in the next handler (e.g. pre-rendered prefix in slog.TextHandler and slog.JSONHandler).
// In this mode, Enabled and Handle callbacks get nil *GroupOrAttrs before
// the first WithGroup call with non-empty group.
//
// If both ProxyWith and ProxyWithAttrs are false,
// all WithAttrs and WithGroup calls are accumulated in a *GroupOrAttrs
// passed to Enabled and Handle callbacks.
// This mode is useful for handlers that want to see all attributes and groups.
//
// If Enabled callback is nil then the next handler's Enabled method is called.
//
// If Handle callback is nil then the Record is passed to the next handler
// after applying accumulated *GroupOrAttrs to it.
//
// If both callbacks are nil and ProxyWith is true
// then WrapHandler behaves like a (useless) transparent proxy.
//
// Example implementation of Enabled and Handle callbacks equivalent to nil callbacks:
//
//	Enabled: func(ctx context.Context, l slog.Level, _ *slogx.GroupOrAttrs, next slog.Handler) bool {
//	    return next.Enabled(ctx, l)
//	},
//	Handle: func(ctx context.Context, r slog.Record, goa *slogx.GroupOrAttrs, next slog.Handler) error {
//	    return next.Handle(ctx, goa.Record(r))
//	}
type WrapHandlerConfig struct {
	Enabled        func(context.Context, slog.Level, *GroupOrAttrs, slog.Handler) bool
	Handle         func(context.Context, slog.Record, *GroupOrAttrs, slog.Handler) error
	ProxyWith      bool // Proxy both WithAttrs and WithGroup calls.
	ProxyWithAttrs bool // Proxy WithAttrs calls before first WithGroup call.
}

// WrapHandler is a [slog.Handler] that wraps another slog.Handler.
// It is a useful building block for handlers that want to wrap another handler.
//
// It is able to proxy or collect WithAttrs and WithGroup calls using [GroupOrAttrs]
// and to call optional Enabled and Handle callbacks depending on configuration.
type WrapHandler struct {
	cfg  WrapHandlerConfig
	next slog.Handler
	goa  *GroupOrAttrs
}

// NewWrapHandler returns a new WrapHandler that delegates to next handler
// using the provided configuration.
// You need to provide at least one callback in the configuration for it to be useful.
func NewWrapHandler(next slog.Handler, cfg WrapHandlerConfig) *WrapHandler {
	return &WrapHandler{cfg: cfg, next: next}
}

// Enabled implements [slog.Handler] interface.
func (h *WrapHandler) Enabled(ctx context.Context, l slog.Level) bool {
	if h.cfg.Enabled != nil {
		return h.cfg.Enabled(ctx, l, h.goa, h.next)
	}
	return h.next.Enabled(ctx, l)
}

// Handle implements [slog.Handler] interface.
func (h *WrapHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.cfg.Handle != nil {
		return h.cfg.Handle(ctx, r, h.goa, h.next)
	}
	return h.next.Handle(ctx, h.goa.Record(r))
}

// WithAttrs implements [slog.Handler] interface.
func (h *WrapHandler) WithAttrs(as []slog.Attr) slog.Handler {
	if h.cfg.ProxyWith || (h.cfg.ProxyWithAttrs && h.goa == nil) {
		if next := h.next.WithAttrs(as); next != h.next {
			h2 := *h
			h2.next = next
			return &h2
		}
	} else if goa := h.goa.WithAttrs(as); goa != h.goa {
		h2 := *h
		h2.goa = goa
		return &h2
	}
	return h
}

// WithGroup implements [slog.Handler] interface.
func (h *WrapHandler) WithGroup(name string) slog.Handler {
	if h.cfg.ProxyWith {
		if next := h.next.WithGroup(name); next != h.next {
			h2 := *h
			h2.next = next
			return &h2
		}
	} else if goa := h.goa.WithGroup(name); goa != h.goa {
		h2 := *h
		h2.goa = goa
		return &h2
	}
	return h
}
