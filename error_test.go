package slogx_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func newLog(fs ...func([]string, slog.Attr) slog.Attr) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: slogx.ChainReplaceAttr(append(fs, removeTime)...),
	}))
	return log, &buf
}

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestErrorAttrs_NewError(tt *testing.T) {
	t := check.Must(tt)
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
		tt.Run("", func(tt *testing.T) {
			t := check.Must(tt)

			buf.Reset()
			log.Info("Msg", "err", tc.err)
			t.Equal(buf.String(), "level=INFO msg=Msg "+tc.want+"\n")

			t.Equal(tc.err.Error(), io.EOF.Error())
			t.Equal(errors.Unwrap(tc.err), io.EOF)
		})
	}
}

func TestErrorAttrs_ReturnOriginal(t *testing.T) {
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
			tt.Parallel()
			t := check.Must(tt)
			attr := slogx.ErrorAttrs()(nil, tc.attr)
			t.True(tc.attr.Equal(attr))
		})
	}
}

func TestErrorAttrs_ExpandOnce(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	log, buf := newLog(slogx.ErrorAttrs(), slogx.ErrorAttrs()) //nolint:gocritic // By design.

	log.Info("Msg", "err", slogx.NewError(io.EOF, "k", "v"))
	t.Equal(buf.String(), "level=INFO msg=Msg k=v err=EOF\n")
}

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestErrorAttrs_Wrapped(t *testing.T) {
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
			t := check.Must(tt)

			buf.Reset()
			log.Info("Msg", "err", tc.err)
			t.Equal(buf.String(), "level=INFO msg=Msg "+tc.want+"\n")
		})
	}
}

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestErrorAttrs_Join(t *testing.T) {
	var (
		log, buf     = newLog(slogx.ErrorAttrs())
		err1         = slogx.NewError(io.EOF, "k1", 1, "k2", 2)
		err1wrap     = fmt.Errorf("wrap1: %w", err1)
		err2         = slogx.NewError(io.ErrUnexpectedEOF, "k3", 3, "k4", 4)
		err2wrap     = fmt.Errorf("wrap2: %w", err2)
		err3         = slogx.NewError(io.ErrShortWrite, "k5", 5)
		err4         = io.ErrShortBuffer
		err12        = errors.Join(err1wrap, err2wrap)
		err12wrap    = fmt.Errorf("wrap12: %w", err12)
		err1234      = errors.Join(err12, errors.Join(err3, err4))
		err1234wrap  = slogx.NewError(fmt.Errorf("wrap1234: %w", err1234), "k6", 6)
		err12wrap34  = errors.Join(err12wrap, err3, err4)
		err1234list  = errors.Join(err1, err2, err3, err4)
		err1234stack = errors.Join(errors.Join(errors.Join(err1, err2), err3), err4)
	)

	tests := []struct {
		err  error
		want string
	}{
		{err12, `k1=1 k2=2 err="wrap1: EOF"` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"`},

		{err12wrap, `k1=1 k2=2 err="wrap12: wrap1: EOF"` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"`},
		{err1234, `k1=1 k2=2 err="wrap1: EOF"` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"` +
			` err-4.k5=5 err-4.err="short write"` +
			` err-4.err-3.err="short buffer"`},
		{err1234wrap, `k6=6 k1=1 k2=2 err="wrap1234: wrap1: EOF"` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"` +
			` err-4.k5=5 err-4.err="short write"` +
			` err-4.err-3.err="short buffer"`},
		{err12wrap34, `k1=1 k2=2 err="wrap12: wrap1: EOF"` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"` +
			` err-3.k5=5 err-3.err="short write"` +
			` err-4.err="short buffer"`},
		{err1234list, `k1=1 k2=2 err=EOF` +
			` err-2.k3=3 err-2.k4=4 err-2.err="unexpected EOF"` +
			` err-3.k5=5 err-3.err="short write"` +
			` err-4.err="short buffer"`},
		{err1234stack, `k1=1 k2=2 err=EOF` +
			` err-2.k3=3 err-2.k4=4 err-2.err="unexpected EOF"` +
			` err-3.k5=5 err-3.err="short write"` +
			` err-4.err="short buffer"`},
	}
	for _, tc := range tests {
		t.Run("", func(tt *testing.T) {
			t := check.Must(tt)

			buf.Reset()
			log.Info("Msg", "err", tc.err)
			t.Equal(buf.String(), "level=INFO msg=Msg "+tc.want+"\n")
		})
	}
}

func TestErrorAttrs_Group(t *testing.T) {
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
			tt.Parallel()
			t := check.Must(tt)
			log, buf := newLog(slogx.ErrorAttrs(tc.opts...))
			log.Info("Msg",
				slog.Int("k3", 3), slog.Any("err", err),
				slog.Group("sub", "k2", 2, "err", err))
			t.Equal(buf.String(), "level=INFO msg=Msg "+tc.want+"\n")
		})
	}
}

