package slogx_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestLogSkip(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := context.Background()
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true, Level: slog.LevelError})

	slogx.LogSkip(ctx, 0, h, slog.LevelWarn, "message", "err", io.EOF)
	t.Len(buf.String(), 0)

	slogx.LogSkip(ctx, 0, h, slog.LevelError, "message", "err", io.EOF)
	t.Match(buf.String(), "level=ERROR source=.*/slogx/skip_test.go:26 msg=message err=EOF")

	buf.Truncate(0)
	slogx.LogSkip(nil, 1, h, slog.LevelError, "message", "err", io.EOF)
	t.Match(buf.String(), "level=ERROR source=.*/testing/testing.go:[0-9]+ msg=message err=EOF")
}

func TestLogAttrsSkip(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := context.Background()
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true, Level: slog.LevelWarn})

	slogx.LogAttrsSkip(ctx, 0, h, slog.LevelInfo, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Len(buf.String(), 0)

	slogx.LogAttrsSkip(ctx, 0, h, slog.LevelWarn, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Match(buf.String(), "level=WARN source=.*/slogx/skip_test.go:45 msg=message ID=18")

	buf.Truncate(0)
	slogx.LogAttrsSkip(nil, 1, h, slog.LevelWarn, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Match(buf.String(), "level=WARN source=.*/testing/testing.go:[0-9]+ msg=message ID=18")
}
