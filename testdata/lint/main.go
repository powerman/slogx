// Linter's testdata.
package main

import (
	"context"
	"log/slog"   //nolint:gocritic,staticcheck // By design.
	l "log/slog" //nolint:revive,gocritic,staticcheck // By design.
)

func main() {
	log := slog.Default()
	ctx := context.Background()

	log.Error("Error message")
	log.Warn("Warn message")
	log.Info("Info message")
	log.Debug("Debug message")
	log.ErrorContext(ctx, "Error message")
	log.WarnContext(ctx, "Warn message")
	log.InfoContext(ctx, "Info message")
	log.DebugContext(ctx, "Debug message")

	slog.Error("Error message")
	slog.Warn("Warn message")
	slog.Info("Info message")
	slog.Debug("Debug message")
	slog.ErrorContext(ctx, "Error message")
	slog.WarnContext(ctx, "Warn message")
	slog.InfoContext(ctx, "Info message")
	slog.DebugContext(ctx, "Debug message")

	ll := l.Default()

	ll.Error("Error message")
	ll.Warn("Warn message")
	ll.Info("Info message")
	ll.Debug("Debug message")
	ll.ErrorContext(ctx, "Error message")
	ll.WarnContext(ctx, "Warn message")
	ll.InfoContext(ctx, "Info message")
	ll.DebugContext(ctx, "Debug message")

	l.Error("Error message")
	l.Warn("Warn message")
	l.Info("Info message")
	l.Debug("Debug message")
	l.ErrorContext(ctx, "Error message")
	l.WarnContext(ctx, "Warn message")
	l.InfoContext(ctx, "Info message")
	l.DebugContext(ctx, "Debug message")
}
