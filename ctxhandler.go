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
	const size = 64 << 10 // TODO
	ctxHandler := &CtxHandler{
		fallback: fallback,
	}
	for _, opt := range opts {
		opt(ctxHandler)
	}
	return ctxHandler
}

// Enabled implements slog.Handler interface.
// It uses handler returned by FromContext or fallback handler.
func (h *CtxHandler) Enabled(ctx context.Context, l slog.Level) bool {
	handler := FromContext(ctx)
	if handler == nil {
		handler = h.fallback
	}
	return handler.Enabled(ctx, l)
}

// Handle works as (slog.Handler).Handler.
// It uses handler returned by FromContext or fallback handler.
// Adds !BADCTX attr if FromContext returns nil. Use LaxCtxHandler to disable this behaviour.
func (h *CtxHandler) Handle(ctx context.Context, r slog.Record) error {
	handler := FromContext(ctx)
	if handler == nil {
		handler = h.fallback
		if !h.omitBadCtx {
			handler = handler.WithAttrs([]slog.Attr{slog.Any(badCtx, ctx)})
		}
	}
	for _, op := range h.ops {
		if len(op.group) > 0 {
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
	ctxHandler := h.clone()
	ctxHandler.ops = append(ctxHandler.ops, handlerOp{
		attrs: attrs,
	})
	return ctxHandler
}

// WithGroup implements slog.Handler interface.
func (h *CtxHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	ctxHandler := h.clone()
	ctxHandler.ops = append(ctxHandler.ops, handlerOp{
		group: name,
	})
	return ctxHandler
}

// SetDefaultCtxHandler sets a CtxHandler as a default logger's handler
// and returns context with this handler inside. TODO (this)
func SetDefaultCtxHandler(ctx context.Context, fallback slog.Handler, opts ...ctxHandlerOption) context.Context {
	slog.SetDefault(slog.New(newCtxHandler(fallback, opts...)))
	return NewContext(ctx, fallback)
}

// ContextWithAttrs applies attrs to a handler stored in ctx.
func ContextWithAttrs(ctx context.Context, attrs ...any) context.Context {
	handler := FromContext(ctx)
	return NewContext(ctx, handler.WithAttrs(argsToAttrSlice(attrs)))
}

// ContextWithGroup applies group to a handler stored in ctx.
func ContextWithGroup(ctx context.Context, group string) context.Context {
	handler := FromContext(ctx)
	return NewContext(ctx, handler.WithGroup(group))
}

// LaxCtxHandler is an option for disable adding !BADCTX attr.
func LaxCtxHandler() ctxHandlerOption { //nolint:revive // By design.
	return func(ctxHandler *CtxHandler) {
		ctxHandler.omitBadCtx = true
	}
}

func (h *CtxHandler) clone() *CtxHandler {
	return &CtxHandler{
		fallback:   h.fallback,
		ops:        h.ops,
		omitBadCtx: h.omitBadCtx,
	}
}

func argsToAttrSlice(args []any) []slog.Attr {
	var (
		attr  slog.Attr
		attrs []slog.Attr
	)
	for len(args) > 0 {
		attr, args = argsToAttr(args)
		attrs = append(attrs, attr)
	}
	return attrs
}

// argsToAttr turns a prefix of the nonempty args slice into an Attr
// and returns the unconsumed portion of the slice.
// If args[0] is an Attr, it returns it.
// If args[0] is a string, it treats the first two elements as
// a key-value pair.
// Otherwise, it treats args[0] as a value with a missing key.
func argsToAttr(args []any) (slog.Attr, []any) {
	switch x := args[0].(type) {
	case string:
		if len(args) == 1 {
			return slog.String(badKey, x), nil
		}
		return slog.Any(x, args[1]), args[2:]

	case slog.Attr:
		return x, args[1:]

	default:
		return slog.Any(badKey, x), args[1:]
	}
}
