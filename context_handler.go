package slogx

import (
	"context"
	"log/slog"

	"github.com/powerman/slogx/internal"
)

const (
	badCtx = "!BADCTX"
)

// ContextHandler provides a way to use slog.Handler stored in a context instead of slog.Logger.
// This makes possible to store extra slog.Attr inside a context and make it magically work
// without needs to get slog.Logger out of context each time you need to log something.
//
// ContextHandler should be used as a default logger's handler. So it's useful only for
// applications but not libraries - libraries shouldn't expect concrete configuration of
// default logger and can't expect availability of ContextHandler's features.
//
// Usually when we need a context-specific logging we have to store pre-configured logger
// inside a context. But then everywhere we need to log something we have to get logger
// from context first, which is annoying. Also it means we have to use own logger instance and
// unable to use global logger and log using functions like slog.InfoContext. Example:
//
//	func main() {
//		log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//		slog.SetDefault(log)
//		// ...
//		srv := &http.Server{
//			//...
//		}
//		srv.ListenAndServe()
//		log.Info("done")
//	}
//
//	func (handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//		ctx := r.Context()
//		log := slog.With("remote_addr", r.RemoteAddr)
//		ctx = slogx.NewContextWithLogger(ctx, log)
//		handleRequest(ctx)
//	}
//
//	func handleRequest(ctx context.Context) {
//		log := slogx.LoggerFromContext(ctx) // <-- THIS LINE IS EVERYWHERE!
//		log.Info("message")                 // Will also log "remote_addr" attribute.
//	}
//
// With ContextHandler same functionality became:
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
//		ctx = slogx.ContextWithAttrs(ctx, "remote_addr", r.RemoteAddr)
//		handleRequest(ctx)
//	}
//
//	func handleRequest(ctx context.Context) {
//		slog.InfoContext(ctx, "message")    // Will also log "remote_addr" attribute.
//	}
//
// Code not aware about ContextHandler (e.g. libraries) will continue to work correctly, but there
// are some extra restrictions:
//   - You should not modify default logger after initial configuration.
//   - If you'll create new logger instance (e.g. using slog.With(...)) then you should not
//     modify Attrs or Group inside ctx while using that logger instance.
//
// Non-Context functions like slog.Info() will work, but they will ignore Attr/Group
// configured inside ctx.
//
// By default ContextHandler will add attr with key "!BADCTX" and value ctx if ctx does not contain
// slog handler, but this can be disabled using LaxContextHandler option.
type ContextHandler struct {
	fallback   slog.Handler
	ops        []handlerOp
	omitBadCtx bool
}

type handlerOp struct {
	group string
	attrs []slog.Attr
}

type contextHandlerOption func(*ContextHandler)

func newContextHandler(fallback slog.Handler, opts ...contextHandlerOption) *ContextHandler {
	h := &ContextHandler{
		fallback: fallback,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
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
// and returns context with this handler inside.
func SetDefaultContextHandler(ctx context.Context, fallback slog.Handler, opts ...contextHandlerOption) context.Context {
	slog.SetDefault(slog.New(newContextHandler(fallback, opts...)))
	return NewContextWithHandler(ctx, fallback)
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
func LaxContextHandler() contextHandlerOption { //nolint:revive // By design.
	return func(h *ContextHandler) {
		h.omitBadCtx = true
	}
}

func (h *ContextHandler) withOp(op handlerOp) *ContextHandler {
	h2 := *h // Create a copy to avoid modifying the original handler.
	h2.ops = append(h2.ops[:len(h2.ops):len(h2.ops)], op)
	return &h2
}
