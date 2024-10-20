package slogx

import (
	"context"
	"log/slog"
)

const (
	badKey = "!BADKEY"
	badCtx = "!BADCTX"
)

// CtxHandler provides a way to use slog.Handler stored in a context instead of slog.Logger.
// This makes possible to store extra slog.Attr inside a context and make it magically work
// without needs to get slog.Logger out of context each time you need to log something.
//
// CtxHandler should be used as a default logger's handler. So it's useful only for
// applications but not libraries - libraries shouldn't expect concrete configuration of
// default logger and can't expect availability of CtxHandler's features.
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
// With CtxHandler same functionality became:
//
//	func main() {
//		handler := slog.NewJSONHandler(os.Stdout, nil)
//		ctx := slogx.SetDefaultCtxHandler(context.Background(), handler)
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
// Code not aware about CtxHandler (e.g. libraries) will continue to work correctly, but there
// are some extra restrictions:
//   - You should not modify default logger after initial configuration.
//   - If you'll create new logger instance (e.g. using slog.With(...)) then you should not
//     modify Attrs or Group inside ctx while using that logger instance.
//
// Non-Context functions like slog.Info() will work, but they will ignore Attr/Group
// configured inside ctx.
//
// By default CtxHandler will add attr with key "!BADCTX" and value ctx if ctx does not contain
// slog handler, but this can be disabled using LaxCtxHandler option.
type CtxHandler struct {
	fallback   slog.Handler
	ops        []handlerOp
	omitBadCtx bool
}

type handlerOp struct {
	group string
	attrs []slog.Attr
}

type ctxHandlerOption func(*CtxHandler)

func newCtxHandler(fallback slog.Handler, opts ...ctxHandlerOption) *CtxHandler {
	ctxHandler := &CtxHandler{
		fallback: fallback,
	}
	for _, opt := range opts {
		opt(ctxHandler)
	}
	return ctxHandler
}

// Enabled implements slog.Handler interface.
// It uses handler returned by HandlerFromContext or fallback handler.
func (h *CtxHandler) Enabled(ctx context.Context, l slog.Level) bool {
	handler := HandlerFromContext(ctx)
	if handler == nil {
		handler = h.fallback
	}
	return handler.Enabled(ctx, l)
}

// Handle implements slog.Handler interface.
// It uses handler returned by HandlerFromContext or fallback handler.
// Adds !BADCTX attr if HandlerFromContext returns nil. Use LaxCtxHandler to disable this behaviour.
func (h *CtxHandler) Handle(ctx context.Context, r slog.Record) error {
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
func (h *CtxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	ctxHandler := h.withOp(handlerOp{attrs: attrs})
	return ctxHandler
}

// WithGroup implements slog.Handler interface.
func (h *CtxHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	ctxHandler := h.withOp(handlerOp{group: name})
	return ctxHandler
}

// SetDefaultCtxHandler sets a CtxHandler as a default logger's handler
// and returns context with this handler inside.
func SetDefaultCtxHandler(ctx context.Context, fallback slog.Handler, opts ...ctxHandlerOption) context.Context {
	slog.SetDefault(slog.New(newCtxHandler(fallback, opts...)))
	return NewContextWithHandler(ctx, fallback)
}

// ContextWithAttrs applies attrs to a handler stored in ctx.
func ContextWithAttrs(ctx context.Context, attrs ...any) context.Context {
	handler := HandlerFromContext(ctx)
	return NewContextWithHandler(ctx, handler.WithAttrs(argsToAttrSlice(attrs)))
}

// ContextWithGroup applies group to a handler stored in ctx.
func ContextWithGroup(ctx context.Context, group string) context.Context {
	handler := HandlerFromContext(ctx)
	return NewContextWithHandler(ctx, handler.WithGroup(group))
}

// LaxCtxHandler is an option for disable adding !BADCTX attr.
func LaxCtxHandler() ctxHandlerOption { //nolint:revive // By design.
	return func(ctxHandler *CtxHandler) {
		ctxHandler.omitBadCtx = true
	}
}

func (h CtxHandler) withOp(op handlerOp) *CtxHandler {
	h.ops = append(h.ops[:len(h.ops):len(h.ops)], op) //nolint:revive // By design.
	return &h
}
