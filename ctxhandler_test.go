package slogx_test

import (
	"bytes"
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
	slogx.SetDefaultCtxHandler(h)
	t.True(slog.Default().Enabled(context.Background(), slog.LevelWarn))

	h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	ctx := slogx.NewContext(context.Background(), h)
	t.True(slog.Default().Enabled(ctx, slog.LevelError))
}

func TestCtxHandler(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	var h slog.Handler
	// With TextHandler
	h = slog.NewTextHandler(&buf, nil).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key1", "value1"), slog.String("key2", "value2")})
	ctx := slogx.NewContext(context.Background(), h)
	slogx.SetDefaultCtxHandler(slog.NewTextHandler(&buf, nil))
	slog.InfoContext(ctx, "some message")
	t.Match(buf.String(), `level=INFO msg="some message" g.key1=value1 g.key2=value2`)

	buf.Truncate(0)
	log := slog.With(slog.String("key1", "modified"))
	log.InfoContext(ctx, "some message")
	t.Match(buf.String(), `level=INFO msg="some message" g.key1=value1 g.key2=value2 g.key1=modified`)

	buf.Truncate(0)
	log = log.WithGroup("g2").With(slog.String("key3", "value3"))
	log.InfoContext(ctx, "some message")
	t.Match(buf.String(), `level=INFO msg="some message" g.key1=value1 g.key2=value2 g.key1=modified g.g2.key3=value3`)

	buf.Truncate(0)
	slog.InfoContext(ctx, "some message", "key4", "value4")
	t.Match(buf.String(), `level=INFO msg="some message" g.key1=value1 g.key2=value2 g.key4=value4`)

	// With JsonHandler
	buf.Truncate(0)
	h = slog.NewJSONHandler(&buf, nil).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key1", "value1"), slog.String("key2", "value2")})
	ctx = slogx.NewContext(context.Background(), h)
	slogx.SetDefaultCtxHandler(slog.NewJSONHandler(&buf, nil))
	slog.InfoContext(ctx, "some message")
	t.Match(buf.String(), `"level":"INFO","msg":"some message","g":{"key1":"value1","key2":"value2"}}`)

	buf.Truncate(0)
	log = slog.With(slog.String("key1", "modified"))
	log.InfoContext(ctx, "some message")
	t.Match(buf.String(), `"level":"INFO","msg":"some message","g":{"key1":"value1","key2":"value2","key1":"modified"}}`)

	buf.Truncate(0)
	log = log.WithGroup("g2").With(slog.String("key3", "value3"))
	log.InfoContext(ctx, "some message")
	t.Match(buf.String(), `"level":"INFO","msg":"some message","g":{"key1":"value1","key2":"value2","key1":"modified","g2":{"key3":"value3"}}}`)

	buf.Truncate(0)
	slog.InfoContext(ctx, "some message", "key4", "value4")
	t.Match(buf.String(), `"level":"INFO","msg":"some message","g":{"key1":"value1","key2":"value2","key4":"value4"}}`)
}

func TestLaxCtxHandler(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := context.Background()
	h := slog.NewTextHandler(&buf, nil).WithAttrs([]slog.Attr{slog.String("key1", "value1")})
	slogx.SetDefaultCtxHandler(h)
	slog.InfoContext(ctx, "some message")
	t.Match(buf.String(), `level=INFO msg="some message" key1=value1 !BADCTX`)

	buf.Truncate(0)
	slogx.SetDefaultCtxHandler(h, slogx.LaxCtxHandler())
	slog.InfoContext(ctx, "some message")
	t.NotMatch(buf.String(), "!BADCTX")
}
