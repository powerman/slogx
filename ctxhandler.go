package slogx

import (
	"context"
	"log/slog"
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
	ignoreBADCTX bool
	handler      slog.Handler
}

// CtxHandlerOption is an option for a CtxHandler.
type CtxHandlerOption func(*CtxHandler)

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

// WithAttrs works as (slog.Handler).WithAttrs.
func (h *CtxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	panic("TODO")
}

// WithGroup works as (slog.Handler).WithGroup.
func (h *CtxHandler) WithGroup(name string) slog.Handler {
	panic("TODO")
}

// SetDefaultCtxHandler sets a CtxHandler as a default logger
// and returns context with set handler inside.
// It applies given options. If opts is nil, the default options are used.
func SetDefaultCtxHandler(fallback slog.Handler, opts ...CtxHandlerOption) context.Context {
	panic("TODO")
}

// ContextWithAttrs applies attrs to a handler stored in ctx.
func ContextWithAttrs(ctx context.Context, attrs ...any) context.Context {
	panic("TODO")
}

// ContextWithGroup applies group to a handler stored in ctx.
func ContextWithGroup(ctx context.Context, group string) context.Context {
	panic("TODO")
}

// LaxCtxHandler is an option for disable adding !BADCTX attr.
func LaxCtxHandler() CtxHandlerOption {
	panic("TODO")
}
