// Linter's testdata.
package main

import (
	"context"
	"log/slog"
	l "log/slog"
)

func main() {
	log := slog.Default()
	ctx := context.Background()

	log.Error("error message")
	log.Warn("warn message")
	log.Info("info message")
	log.Debug("debug message")
	log.ErrorContext(ctx, "error message")
	log.WarnContext(ctx, "warn message")
	log.InfoContext(ctx, "info message")
	log.DebugContext(ctx, "debug message")

	slog.Error("error message")
	slog.Warn("warn message")
	slog.Info("info message")
	slog.Debug("debug message")
	slog.ErrorContext(ctx, "error message")
	slog.WarnContext(ctx, "warn message")
	slog.InfoContext(ctx, "info message")
	slog.DebugContext(ctx, "debug message")

	ll := l.Default()

	ll.Error("error message")
	ll.Warn("warn message")
	ll.Info("info message")
	ll.Debug("debug message")
	ll.ErrorContext(ctx, "error message")
	ll.WarnContext(ctx, "warn message")
	ll.InfoContext(ctx, "info message")
	ll.DebugContext(ctx, "debug message")

	l.Error("error message")
	l.Warn("warn message")
	l.Info("info message")
	l.Debug("debug message")
	l.ErrorContext(ctx, "error message")
	l.WarnContext(ctx, "warn message")
	l.InfoContext(ctx, "info message")
	l.DebugContext(ctx, "debug message")
}