type errCase struct {
	err  error
	want string // Output of ErrorAttrsOf.
}

func errWrappedCases() []errCase {
	var (
		err1 = slogx.NewError(io.EOF, "k1", 1, "k2", 2)
		err2 = fmt.Errorf("wrap2: %w", err1)
		err3 = slogx.NewError(err2, "k3", 3, "k4", 4)
		err4 = fmt.Errorf("wrap4: %w", err3)
	)
	return []errCase{
		{err1, `k1=1 k2=2`},
		{err2, `k1=1 k2=2`},
		{err3, `k3=3 k4=4 k1=1 k2=2`},
		{err4, `k3=3 k4=4 k1=1 k2=2`},
	}
}

func errJoinCases() []errCase {
	var (
		err1         = slogx.NewError(io.EOF, "k1", 1, "k2", 2)
		err1wrap     = fmt.Errorf("wrap1: %w", err1)
		err2         = slogx.NewError(io.ErrUnexpectedEOF, "k3", 3, "k4", 4)
		err2wrap     = fmt.Errorf("wrap2: %w", err2)
		err3         = slogx.NewError(io.ErrShortWrite, "k5", 5)
		err4         = io.ErrShortBuffer
		err12        = errors.Join(err1wrap, err2wrap)
		err12wrap    = fmt.Errorf("wrap12: %w", err12)
		err1234      = errors.Join(err12, errors.Join(err3, err4))
		err1234wrap  = slogx.NewError(fmt.Errorf("wrap1234: %w", err1234), "k6", 6)
		err12wrap34  = errors.Join(err12wrap, err3, err4)
		err1234list  = errors.Join(err1, err2, err3, err4)
		err1234stack = errors.Join(errors.Join(errors.Join(err1, err2), err3), err4)
	)
	return []errCase{
		{err12, `k1=1 k2=2` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"`},
		{err12wrap, `k1=1 k2=2` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"`},
		{err1234, `k1=1 k2=2` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"` +
			` err-4.k5=5 err-4.err="short write"` +
			` err-4.err-3.err="short buffer"`},
		{err1234wrap, `k6=6 k1=1 k2=2` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"` +
			` err-4.k5=5 err-4.err="short write"` +
			` err-4.err-3.err="short buffer"`},
		{err12wrap34, `k1=1 k2=2` +
			` err-2.k3=3 err-2.k4=4 err-2.err="wrap2: unexpected EOF"` +
			` err-3.k5=5 err-3.err="short write"` +
			` err-4.err="short buffer"`},
		{err1234list, `k1=1 k2=2` +
			` err-2.k3=3 err-2.k4=4 err-2.err="unexpected EOF"` +
			` err-3.k5=5 err-3.err="short write"` +
			` err-4.err="short buffer"`},
		{err1234stack, `k1=1 k2=2` +
			` err-2.k3=3 err-2.k4=4 err-2.err="unexpected EOF"` +
			` err-3.k5=5 err-3.err="short write"` +
			` err-4.err="short buffer"`},
	}
}

// logErrorAttrsOf returns text of the attrs of ErrorAttrsOf,
// logged by a handler which doesn't use ErrorAttrs (attrs are already expanded).
func logErrorAttrsOf(attrs []slog.Attr) string {
	log, buf := newLog()
	log.LogAttrs(context.Background(), slog.LevelInfo, "Msg", attrs...)
	return buf.String()
}

func TestErrorAttrsOf_Wrapped(t *testing.T) {
	t.Parallel()
	for _, tc := range errWrappedCases() {
		t.Run("", func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			t.Equal(logErrorAttrsOf(slogx.ErrorAttrsOf("err", tc.err)),
				"level=INFO msg=Msg "+tc.want+"\n")
		})
	}
}

func TestErrorAttrsOf_Join(t *testing.T) {
	t.Parallel()
	for _, tc := range errJoinCases() {
		t.Run("", func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			t.Equal(logErrorAttrsOf(slogx.ErrorAttrsOf("err", tc.err)),
				"level=INFO msg=Msg "+tc.want+"\n")
		})
	}
}

