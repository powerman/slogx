package slogx_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/powerman/slogx"
)

func BenchmarkContextHandler(b *testing.B) {
	for _, next := range benchNextHandlers() {
		base, h := slogx.NewContextHandler(b.Context(), next.h)
		withCtx := slogx.ContextWith(base, "a", 1, "b", "x") // Proxied to the handler in ctx.
		for _, c := range []struct {
			name string
			ctx  context.Context //nolint:containedctx // Test table.
			log  *slog.Logger
		}{
			{"plain", base, slog.New(h)},
			{"ContextWith", withCtx, slog.New(h)},
			{"logger.With", base, slog.New(h).With("a", 1, "b", "x")}, // Collected in goa.
		} {
			b.Run(next.name+"/"+c.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					c.log.LogAttrs(c.ctx, slog.LevelInfo, "msg", slog.Int("r", 1))
				}
			})
		}
	}
}
