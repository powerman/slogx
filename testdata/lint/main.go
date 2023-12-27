// Linter's testdata.
package main

import "log/slog"

func main() {
	log := slog.Default()
	log.Error("error message")
	log.Warn("warn message")
	log.Info("info message")
	log.Debug("debug message")

	slog.Error("error message")
	slog.Warn("warn message")
	slog.Info("info message")
	slog.Debug("debug message")
}
