package slogx

import (
	"context"
	"log/slog"

	"github.com/powerman/slogx/internal"
)

const (
	badCtx = "!BADCTX"
)

// ContextHandler provides a way to use an [slog.Handler] stored in a context.
// This makes possible to store attrs and groups inside a context and make it magically work
// with global logger functions like [slog.InfoContext] without extra efforts
// (like getting logger from context first or providing logger explicitly in function arguments).
//
// ContextHandler must be set as a default logger's handler.
// Without this it will be useless, because if you'll use non-default logger instance
// everythere then you can add attrs to it directly and there is no need in ContextHandler.
//
// Example:
//
//	func main() {
//		handler := slog.NewJSONHandler(os.Stdout, nil)
//		ctx := slogx.SetDefaultContextHandler(context.Background(), handler)
//		// ...
//		srv := &http.Server{
//			BaseContext: func(net.Listener) context.Context { return ctx },
//			//...
//		}
//		srv.ListenAndServe()
//		slog.InfoContext(ctx, "done")
//	}
//
//	func (handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//		ctx := r.Context()
//		ctx = slogx.ContextWith(ctx, "remote_addr", r.RemoteAddr)
//		handleRequest(ctx)
//	}
//
//	func handleRequest(ctx context.Context) {
//		slog.InfoContext(ctx, "message")    // Will also log "remote_addr" attribute.
//	}
type ContextHandler struct {
	fallback   slog.Handler
	ops        []handlerOp
	omitBadCtx bool
}

type handlerOp struct {
	group string
	attrs []slog.Attr
}

// ContextHandlerOption is an option for [NewContextHandler].
type ContextHandlerOption func(*ContextHandler)

// NewContextHandler creates a ContextHandler and a context with next handler inside.
//
// Use [ContextWith], [ContextWithAttrs] and [ContextWithGroup] functions to
// add attrs and groups to a handler stored in a context.
//
// Follow these rules to use ContextHandler correctly:
//
//  1. You should set returned ContextHandler as a default logger's handler.
//  2. You should use returned context as a base context for your application.
//  3. Your application code should use slog.*Context functions
//     or (*slog.Logger).*Context methods for logging.
//  4. Use [NewDefaultContextLogger] to create a logger instance for third-party libraries
//     which do not support context-aware logging functions but support custom logger instance.
//  5. Do not use [ContextWith], [ContextWithAttrs] and [ContextWithGroup] functions after
//     slog.With* functions or (*slog.Logger).With* methods calls.
//
// By default ContextHandler will add attr with key "!BADCTX" and current context value
// if it does not contain an [slog.Handler] - this helps to detect violations of the rules above.
// Use [LaxContextHandler] option to disable this behaviour.
//
// If you won't use slog.*Context functions or (*slog.Logger).*Context methods for logging
// or will use them with a context not created using returned context then next handler
// will be used as a fallback (thus attrs and groups stored in context will be ignored).
// Even if your application code is correct, this may still happen in third-party libraries
// which do not support context-aware logging functions and do not accept custom logger instance.
func NewContextHandler(ctx context.Context, next slog.Handler, opts ...ContextHandlerOption) (context.Context, *ContextHandler) {
	h := &ContextHandler{
		fallback: next,
	}
	for _, opt := range opts {
		opt(h)
	}
	return NewContextWithHandler(ctx, next), h
}

// Enabled implements slog.Handler interface.
// It uses handler returned by HandlerFromContext or fallback handler.
func (h *ContextHandler) Enabled(ctx context.Context, l slog.Level) bool {
	handler := HandlerFromContext(ctx)
	if handler == nil {
		handler = h.fallback
	}
	return handler.Enabled(ctx, l)
}

// Handle implements slog.Handler interface.
// It uses handler returned by HandlerFromContext or fallback handler.
// Adds !BADCTX attr if HandlerFromContext returns nil. Use LaxContextHandler to disable this behaviour.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	handler := HandlerFromContext(ctx)
	if handler == nil {
		handler = h.fallback
		if !h.omitBadCtx {
			handler = handler.WithAttrs([]slog.Attr{slog.Any(badCtx, ctx)})
		}
	}
	for _, op := range h.ops {
		if op.group != "" {
			handler = handler.WithGroup(op.group)
		} else {
			handler = handler.WithAttrs(op.attrs)
		}
	}
	return handler.Handle(ctx, r)
}

// WithAttrs implements slog.Handler interface.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.withOp(handlerOp{attrs: attrs})
	return h2
}

// WithGroup implements slog.Handler interface.
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.withOp(handlerOp{group: name})
	return h2
}

// SetDefaultContextHandler sets a ContextHandler as a default logger's handler
// and returns a context with next handler inside.
// It is a shortcut for NewContextHandler + [slog.SetDefault].
//
// See [NewContextHandler] for details and usage rules.
func SetDefaultContextHandler(ctx context.Context, next slog.Handler, opts ...ContextHandlerOption) context.Context {
	ctx, h := NewContextHandler(ctx, next, opts...)
	slog.SetDefault(slog.New(h))
	return ctx
}

// ContextWithAttrs applies attrs to a handler stored in ctx.
func ContextWithAttrs(ctx context.Context, attrs ...any) context.Context {
	handler := HandlerFromContext(ctx)
	return NewContextWithHandler(ctx, handler.WithAttrs(internal.ArgsToAttrSlice(attrs)))
}

// ContextWithGroup applies group to a handler stored in ctx.
func ContextWithGroup(ctx context.Context, group string) context.Context {
	handler := HandlerFromContext(ctx)
	return NewContextWithHandler(ctx, handler.WithGroup(group))
}

// LaxContextHandler is an option for disable adding !BADCTX attr.
func LaxContextHandler() ContextHandlerOption {
	return func(h *ContextHandler) {
		h.omitBadCtx = true
	}
}

func (h *ContextHandler) withOp(op handlerOp) *ContextHandler {
	h2 := *h // Create a copy to avoid modifying the original handler.
	h2.ops = append(h2.ops[:len(h2.ops):len(h2.ops)], op)
	return &h2
}
