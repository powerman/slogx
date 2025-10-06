package slogx_test

import (
	"bytes"
	"log/slog"
	"os"
	"testing"
	"testing/slogtest"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestContextHandler(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer
	_, h := slogx.NewContextHandler(t.Context(), slog.NewTextHandler(&buf, nil))
	t.Nil(slogtest.TestHandler(h, makeTextResults(t, &buf)))
}

func TestContextHandler_Enabled(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	h := slog.NewTextHandler(os.Stdout, nil)
	ctx := slogx.SetDefaultContextHandler(t.Context(), h)
	t.False(slog.Default().Enabled(t.Context(), slog.LevelDebug))
	t.True(slog.Default().Enabled(t.Context(), slog.LevelInfo))
	t.False(slog.Default().Enabled(ctx, slog.LevelDebug))
	t.True(slog.Default().Enabled(ctx, slog.LevelInfo))

	h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	ctx, _ = slogx.NewContextHandler(t.Context(), h)
	t.False(slog.Default().Enabled(t.Context(), slog.LevelDebug))
	t.True(slog.Default().Enabled(t.Context(), slog.LevelInfo))
	t.False(slog.Default().Enabled(ctx, slog.LevelWarn))
	t.True(slog.Default().Enabled(ctx, slog.LevelError))
}

func TestContextHandler_Smoke(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	var h slog.Handler

	// With TextHandler
	slogx.SetDefaultContextHandler(t.Context(), slog.NewTextHandler(&buf, nil))
	h = slog.NewTextHandler(&buf, nil).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key1", "value1"), slog.String("key2", "value2")})
	ctx, _ := slogx.NewContextHandler(t.Context(), h)
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
	slogx.SetDefaultContextHandler(t.Context(), slog.NewJSONHandler(&buf, nil))
	h = slog.NewJSONHandler(&buf, nil).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key1", "value1"), slog.String("key2", "value2")})
	ctx, _ = slogx.NewContextHandler(t.Context(), h)
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
	ctx := slogx.SetDefaultContextHandler(t.Context(), slog.NewTextHandler(&buf, nil))

	ctx = slogx.ContextWith(ctx, "k1", "v1", "k2", 2)
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"Some message" k1=v1 k2=2\n$`)

	buf.Reset()
	ctx = slogx.ContextWithGroup(ctx, "g1")
	ctx = slogx.ContextWith(ctx, slog.String("k3", "v3"), "k4", 4)
	slog.InfoContext(ctx, "Some message", "a", 42)
	t.Match(buf.String(), `"Some message" k1=v1 k2=2 g1.k3=v3 g1.k4=4 g1.a=42\n$`)

	buf.Reset()
	ctx = slogx.ContextWithGroup(ctx, "g2")
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"Some message" k1=v1 k2=2 g1.k3=v3 g1.k4=4\n$`)
	slog.InfoContext(ctx, "Some message", "a", 42)
	t.Match(buf.String(), `"Some message" k1=v1 k2=2 g1.k3=v3 g1.k4=4 g1.g2.a=42\n$`)
	ctx = slogx.ContextWithAttrs(ctx, slog.String("k5", "v5"), slog.Int("k6", 6))
	slog.InfoContext(ctx, "Some message")
	t.Match(buf.String(), `"Some message" k1=v1 k2=2 g1.k3=v3 g1.k4=4 g1.g2.k5=v5 g1.g2.k6=6\n$`)
}

func TestLaxContextHandler(tt *testing.T) {
	t := check.T(tt)

	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, nil).WithAttrs([]slog.Attr{slog.String("key1", "value1")})
	slogx.SetDefaultContextHandler(t.Context(), h)
	slog.InfoContext(t.Context(), "Some message")
	t.Match(buf.String(), `level=INFO msg="Some message" key1=value1 !BADCTX=context.Background`)

	buf.Reset()
	slogx.SetDefaultContextHandler(t.Context(), h, slogx.LaxContextHandler())
	slog.InfoContext(t.Context(), "Some message")
	t.NotMatch(buf.String(), "!BADCTX")
}
