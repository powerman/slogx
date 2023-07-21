package slogx

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// LogSkip emits a log record using handler with the
// current time and the given level and message.
// Value skip=0 works exactly like (*slog.Logger).Log,
// value skip=1 skips caller of LogSkip() etc.
func LogSkip(ctx context.Context, skip int, handler slog.Handler, level slog.Level, msg string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !handler.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(2+skip, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = handler.Handle(ctx, r)
}

// LogAttrsSkip emits a log record using handler with the
// current time and the given level and message.
// Value skip=0 works exactly like (*slog.Logger).Log,
// value skip=1 skips caller of LogSkip() etc.
func LogAttrsSkip(ctx context.Context, skip int, handler slog.Handler, level slog.Level, msg string, attrs ...slog.Attr) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !handler.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(2+skip, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = handler.Handle(ctx, r)
}
