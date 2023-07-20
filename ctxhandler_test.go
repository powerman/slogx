package slogx_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestEnabled(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	h := slog.NewTextHandler(os.Stdout, nil)
	handler := slogx.NewCtxHandler(h)
	t.True(handler.Enabled(context.Background(), slog.LevelWarn))

	h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	ctx := slogx.NewContext(context.Background(), h)
	t.True(handler.Enabled(ctx, slog.LevelError))
}
