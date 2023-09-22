package slogx_test

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestColumnarHandlerEnabled(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	ctx := context.Background()
	var buf bytes.Buffer

	ch := slogx.NewColumnarHandler(&buf, nil)
	t.True(ch.Enabled(ctx, slog.LevelWarn))
	t.True(ch.Enabled(ctx, slog.LevelError))
	t.False(ch.Enabled(ctx, slog.LevelDebug))

	ch = slogx.NewColumnarHandler(&buf, &slogx.ColumnarHandlerOption{
		HandlerOptions: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	})
	t.True(ch.Enabled(ctx, slog.LevelDebug))
}

func TestColumnarHandlerPackage(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	offset := 1
	var pcs [1]uintptr
	runtime.Callers(offset, pcs[:])

	var (
		ctx = context.Background()
		buf bytes.Buffer

		rPC   = slog.NewRecord(time.Now(), slog.LevelInfo, "message", pcs[0])
		rNoPC = slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0)

		chNoOpts     = slogx.NewColumnarHandler(&buf, nil)
		chAddPackage = slogx.NewColumnarHandler(&buf, &slogx.ColumnarHandlerOption{
			AddPackage: true,
		})
		chAddPackageModGroup = slogx.NewColumnarHandler(&buf, &slogx.ColumnarHandlerOption{
			AddPackage: true,
			ModPackage: map[string]string{
				"github.com/powerman/...": "pkg",
			},
		})
		chAddPackageModPackage = slogx.NewColumnarHandler(&buf, &slogx.ColumnarHandlerOption{
			AddPackage: true,
			ModPackage: map[string]string{
				"github.com/powerman/slogx_test": "test",
			},
		})
		chModPackage = slogx.NewColumnarHandler(&buf, &slogx.ColumnarHandlerOption{
			AddPackage: false,
			ModPackage: map[string]string{
				"github.com/powerman/slogx_test": "test",
			},
		})
	)

	tests := []struct {
		ch   *slogx.ColumnarHandler
		want string
	}{
		{chNoOpts, "level=INFO msg=message\n"},
		{chAddPackage, "level=INFO msg=message package=slogx_test"},
		{chAddPackageModGroup, "level=INFO msg=message package=pkg"},
		{chAddPackageModPackage, "level=INFO msg=message package=test"},
		{chModPackage, "level=INFO msg=message\n"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(tt *testing.T) {
			t := check.T(tt).MustAll()

			buf.Reset()
			tc.ch.Handle(ctx, rPC)
			t.Match(buf.String(), tc.want)
		})
	}

	buf.Reset()
	chAddPackageModPackage.Handle(ctx, rNoPC)
	t.Match(buf.String(), `level=INFO msg=message package=""`)
}

func TestColumnarHandlerPrefixSuffix(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	ctx := context.Background()
	var buf bytes.Buffer

	offset := 1
	var pcs [1]uintptr
	runtime.Callers(offset, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "message", pcs[0])
	ch := slogx.NewColumnarHandler(&buf, &slogx.ColumnarHandlerOption{
		AddPackage: true,
		ModPackage: map[string]string{
			"github.com/powerman/...": "pkg",
		},
		HandlerOptions: slog.HandlerOptions{
			ReplaceAttr: func(groupe []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		},
	})
	h := ch.WithAttrs([]slog.Attr{slog.String("key1", "value1")}).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key2", "value2")})
	_ = h.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message package=pkg key1=value1 g.key2=value2\n")

	buf.Reset()
	ch = h.(*slogx.ColumnarHandler).SetAttrsPrefix([]slog.Attr{slog.String("prefixKey1", "prefixValue1")})
	ch = ch.AppendAttrsPrefix([]slog.Attr{slog.Int("prefixKey2", 2)})
	ch = ch.AppendAttrsPrefix([]slog.Attr{slog.Int("prefixKey3", 3)})
	_ = ch.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message package=pkg prefixKey1=prefixValue1 prefixKey2=2 prefixKey3=3 key1=value1 g.key2=value2\n")
	buf.Reset()
	ch = ch.SetAttrsPrefix([]slog.Attr{slog.Int("prefixKey3", 3)})
	_ = ch.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message package=pkg prefixKey3=3 key1=value1 g.key2=value2\n")
	chPrefix := ch

	buf.Reset()
	ch = ch.SetAttrsSufix([]slog.Attr{slog.String("suffixKey1", "suffixValue1"), slog.String("suffixKey2", "suffixValue2")})
	ch = ch.PrependAttrsSufix([]slog.Attr{slog.Int("suffixKey3", 3), slog.Int("suffixKey4", 4)})
	ch = ch.PrependAttrsSufix([]slog.Attr{slog.Int("suffixKey5", 5)})
	_ = ch.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message package=pkg prefixKey3=3 key1=value1 g.key2=value2 g.suffixKey5=5 g.suffixKey3=3 g.suffixKey4=4 g.suffixKey1=suffixValue1 g.suffixKey2=suffixValue2\n")
	buf.Reset()
	ch = ch.SetAttrsSufix([]slog.Attr{slog.String("suffixKey6", "suffixValue6")})
	_ = ch.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message package=pkg prefixKey3=3 key1=value1 g.key2=value2 g.suffixKey6=suffixValue6\n")

	buf.Reset()
	_ = chPrefix.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message package=pkg prefixKey3=3 key1=value1 g.key2=value2\n")
}

func TestColumnarHandlerFormat(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var buf bytes.Buffer
	ctx := context.Background()
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0)

	ch := slogx.NewColumnarHandler(&buf, &slogx.ColumnarHandlerOption{
		HandlerOptions: slog.HandlerOptions{
			ReplaceAttr: func(groupe []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		},
	})
	ch = ch.SetAttrsPrefix([]slog.Attr{slog.String("prefixKey1", "prefixValue1"), slog.String("prefixKey2", "prefixValue2")})
	ch = ch.SetAttrsSufix([]slog.Attr{slog.String("suffixKey1", "suffixValue1"), slog.String("suffixKey2", "suffixValue2")})
	ch = (ch.WithAttrs([]slog.Attr{slog.String("key1", "value1")}).WithGroup("g").WithAttrs([]slog.Attr{slog.String("key2", "value2")})).(*slogx.ColumnarHandler)
	_ = ch.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message prefixKey1=prefixValue1 prefixKey2=prefixValue2 key1=value1 g.key2=value2 g.suffixKey1=suffixValue1 g.suffixKey2=suffixValue2\n")

	ch = ch.SetAttrsFormat(map[string]string{
		"key1":       "_%v_",
		"key2":       "_%v_",
		"prefixKey1": "%v:",
		"suffixKey2": ":%v",
	})

	buf.Reset()
	_ = ch.Handle(ctx, r)
	t.Equal(buf.String(), "level=INFO msg=message prefixKey1=prefixValue1: prefixKey2=prefixValue2 key1=_value1_ g.key2=_value2_ g.suffixKey1=suffixValue1 g.suffixKey2=:suffixValue2\n")
}
