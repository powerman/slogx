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
	slogx.LogSkip(ctx, 0, slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true}), slog.Level(8), "message", "err", io.EOF)
	t.Match(buf.String(), "level=ERROR source=.*/slogx/skip_test.go:21 msg=message err=EOF")

	buf.Truncate(0)
	slogx.LogSkip(ctx, 1, slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true}), slog.Level(8), "message", "err", io.EOF)
	t.Match(buf.String(), "level=ERROR source=.*/testing/testing.go:1595 msg=message err=EOF")
}

func TestLogAttrsSkip(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := context.Background()
	slogx.LogAttrsSkip(ctx, 0, slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true}), slog.Level(0), "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Match(buf.String(), "level=INFO source=.*/slogx/skip_test.go:35 msg=message ID=18")

	buf.Truncate(0)
	slogx.LogAttrsSkip(ctx, 1, slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true}), slog.Level(0), "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Match(buf.String(), "level=INFO source=.*/testing/testing.go:1595 msg=message ID=18")
}
