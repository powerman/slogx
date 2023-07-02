package slogx

import (
	"log/slog"
	"strings"
)

// ParseLevel converts log level name into slog.Level.
// It is case insensitive, ignores surrounding spaces
// and accepts shortened level name. In case of unknown
// log level name it will return slog.LevelDebug.
func ParseLevel(levelName string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(levelName)) {
	case "err", "error":
		return slog.LevelError
	case "wrn", "warn", "warning":
		return slog.LevelWarn
	case "inf", "info":
		return slog.LevelInfo
	case "dbg", "debug":
		return slog.LevelDebug

	default:
		return slog.LevelDebug
	}
}
