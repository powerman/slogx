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
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true, Level: slog.Level(8)})

	slogx.LogSkip(ctx, 0, h, slog.Level(4), "message", "err", io.EOF)
	t.Len(buf.String(), 0)

	slogx.LogSkip(ctx, 0, h, slog.Level(8), "message", "err", io.EOF)
	t.Match(buf.String(), "level=ERROR source=.*/slogx/skip_test.go:26 msg=message err=EOF")

	buf.Truncate(0)
	slogx.LogSkip(nil, 1, h, slog.Level(8), "message", "err", io.EOF)
	t.Match(buf.String(), "level=ERROR source=.*/testing/testing.go:1595 msg=message err=EOF")
}

func TestLogAttrsSkip(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := context.Background()
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true, Level: slog.Level(4)})

	slogx.LogAttrsSkip(ctx, 0, h, slog.Level(0), "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Len(buf.String(), 0)

	slogx.LogAttrsSkip(ctx, 0, h, slog.Level(4), "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Match(buf.String(), "level=WARN source=.*/slogx/skip_test.go:45 msg=message ID=18")

	buf.Truncate(0)
	slogx.LogAttrsSkip(nil, 1, h, slog.Level(4), "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Match(buf.String(), "level=WARN source=.*/testing/testing.go:1595 msg=message ID=18")
}
