package slogx_test

import (
	"log/slog"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestParseLevel(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	tests := []struct {
		levelName string
		want      slog.Level
	}{
		{"Err", slog.LevelError},
		{"error ", slog.LevelError},
		{" fatal", slog.LevelError},
		{"crit", slog.LevelError},
		{"critical", slog.LevelError},
		{"alert", slog.LevelError},
		{"emerg", slog.LevelError},
		{"emergency", slog.LevelError},
		{"wrn", slog.LevelWarn},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"inf", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"notice", slog.LevelInfo},
		{"dbg", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"trace", slog.LevelDebug},
		{"", slog.LevelDebug},
		{"qwe", slog.LevelDebug},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(tt *testing.T) {
			t := check.T(tt).MustAll()
			t.Equal(slogx.ParseLevel(tc.levelName), tc.want)
		})
	}
}
