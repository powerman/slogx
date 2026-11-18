package slogx_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/powerman/slogx"
)

// sinkRecord prevents the compiler from dropping the benchmarked result.
var sinkRecord slog.Record

func BenchmarkGroupOrAttrs_Record(b *testing.B) {
	two := []slog.Attr{slog.Int("a", 1), slog.String("b", "x")}
	twoMore := []slog.Attr{slog.Int("c", 3), slog.String("d", "y")}
	six := []slog.Attr{
		slog.Int("a", 1), slog.Int("b", 2), slog.Int("c", 3),
		slog.Int("d", 4), slog.Int("e", 5), slog.Int("f", 6),
	}

	chain := func(n int) *slogx.GroupOrAttrs {
		var goa *slogx.GroupOrAttrs
		for range n {
			goa = goa.WithAttrs([]slog.Attr{slog.Int("a", 1)})
		}
		return goa
	}

	goas := []struct {
		name string
		goa  *slogx.GroupOrAttrs
	}{
		{"nil", nil},
		{"2 attrs", new(slogx.GroupOrAttrs).WithAttrs(two)},
		{"2+2 attrs", new(slogx.GroupOrAttrs).WithAttrs(two).WithAttrs(twoMore)},
		{"6 attrs", new(slogx.GroupOrAttrs).WithAttrs(six)},
		{"8x1 attrs", chain(8)},
		{"20x1 attrs", chain(20)},
		{"group+2 attrs", new(slogx.GroupOrAttrs).WithGroup("g").WithAttrs(two)},
		{"attrs+group+attrs", new(slogx.GroupOrAttrs).WithAttrs(two).WithGroup("g").WithAttrs(twoMore)},
		{"2 groups", new(slogx.GroupOrAttrs).WithGroup("g1").WithGroup("g2")},
	}
	recAttrs := []struct {
		name  string
		attrs []slog.Attr
	}{
		{"r0", nil},
		{"r1", []slog.Attr{slog.Int("r", 1)}},
		{"r5", []slog.Attr{
			slog.Int("r1", 1), slog.Int("r2", 2), slog.Int("r3", 3),
			slog.Int("r4", 4), slog.Int("r5", 5),
		}},
	}

	for _, g := range goas {
		for _, ra := range recAttrs {
			r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
			r.AddAttrs(ra.attrs...)
			b.Run(g.name+"/"+ra.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					sinkRecord = g.goa.Record(r)
				}
			})
		}
	}
}
