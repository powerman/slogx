// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.

package benchmarks

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/powerman/slogx/internal"
	"github.com/powerman/slogx/internal/race"
)

// We pass Attrs inline because it affects allocations: building
// up a list outside of the benchmarked code and passing it in with "..."
// reduces measured allocations.

func BenchmarkAttrs(b *testing.B) {
	ctx := context.Background()
	noopOpts := &internal.LayoutHandlerOptions{
		Format: map[string]internal.AttrFormat{
			"string":   {Prefix: " string=", MaxWidth: -1},
			"status":   {Prefix: " status=", MaxWidth: -1},
			"duration": {Prefix: " duration=", MaxWidth: -1},
			"time":     {Prefix: " time=", MaxWidth: -1},
			"error":    {Prefix: " error=", MaxWidth: -1},
		},
	}
	disabledOpts := &internal.LayoutHandlerOptions{
		Format: map[string]internal.AttrFormat{
			"string":   {},
			"status":   {},
			"duration": {},
			"time":     {},
			"error":    {},
		},
	}
	for _, handler := range []struct {
		name     string
		h        slog.Handler
		skipRace bool
	}{
		{"disabled", disabledHandler{}, false},
		{"async discard", newAsyncHandler(), true},
		{"fastText discard", newFastTextHandler(io.Discard), false},
		{"Text discard", slog.NewTextHandler(io.Discard, nil), false},
		{"xText", internal.NewTextHandler(io.Discard, nil), false},
		{"xLayout noop", internal.NewLayoutHandler(io.Discard, noopOpts), false},
		{"xLayout disabled", internal.NewLayoutHandler(io.Discard, disabledOpts), false},
		{"JSON discard", slog.NewJSONHandler(io.Discard, nil), false},
	} {
		logger := slog.New(handler.h)
		b.Run(handler.name, func(b *testing.B) {
			if handler.skipRace && race.Enabled {
				b.Skip("skipping benchmark in race mode")
			}
			for _, call := range []struct {
				name string
				f    func()
			}{
				{
					// The number should match nAttrsInline in slog/record.go.
					// This should exercise the code path where no allocations
					// happen in Record or Attr. If there are allocations, they
					// should only be from Duration.String and Time.String.
					"5 args",
					func() {
						logger.LogAttrs(nil, slog.LevelInfo, testMessage,
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
						)
					},
				},
				{
					"5 args ctx",
					func() {
						logger.LogAttrs(ctx, slog.LevelInfo, testMessage,
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
						)
					},
				},
				{
					"10 args",
					func() {
						logger.LogAttrs(nil, slog.LevelInfo, testMessage,
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
						)
					},
				},
				{
					// Try an extreme value to see if the results are reasonable.
					"40 args",
					func() {
						logger.LogAttrs(nil, slog.LevelInfo, testMessage,
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
							slog.String("string", testString),
							slog.Int("status", testInt),
							slog.Duration("duration", testDuration),
							slog.Time("time", testTime),
							slog.Any("error", testError),
						)
					},
				},
			} {
				b.Run(call.name, func(b *testing.B) {
					b.ReportAllocs()
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							call.f()
						}
					})
				})
			}
		})
	}
}
