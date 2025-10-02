package main

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/powerman/slogx"
)

func main() {
	slog.SetDefault(slog.New(slogx.NewLayoutHandler(os.Stdout, &slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey:    "",              // Remove time completely.
			slog.LevelKey:   "%-5s",          // left aligned, minimum width 5 to fit all levels
			slog.MessageKey: " %s",           // Simple formatting for message
			"password":      " password=***", // Hide password completely
			"error":         " err: %s",      // Custom prefix for error
			"duration":      " %8s",          // right aligned, minimum width 10 for better alignment
			"user":          " @%s",
		},
		PrefixKeys: []string{
			"duration", // duration width is known, put it at the beginning
		},
		SuffixKeys: []string{
			"error",
			"user",
			slog.SourceKey, // source width is unknown, put it at the end
		},
	})))

	slog.Info("User login attempt",
		slog.String("user", "alice"),
		slog.String("password", "secret123"),
		slog.Duration("duration", 1230*time.Millisecond),
		slog.Int("status", 200),
		slog.String("ip", "192.168.1.1"),
	)

	slog.Error("Database connection failed",
		slog.String("user", "bob"),
		slog.Duration("duration", 30456*time.Millisecond),
		slog.Any("error", errors.New("connection timeout")),
	)
}
