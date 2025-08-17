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
	slogx.SetDefaultCtxHandler(context.Background(), h)
	t.True(slog.Default().Enabled(context.Background(), slog.LevelWarn))
	t.False(slog.Default().Enabled(context.Background(), slog.LevelDebug))

	h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	ctx := slogx.NewContextWithHandler(context.Background(), h)
	t.True(slog.Default().Enabled(ctx, slog.LevelError))
	t.False(slog.Default().Enabled(ctx, slog.LevelWarn))
}

func TestCtxHandler(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	var h slog.Handler
	// With TextHandler
	slogx.SetDefaultCtxHandler(context.Background(), slog.NewTextHandler(&buf, nil))
	h = slog.NewTextHandler(&buf, nil).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key1", "value1"), slog.String("key2", "value2")})
	ctx := slogx.NewContextWithHandler(context.Background(), h)
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `level=INFO msg="Some message" g.key1=value1 g.key2=value2`)

	buf.Reset()
	log := slog.With(slog.String("key1", "modified"))
	log.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `level=INFO msg="Some message" g.key1=value1 g.key2=value2 g.key1=modified`)

	buf.Reset()
	log = log.WithGroup("g2").With(slog.String("key3", "value3"))
	log.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `level=INFO msg="Some message" g.key1=value1 g.key2=value2 g.key1=modified g.g2.key3=value3`)

	buf.Reset()
	slog.InfoContext(ctx, "Some message", "key4", "value4")
	t.Match(buf.String(), `level=INFO msg="Some message" g.key1=value1 g.key2=value2 g.key4=value4`)

	// With JsonHandler
	slogx.SetDefaultCtxHandler(context.Background(), slog.NewJSONHandler(&buf, nil))
	h = slog.NewJSONHandler(&buf, nil).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key1", "value1"), slog.String("key2", "value2")})
	ctx = slogx.NewContextWithHandler(context.Background(), h)
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"level":"INFO","msg":"Some message","g":{"key1":"value1","key2":"value2"}}`)

	buf.Reset()
	log = slog.With(slog.String("key1", "modified"))
	log.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"level":"INFO","msg":"Some message","g":{"key1":"value1","key2":"value2","key1":"modified"}}`)

	buf.Reset()
	log = log.WithGroup("g2").With(slog.String("key3", "value3"))
	log.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"level":"INFO","msg":"Some message","g":{"key1":"value1","key2":"value2","key1":"modified","g2":{"key3":"value3"}}}`)

	buf.Reset()
	slog.InfoContext(ctx, "Some message", "key4", "value4")
	t.Match(buf.String(), `"level":"INFO","msg":"Some message","g":{"key1":"value1","key2":"value2","key4":"value4"}}`)

	// WithAttrs/WithGroup with empty parameter
	handler := slog.Default().Handler()
	t.DeepEqual(handler.WithAttrs([]slog.Attr{}), handler)
	t.DeepEqual(handler.WithGroup(""), handler)
}

func TestContextWith(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := slogx.SetDefaultCtxHandler(context.Background(), slog.NewTextHandler(&buf, nil))

	ctx = slogx.ContextWithAttrs(ctx, "key1", "value1")
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"Some message" key1=value1`)

	buf.Reset()
	ctx = slogx.ContextWithGroup(ctx, "g1")
	ctx = slogx.ContextWithAttrs(ctx, "key2", "value2")
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"Some message" key1=value1 g1.key2=value2`)

	buf.Reset()
	ctx = slogx.ContextWithGroup(ctx, "g2")
	ctx = slogx.ContextWithAttrs(ctx, "key3", 3)
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"Some message" key1=value1 g1.key2=value2 g1.g2.key3=3`)
}

func TestLaxCtxHandler(tt *testing.T) {
	t := check.T(tt)

	var buf bytes.Buffer
	ctx := context.Background()
	h := slog.NewTextHandler(&buf, nil).WithAttrs([]slog.Attr{slog.String("key1", "value1")})
	slogx.SetDefaultCtxHandler(context.Background(), h)
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `level=INFO msg="Some message" key1=value1 !BADCTX=context.Background`)

	buf.Reset()
	slogx.SetDefaultCtxHandler(context.Background(), h, slogx.LaxCtxHandler())
	slog.InfoContext(ctx, "Some message")
	t.NotMatch(buf.String(), "!BADCTX")
}
