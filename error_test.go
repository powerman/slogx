package slogx_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func removeTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.Attr{}
	}
	return a
}

func newLog(fs ...func([]string, slog.Attr) slog.Attr) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: slogx.ChainReplaceAttr(append(fs, removeTime)...),
	}))
	return log, &buf
}

func TestErrorAttrs_NewError(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var (
		log, buf = newLog(slogx.ErrorAttrs())
		args     = []any{"k1", "v1", slog.String("k2", "v2")}
		attrs    = []slog.Attr{slog.Int("k3", 3), slog.Int("k4", 4)}
		err1     = slogx.NewError(io.EOF)
		err2     = slogx.NewErrorAttrs(io.EOF)
		err3     = slogx.NewError(io.EOF, args...)
		err4     = slogx.NewErrorAttrs(io.EOF, attrs...)
	)

	t.Nil(slogx.NewError(nil))
	t.Nil(slogx.NewError(nil, args...))
	t.Nil(slogx.NewErrorAttrs(nil))
	t.Nil(slogx.NewErrorAttrs(nil, attrs...))

	tests := []struct {
		err  error
		want string
	}{
		{err1, "err=EOF"},
		{err2, "err=EOF"},
		{err3, "k1=v1 k2=v2 err=EOF"},
		{err4, "k3=3 k4=4 err=EOF"},
	}
	for _, tc := range tests {
		t.Run("", func(tt *testing.T) {
			t := check.T(tt)

			buf.Reset()
			log.Info("Msg", "err", tc.err)
			t.Equal(buf.String(), "level=INFO msg=Msg "+tc.want+"\n")

			t.Equal(tc.err.Error(), io.EOF.Error())
			t.Equal(errors.Unwrap(tc.err), io.EOF)
		})
	}
}

func TestErrorAttrs_ReturnOriginal(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	tests := []struct {
		attr slog.Attr
	}{
		{slog.Int("not_any", 1)},
		{slog.Any("not_error", 2)},
		{slog.Any("no_attrs", io.EOF)},
		{slog.Any("empty_args", slogx.NewError(io.EOF))},
		{slog.Any("empty_attrs", slogx.NewErrorAttrs(io.EOF))},
	}
	for _, tc := range tests {
		t.Run(tc.attr.Key, func(tt *testing.T) {
			t := check.T(tt)
			attr := slogx.ErrorAttrs()(nil, tc.attr)
			t.True(tc.attr.Equal(attr))
		})
	}
}

func TestErrorAttrs_ExpandOnce(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	log, buf := newLog(slogx.ErrorAttrs(), slogx.ErrorAttrs())

	log.Info("Msg", "err", slogx.NewError(io.EOF, "k", "v"))
	t.Equal(buf.String(), "level=INFO msg=Msg k=v err=EOF\n")
}

func TestErrorAttrs_Wrapped(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var (
		log, buf = newLog(slogx.ErrorAttrs())
		err1     = slogx.NewError(io.EOF, "k1", 1, "k2", 2)
		err2     = fmt.Errorf("wrap2: %w", err1)
		err3     = slogx.NewError(err2, "k3", 3, "k4", 4)
		err4     = fmt.Errorf("wrap4: %w", err3)
	)

	tests := []struct {
		err  error
		want string
	}{
		{err1, `k1=1 k2=2 err=EOF`},
		{err2, `k1=1 k2=2 err="wrap2: EOF"`},
		{err3, `k3=3 k4=4 k1=1 k2=2 err="wrap2: EOF"`},
		{err4, `k3=3 k4=4 k1=1 k2=2 err="wrap4: wrap2: EOF"`},
	}
	for _, tc := range tests {
		t.Run("", func(tt *testing.T) {
			t := check.T(tt)

			buf.Reset()
			log.Info("Msg", "err", tc.err)
			t.Equal(buf.String(), "level=INFO msg=Msg "+tc.want+"\n")
		})
	}
}

func TestErrorAttrs_Group(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	err := slogx.NewError(io.EOF, "k1", 1)

	tests := []struct {
		opts []slogx.ErrorAttrsOption
		want string
	}{
		{
			[]slogx.ErrorAttrsOption{},
			"k3=3 k1=1 err=EOF sub.k2=2 sub.err.k1=1 sub.err.err=EOF",
		},
		{
			[]slogx.ErrorAttrsOption{slogx.GroupTopErrorAttrs()},
			"k3=3 err.k1=1 err.err=EOF sub.k2=2 sub.err.k1=1 sub.err.err=EOF",
		},
		{
			[]slogx.ErrorAttrsOption{slogx.InlineSubErrorAttrs()},
			"k3=3 k1=1 err=EOF sub.k2=2 sub.k1=1 sub.err=EOF",
		},
		{
			[]slogx.ErrorAttrsOption{slogx.GroupTopErrorAttrs(), slogx.InlineSubErrorAttrs()},
			"k3=3 err.k1=1 err.err=EOF sub.k2=2 sub.k1=1 sub.err=EOF",
		},
	}
	for _, tc := range tests {
		t.Run("", func(tt *testing.T) {
			t := check.T(tt)
			log, buf := newLog(slogx.ErrorAttrs(tc.opts...))
			log.Info("Msg",
				slog.Int("k3", 3), slog.Any("err", err),
				slog.Group("sub", "k2", 2, "err", err))
			t.Equal(buf.String(), "level=INFO msg=Msg "+tc.want+"\n")
		})
	}
}
