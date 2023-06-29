package slogx

import (
	"log/slog"
	"strings"
)

// ParseLevel convert levelName from flag or config file into slog.Level.
func ParseLevel(levelName string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(levelName)) {
	case "err", "error", "fatal", "crit", "critical", "alert", "emerg", "emergency":
		return slog.LevelError
	case "wrn", "warn", "warning":
		return slog.LevelWarn
	case "inf", "info", "notice":
		return slog.LevelInfo
	case "dbg", "debug", "trace":
		return slog.LevelDebug

	default:
		slog.Debug("failed", "levelName", levelName) // Will be changed to use custom DefaultLogger.
		return slog.LevelDebug
	}
}
