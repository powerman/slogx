package slogx_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/powerman/slogx"
)

type wrapBenchConfig struct {
	name string
	cfg  *slogx.WrapHandlerConfig // Nil means bare next handler.
}

type benchNextHandler struct {
	name string
	h    slog.Handler
}

// wrapBenchConfigs returns the configs compared by the WrapHandler benchmarks.
// The callbacks are the ones from the doc comment of WrapHandlerConfig.
func wrapBenchConfigs() []wrapBenchConfig {
	enabled := func(ctx context.Context, l slog.Level, _ *slogx.GroupOrAttrs, next slog.Handler) bool {
		return next.Enabled(ctx, l)
	}
	handle := func(ctx context.Context, r slog.Record, goa *slogx.GroupOrAttrs, next slog.Handler) error {
		return next.Handle(ctx, goa.Record(r))
	}
	return []wrapBenchConfig{
		{"bare", nil},
		{"ProxyWith", &slogx.WrapHandlerConfig{ProxyWith: true}},
		{"ProxyWith+callbacks", &slogx.WrapHandlerConfig{ProxyWith: true, Enabled: enabled, Handle: handle}},
		{"collect", &slogx.WrapHandlerConfig{}},
		{"collect+Enabled", &slogx.WrapHandlerConfig{Enabled: enabled}},
		{"collect+callbacks", &slogx.WrapHandlerConfig{Enabled: enabled, Handle: handle}},
		{"ProxyWithAttrs", &slogx.WrapHandlerConfig{ProxyWithAttrs: true}},
	}
}

// benchNextHandlers returns the handlers to wrap:
// the no-op one shows the overhead of a wrapper itself,
// the text one shows it against a realistic handler.
func benchNextHandlers() []benchNextHandler {
	return []benchNextHandler{
		{"nop", nopHandler{}},
		{"text", slog.NewTextHandler(io.Discard, nil)},
	}
}

// wrapBenchHandlers calls f for every config, derived with attrs and with group+attrs.
func wrapBenchHandlers(next slog.Handler, f func(name string, h slog.Handler)) {
	attrs := []slog.Attr{slog.Int("a", 1), slog.String("b", "x")}
	for _, c := range wrapBenchConfigs() {
		h := next
		if c.cfg != nil {
			h = slogx.NewWrapHandler(next, *c.cfg)
		}
		f(c.name+"/attrs", h.WithAttrs(attrs))
		f(c.name+"/group+attrs", h.WithGroup("g").WithAttrs(attrs))
	}
}

// BenchmarkWrapHandler_Enabled calls the handler directly,
// because slog.Logger adds ~170 ns (time.Now and runtime.Callers) and hides the difference.
func BenchmarkWrapHandler_Enabled(b *testing.B) {
	ctx := b.Context()
	wrapBenchHandlers(nopHandler{}, func(name string, h slog.Handler) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				h.Enabled(ctx, slog.LevelDebug)
			}
		})
	})
}

// BenchmarkWrapHandler_Handle calls the handler directly, see BenchmarkWrapHandler_Enabled.
func BenchmarkWrapHandler_Handle(b *testing.B) {
	ctx := b.Context()
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.Int("r", 1))
	for _, next := range benchNextHandlers() {
		wrapBenchHandlers(next.h, func(name string, h slog.Handler) {
			b.Run(next.name+"/"+name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_ = h.Handle(ctx, r)
				}
			})
		})
	}
}

// BenchmarkWrapHandler_Logger goes through slog.Logger, so it includes its overhead.
func BenchmarkWrapHandler_Logger(b *testing.B) {
	ctx := b.Context()
	for _, next := range benchNextHandlers() {
		wrapBenchHandlers(next.h, func(name string, h slog.Handler) {
			b.Run(next.name+"/"+name, func(b *testing.B) {
				b.ReportAllocs()
				log := slog.New(h)
				for b.Loop() {
					log.LogAttrs(ctx, slog.LevelInfo, "msg", slog.Int("r", 1))
				}
			})
		})
	}
}

// BenchmarkWrapHandler_With measures deriving a proxying WrapHandler,
// both when next returns itself and when it returns a new handler.
func BenchmarkWrapHandler_With(b *testing.B) {
	attrs := []slog.Attr{slog.Int("a", 1)}
	nexts := []benchNextHandler{
		{"nop", nopHandler{}},
		{"discard", slog.DiscardHandler},
		{"text", slog.NewTextHandler(io.Discard, nil)},
		{"uncomparable", uncomparableHandler{out: new([]string)}},
	}
	for _, next := range nexts {
		h := slogx.NewWrapHandler(next.h, slogx.WrapHandlerConfig{ProxyWith: true})
		b.Run(next.name+"/attrs", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				h.WithAttrs(attrs)
			}
		})
		b.Run(next.name+"/group", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				h.WithGroup("g")
			}
		})
	}
}
