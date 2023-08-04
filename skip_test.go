package slogx_test

//go:generate -command MOCKGEN sh -c "$(git rev-parse --show-toplevel)/.buildcache/bin/$DOLLAR{DOLLAR}0 \"$DOLLAR{DOLLAR}@\"" mockgen
//go:generate MOCKGEN -destination=mock.handler_test.go -package=$GOPACKAGE log/slog Handler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/golang/mock/gomock"
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

	t.Panic(func() { slogx.LogSkip(ctx, 0, nil, slog.LevelError, "message", "err", io.EOF) })

	slogx.LogSkip(ctx, 0, h, slog.LevelError, "message", "err", io.EOF)
	t.Match(buf.String(), "level=ERROR source=\\S*/slogx/skip_test.go:32 msg=message err=EOF")

	buf.Truncate(0)
	func() {
		slogx.LogSkip(nil, 1, h, slog.LevelError, "message", "err", io.EOF)
	}()
	t.Match(buf.String(), "level=ERROR source=\\S*/slogx/skip_test.go:38 msg=message err=EOF")
}

func TestLogAttrsSkip(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := context.Background()
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true, Level: slog.LevelWarn})

	slogx.LogAttrsSkip(ctx, 0, h, slog.LevelInfo, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Len(buf.String(), 0)

	t.Panic(func() {
		slogx.LogAttrsSkip(ctx, 0, nil, slog.LevelWarn, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	})

	slogx.LogAttrsSkip(ctx, 0, h, slog.LevelWarn, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	t.Match(buf.String(), "level=WARN source=\\S*/slogx/skip_test.go:57 msg=message ID=18")

	buf.Truncate(0)
	func() {
		slogx.LogAttrsSkip(nil, 1, h, slog.LevelWarn, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
	}()
	t.Match(buf.String(), "level=WARN source=\\S*/slogx/skip_test.go:63 msg=message ID=18")
}

func TestLogSkipCtx(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	ctrl := gomock.NewController(t)

	h := NewMockHandler(ctrl)
	ctx := context.Background()
	h.EXPECT().Enabled(ctx, slog.LevelError).Return(true)
	h.EXPECT().Handle(ctx, gomock.Any()).Return(nil)
	slogx.LogSkip(nil, 0, h, slog.LevelError, "message", "err", io.EOF)
}

func TestLogAttrsSkipCtx(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	ctrl := gomock.NewController(t)

	h := NewMockHandler(ctrl)
	ctx := context.Background()
	h.EXPECT().Enabled(ctx, slog.LevelWarn).Return(true)
	h.EXPECT().Handle(ctx, gomock.Any()).Return(nil)
	slogx.LogAttrsSkip(nil, 1, h, slog.LevelWarn, "message", slog.Attr{Key: "ID", Value: slog.IntValue(18)})
}
