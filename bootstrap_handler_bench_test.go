package slogx_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/powerman/slogx"
)

// nopHandler is the honest baseline for a wrapper:
// a fast production handler often only hands the record to a background goroutine.
type nopHandler struct{}

func (nopHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelInfo }

func (nopHandler) Handle(context.Context, slog.Record) error { return nil }

func (h nopHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h nopHandler) WithGroup(string) slog.Handler { return h }

// BenchmarkBootstrapHandler compares a BootstrapHandler after HandOffTo with the bare real handler.
//
// Every case is a single-threaded loop of nanosecond operations, so it is stable already
// at a short -benchtime, and the whole run is sequential (it can't use more than one CPU):
//
//	go test -run '^$' -bench BenchmarkBootstrapHandler -benchtime 300ms -count 10 -benchmem
//
// The nop handler shows the overhead of the wrapper itself (Enabled and Handle),
// the text handler shows it against a realistic handler (Handle and Logger).
func BenchmarkBootstrapHandler(b *testing.B) {
	attrs := []slog.Attr{slog.Int("a", 1), slog.String("b", "x")}
	ctx := b.Context()
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.Int("r", 1))

	for _, next := range []struct {
		name string
		h    slog.Handler
	}{
		{"nop", nopHandler{}},
		{"text", slog.NewTextHandler(io.Discard, nil)},
	} {
		handedOff := func(derive bool) slog.Handler {
			bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
			var h slog.Handler = bootstrap
			if derive {
				h = h.WithAttrs(attrs) // Derived before the handoff.
			}
			err := bootstrap.HandOffTo(next.h)
			if err != nil {
				b.Fatal(err)
			}
			return h
		}
		derivedAfter := func() slog.Handler {
			bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
			err := bootstrap.HandOffTo(next.h)
			if err != nil {
				b.Fatal(err)
			}
			return bootstrap.WithAttrs(attrs) // Must equal to bare next.h.WithAttrs.
		}

		for _, c := range []struct {
			name string
			h    slog.Handler
			ref  bool // Only a reference for the previous case: it must be the same.
		}{
			{"bare", next.h, false},
			{"root", handedOff(false), false},
			{"derived before", handedOff(true), false},
			{"bare derived", next.h.WithAttrs(attrs), true},
			{"derived after", derivedAfter(), true},
		} {
			if next.name == "nop" && !c.ref {
				b.Run(next.name+"/Enabled/"+c.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						c.h.Enabled(ctx, slog.LevelDebug)
					}
				})
				b.Run(next.name+"/Handle/"+c.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						_ = c.h.Handle(ctx, r)
					}
				})
			}
			if next.name == "text" {
				b.Run(next.name+"/Handle/"+c.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						_ = c.h.Handle(ctx, r)
					}
				})
				b.Run(next.name+"/Logger/"+c.name, func(b *testing.B) {
					b.ReportAllocs()
					log := slog.New(c.h)
					for b.Loop() {
						log.LogAttrs(ctx, slog.LevelInfo, "msg", slog.Int("r", 1))
					}
				})
			}
		}
	}
}
