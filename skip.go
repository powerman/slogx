package slogx

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

func LogSkip(ctx context.Context, skip int, handler slog.Handler, level slog.Level, msg string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(1+skip, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = handler.Handle(ctx, r)
}

func LogAttrsSkip(ctx context.Context, skip int, handler slog.Handler, level slog.Level, msg string, attrs ...slog.Attr) {
	var pcs [1]uintptr
	runtime.Callers(1+skip, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = handler.Handle(ctx, r)
}
