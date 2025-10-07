package slogx

import (
	"context"
	"log/slog"

	"github.com/powerman/slogx/internal"
)

const (
	badCtx = "!BADCTX"
)

type contextKey int

const (
	contextKeyHandler contextKey = iota
)

// newContextWithHandler returns a new Context that carries value handler.
func newContextWithHandler(ctx context.Context, handler slog.Handler) context.Context {
	return context.WithValue(ctx, contextKeyHandler, handler)
}

// handlerFromContext returns a Handler value stored in ctx if exists or nil.
func handlerFromContext(ctx context.Context) slog.Handler {
	handler, _ := ctx.Value(contextKeyHandler).(slog.Handler)
	return handler
}

type contextHandlerConfig struct {
	omitBadCtx bool
}

// ContextHandlerOption is an option for [NewContextHandler].
type ContextHandlerOption func(*contextHandlerConfig)

// LaxContextHandler is an option for disable adding !BADCTX attr.
func LaxContextHandler() ContextHandlerOption {
	return func(h *contextHandlerConfig) {
		h.omitBadCtx = true
	}
}

// NewContextHandler creates an slog.Handler and a context with next handler inside.
//
// Returned contextHandler provides a way to use an [slog.Handler] stored in a context.
// This makes possible to store attrs and groups inside a (handler in a) context and
// make it work with default logger functions (like [slog.InfoContext]) without extra efforts
// (like getting logger from context first or providing logger explicitly in function arguments).
//
// Use [ContextWith], [ContextWithAttrs] and [ContextWithGroup] functions to
// add attrs and groups to a handler stored in a context.
//
// Follow these rules to use contextHandler correctly:
//
//  1. You should set returned contextHandler as a default logger's handler.
//     Without this it will be useless, because if you'll use non-default logger instance
//     everythere then you can add attrs to it directly and there is no need in contextHandler.
//  2. You should use returned context as a base context for your application.
//  3. Your application code should use slog.*Context functions
//     or (*slog.Logger).*Context methods for logging.
//  4. Use [NewDefaultContextLogger] to create a logger instance for third-party libraries
//     which do not support context-aware logging functions but support custom logger instance.
//  5. Do not use [ContextWith], [ContextWithAttrs] and [ContextWithGroup] functions after
//     slog.With* functions or (*slog.Logger).With* methods calls.
//
// By default contextHandler will add attr with key "!BADCTX" and current context value
// if it does not contain an [slog.Handler] - this helps to detect violations of the rules above.
// Use [LaxContextHandler] option to disable this behaviour.
//
// If you won't use slog.*Context functions or (*slog.Logger).*Context methods for logging
// or will use them with a context not created using returned context then next handler
// will be used as a fallback (thus attrs and groups stored in context will be ignored).
// Even if your application code is correct, this may still happen in third-party libraries
// which do not support context-aware logging functions and do not accept custom logger instance.
//
// Example usage in an HTTP server:
//
//	func main() {
//		handler := slog.NewJSONHandler(os.Stdout, nil)
//		ctx := slogx.SetDefaultContextHandler(context.Background(), handler)
//		ctx = slogx.ContextWith(ctx, "app", "example")
//		// ...
//		srv := &http.Server{
//			BaseContext: func(net.Listener) context.Context { return ctx },
//			//...
//		}
//		srv.ListenAndServe()
//		slog.InfoContext(ctx, "finished") // Will log "app" attribute.
//	}
//
//	func (handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//		ctx := r.Context()
//		ctx = slogx.ContextWith(ctx, "remote_addr", r.RemoteAddr)
//		slog.InfoContext(ctx, "message") // Will log "app" and "remote_addr" attributes.
//	}
func NewContextHandler(ctx context.Context, next slog.Handler, opts ...ContextHandlerOption) (
	ctxWithNext context.Context, contextHandler slog.Handler,
) {
	cfg := contextHandlerConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	h := NewWrapHandler(next, WrapHandlerConfig{
		Enabled: func(ctx context.Context, l slog.Level, _ *GroupOrAttrs, next slog.Handler) bool {
			handler := handlerFromContext(ctx)
			if handler == nil {
				handler = next
			}
			return handler.Enabled(ctx, l)
		},
		Handle: func(ctx context.Context, r slog.Record, goa *GroupOrAttrs, next slog.Handler) error {
			r = goa.Record(r)
			handler := handlerFromContext(ctx)
			if handler == nil {
				handler = next
				if !cfg.omitBadCtx {
					r.AddAttrs(slog.Any(badCtx, ctx))
				}
			}
			return handler.Handle(ctx, r)
		},
	})
	return newContextWithHandler(ctx, next), h
}

// SetDefaultContextHandler sets a contextHandler returned by [NewContextHandler]
// as a default logger's handler and returns a context with next handler inside.
// It is a shortcut for NewContextHandler + [slog.SetDefault].
//
// See NewContextHandler for details and usage rules.
func SetDefaultContextHandler(ctx context.Context, next slog.Handler, opts ...ContextHandlerOption) context.Context {
	ctx, h := NewContextHandler(ctx, next, opts...)
	slog.SetDefault(slog.New(h))
	return ctx
}

// ContextWith applies attrs to a handler stored in ctx.
func ContextWith(ctx context.Context, attrs ...any) context.Context {
	handler := handlerFromContext(ctx)
	return newContextWithHandler(ctx, handler.WithAttrs(internal.ArgsToAttrSlice(attrs)))
}

// ContextWithAttrs applies attrs to a handler stored in ctx.
func ContextWithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	handler := handlerFromContext(ctx)
	return newContextWithHandler(ctx, handler.WithAttrs(attrs))
}

// ContextWithGroup applies group to a handler stored in ctx.
func ContextWithGroup(ctx context.Context, group string) context.Context {
	handler := handlerFromContext(ctx)
	return newContextWithHandler(ctx, handler.WithGroup(group))
}