func TestErrorAttrsOf_Empty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"no_attrs", io.EOF},
		{"wrapped_no_attrs", fmt.Errorf("wrap: %w", io.EOF)},
		{"empty_args", slogx.NewError(io.EOF)},
		{"empty_attrs", slogx.NewErrorAttrs(io.EOF)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			t.Len(slogx.ErrorAttrsOf("err", tc.err), 0)
			// Same condition under which ErrorAttrs returns the original attr.
			if tc.err != nil {
				attr := slog.Any("err", tc.err)
				t.True(slogx.ErrorAttrs()(nil, attr).Equal(attr))
			}
		})
	}
}

func TestErrorAttrsOf_JoinPlain(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	err := errors.Join(io.EOF, io.ErrUnexpectedEOF)
	t.Equal(logErrorAttrsOf(slogx.ErrorAttrsOf("err", err)),
		"level=INFO msg=Msg err-2.err=\"unexpected EOF\"\n")

	// ErrorAttrs is consistent with it.
	log, buf := newLog(slogx.ErrorAttrs())
	log.Info("Msg", "err", err)
	t.Equal(buf.String(), "level=INFO msg=Msg err=EOF err-2.err=\"unexpected EOF\"\n")
}

func TestErrorAttrsOf_Key(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	err := errors.Join(slogx.NewError(io.EOF, "k1", 1), io.ErrUnexpectedEOF)
	t.Equal(logErrorAttrsOf(slogx.ErrorAttrsOf("e", err)),
		"level=INFO msg=Msg k1=1 e-2.e=\"unexpected EOF\"\n")
}

func TestErrorAttrsOf_SameKeyAndText(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	// The attached attr must be kept even if it looks like the error itself.
	err := slogx.NewError(io.EOF, "err", io.EOF)
	t.Equal(logErrorAttrsOf(slogx.ErrorAttrsOf("err", err)), "level=INFO msg=Msg err=EOF\n")
}

func TestErrorAttrsOf_NoAliasing(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	// A single error with attrs: without a copy the result would be s itself.
	s := make([]slog.Attr, 1, 8)
	s[0] = slog.Int("a", 1)
	err := slogx.NewErrorAttrs(io.EOF, s...)
	want := "level=INFO msg=Msg a=1\n"

	res := slogx.ErrorAttrsOf("err", err)
	t.Equal(logErrorAttrsOf(res), want)

	res[0] = slog.Attr{}
	_ = append(res, slog.Int("c", 3))
	t.Equal(logErrorAttrsOf(slogx.ErrorAttrsOf("err", err)), want)
	t.DeepEqual(s[:cap(s)][1:], make([]slog.Attr, 7))
}

// The attrs attached to an error are shared between all users of the error,
// so getting attrs from one error concurrently must not race.
func TestErrorAttrs_NoRace(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	s := make([]slog.Attr, 1, 8)
	s[0] = slog.Int("a", 1)
	err := slogx.NewErrorAttrs(slogx.NewError(io.EOF, "x", 1), s...)
	replace := slogx.ErrorAttrs()

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				replace(nil, slog.Any("err", err))
				slogx.ErrorAttrsOf("err", err)
			}
		})
	}
	wg.Wait()

	t.DeepEqual(s[:cap(s)][1:], make([]slog.Attr, 7))
}

func TestErrorAttrsOf_ConsistentWithErrorAttrs(t *testing.T) {
	t.Parallel()

	// The attr of the error itself goes right after the attached attrs.
	topErrRe := regexp.MustCompile(` err=(?:"[^"]*"|\S+)`)

	for _, tc := range append(errWrappedCases(), errJoinCases()...) {
		t.Run("", func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)

			log, buf := newLog(slogx.ErrorAttrs())
			log.Info("Msg", "err", tc.err)
			full := buf.String()
			t.Equal(len(topErrRe.FindAllString(full, -1)), 1)

			t.Equal(logErrorAttrsOf(slogx.ErrorAttrsOf("err", tc.err)),
				topErrRe.ReplaceAllString(full, ""))
		})
	}
}

func ExampleErrorAttrsOf() {
	err := fmt.Errorf("load config: %w", slogx.NewError(io.ErrUnexpectedEOF, "file", "app.toml"))
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: removeTime}))

	// The text of the error is the message, so the error itself must not be logged again.
	log.LogAttrs(context.Background(), slog.LevelError, err.Error(), slogx.ErrorAttrsOf("err", err)...)
	// Output:
	// level=ERROR msg="load config: unexpected EOF" file=app.toml
}
