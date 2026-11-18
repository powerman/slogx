package slogx_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/powerman/check"
	slogmulti "github.com/samber/slog-multi"
	"go.uber.org/mock/gomock"

	"github.com/powerman/slogx"
)

func TestWrapHandlerConfig_Resolve(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt)

	cfg := slogx.WrapHandlerConfig{ProxyWith: true}
	got, err := cfg.Resolve()
	t.Nil(err)
	t.DeepEqual(got, cfg)
}

func TestWrapHandler(t *testing.T) {
	t.Parallel()

	enabledProxy := func(ctx context.Context, l slog.Level, _ *slogx.GroupOrAttrs, next slog.Handler) bool {
		return next.Enabled(ctx, l)
	}
	handleProxy := func(ctx context.Context, r slog.Record, goa *slogx.GroupOrAttrs, next slog.Handler) error {
		return next.Handle(ctx, goa.Record(r))
	}

	tests := []slogx.WrapHandlerConfig{
		{ProxyWith: true}, // Transparent proxy.
		{ProxyWith: true, ProxyWithAttrs: true},
		{ProxyWith: true, Enabled: enabledProxy},
		{ProxyWith: true, Handle: handleProxy},
		{ProxyWith: true, Enabled: enabledProxy, Handle: handleProxy},
		{ProxyWithAttrs: true},
		{ProxyWithAttrs: true, Enabled: enabledProxy},
		{ProxyWithAttrs: true, Handle: handleProxy},
		{ProxyWithAttrs: true, Enabled: enabledProxy, Handle: handleProxy},
		{Enabled: enabledProxy},
		{Handle: handleProxy},
		{Enabled: enabledProxy, Handle: handleProxy},
		{}, // Collect all WithAttrs and WithGroup calls and apply them to each Record.
	}
	for _, tc := range tests {
		t.Run(fmt.Sprint(tc), func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			var buf bytes.Buffer
			next := slog.NewTextHandler(&buf, nil)
			h := slogx.NewWrapHandler(next, tc)
			t.Nil(slogtest.TestHandler(h, makeTextResults(t, &buf)))
		})
	}
}

// uncomparableHandler has a slice field, so comparing two of its values with == panics.
type uncomparableHandler struct {
	attrs []slog.Attr
	out   *[]string
}

func (uncomparableHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h uncomparableHandler) Handle(_ context.Context, r slog.Record) error {
	*h.out = append(*h.out, fmt.Sprintf("%s attrs=%d", r.Message, len(h.attrs)))
	return nil
}

func (h uncomparableHandler) WithAttrs(as []slog.Attr) slog.Handler {
	h.attrs = append(slices.Clone(h.attrs), as...)
	return h
}

func (h uncomparableHandler) WithGroup(string) slog.Handler { return h }

// comparablePtrHandler is comparable, but derives uncomparableHandler.
type comparablePtrHandler struct{ out *[]string }

func (*comparablePtrHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *comparablePtrHandler) Handle(ctx context.Context, r slog.Record) error {
	return uncomparableHandler{out: h.out}.Handle(ctx, r)
}

func (h *comparablePtrHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return uncomparableHandler{out: h.out}.WithAttrs(as)
}

func (h *comparablePtrHandler) WithGroup(string) slog.Handler {
	return uncomparableHandler{out: h.out}
}

// ifaceFieldHandler has a comparable type, but == on it panics
// because its interface field holds an uncomparable dynamic value.
type ifaceFieldHandler struct {
	v   any
	n   int
	out *[]string
}

func (ifaceFieldHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h ifaceFieldHandler) Handle(_ context.Context, r slog.Record) error {
	*h.out = append(*h.out, fmt.Sprintf("%s attrs=%d", r.Message, h.n))
	return nil
}

func (h ifaceFieldHandler) WithAttrs(as []slog.Attr) slog.Handler {
	h.n += len(as)
	return h
}

func (h ifaceFieldHandler) WithGroup(string) slog.Handler { return h }

// zeroArrayHandler is uncomparable because of a zero-length array of uncomparable structs,
// but reflect.Value.Comparable reports it as comparable.
type zeroArrayHandler struct {
	_   [0]struct{ _ func() }
	n   int
	out *[]string
}

func (zeroArrayHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h zeroArrayHandler) Handle(_ context.Context, r slog.Record) error {
	*h.out = append(*h.out, fmt.Sprintf("%s attrs=%d", r.Message, h.n))
	return nil
}

func (h zeroArrayHandler) WithAttrs(as []slog.Attr) slog.Handler {
	h.n += len(as)
	return h
}

func (h zeroArrayHandler) WithGroup(string) slog.Handler { return h }

func TestWrapHandler_UncomparableNext(tt *testing.T) {
	tt.Parallel()

	nexts := map[string]func(out *[]string) slog.Handler{
		"slice field":     func(out *[]string) slog.Handler { return uncomparableHandler{out: out} },
		"changes type":    func(out *[]string) slog.Handler { return &comparablePtrHandler{out: out} },
		"interface field": func(out *[]string) slog.Handler { return ifaceFieldHandler{v: []int{}, out: out} },
		"zero array":      func(out *[]string) slog.Handler { return zeroArrayHandler{out: out} },
	}
	cfgs := map[string]slogx.WrapHandlerConfig{
		"ProxyWith":      {ProxyWith: true},
		"ProxyWithAttrs": {ProxyWithAttrs: true},
	}
	for nextName, newNext := range nexts {
		for cfgName, cfg := range cfgs {
			tt.Run(nextName+"/"+cfgName, func(tt *testing.T) {
				tt.Parallel()
				t := check.Must(tt)
				var out []string
				h := slogx.NewWrapHandler(newNext(&out), cfg)

				attrs := []slog.Attr{slog.Int("a", 1)}
				h2 := h.WithAttrs(attrs).WithAttrs(attrs)
				if cfg.ProxyWith {
					h2 = h2.WithGroup("g").WithGroup("g")
				}
				t.Nil(h2.Handle(tt.Context(), slog.NewRecord(time.Time{}, slog.LevelInfo, "msg", 0)))
				t.DeepEqual(out, []string{"msg attrs=2"})
			})
		}
	}
}

func TestWrapHandler_With(tt *testing.T) {
	tt.Parallel()

	attrs := []slog.Attr{slog.Int("a", 1)}
	emptyGroups := []slog.Attr{slog.Group("empty")}

	tt.Run("collect", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		// Mock without expectations fails the test on any call.
		h := slogx.NewWrapHandler(NewMockHandler(gomock.NewController(tt)), slogx.WrapHandlerConfig{})

		t.Equal(h.WithAttrs(nil), slog.Handler(h))
		t.Equal(h.WithAttrs(emptyGroups), slog.Handler(h))
		t.Equal(h.WithGroup(""), slog.Handler(h))
	})

	cfgs := map[string]slogx.WrapHandlerConfig{
		"ProxyWith":      {ProxyWith: true},
		"ProxyWithAttrs": {ProxyWithAttrs: true},
	}
	for name, cfg := range cfgs {
		tt.Run(name+"/same next", func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			next := NewMockHandler(gomock.NewController(tt))
			next.EXPECT().WithAttrs(attrs).Return(next)
			if cfg.ProxyWith {
				next.EXPECT().WithGroup("g").Return(next)
				next.EXPECT().WithGroup("").Return(next)
			}
			h := slogx.NewWrapHandler(next, cfg)

			t.Equal(h.WithAttrs(attrs), slog.Handler(h))
			if cfg.ProxyWith {
				t.Equal(h.WithGroup("g"), slog.Handler(h))
				t.Equal(h.WithGroup(""), slog.Handler(h))
			}
		})
		tt.Run(name+"/new next", func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			ctrl := gomock.NewController(tt)
			next := NewMockHandler(ctrl)
			h := slogx.NewWrapHandler(next, cfg)
			wantWraps := func(h2 slog.Handler, next2 *MockHandler) {
				tt.Helper()
				next2.EXPECT().Enabled(gomock.Any(), slog.LevelInfo).Return(true)
				t.True(h2.Enabled(tt.Context(), slog.LevelInfo))
			}

			// Without a contract for empty attrs, the handler returned by next must be used.
			for _, as := range [][]slog.Attr{nil, emptyGroups} {
				next2 := NewMockHandler(ctrl)
				next.EXPECT().WithAttrs(as).Return(next2)
				wantWraps(h.WithAttrs(as), next2)
			}
			if cfg.ProxyWith {
				// A transparent proxy does not hide a violation of the slog.Handler contract.
				next2 := NewMockHandler(ctrl)
				next.EXPECT().WithGroup("").Return(next2)
				wantWraps(h.WithGroup(""), next2)
			}
		})
	}
}

func TestWrapMiddleware(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer
	enabledProxy := func(_ context.Context, _ slog.Level, _ *slogx.GroupOrAttrs, _ slog.Handler) bool {
		return true
	}
	handleProxy := func(ctx context.Context, r slog.Record, goa *slogx.GroupOrAttrs, next slog.Handler) error {
		r = goa.Record(r)
		r.AddAttrs(slog.Bool("middleware", true))
		return next.Handle(ctx, r)
	}

	log := slog.New(slogmulti.
		Pipe(slogx.NewWrapMiddleware(slogx.WrapHandlerConfig{
			Enabled: enabledProxy,
			Handle:  handleProxy,
		})).
		Handler(slog.NewTextHandler(&buf, nil)))
	log.With("a", 1).WithGroup("g").Debug("Test", "b", 2)
	t.Match(buf.String(), `level=DEBUG msg=Test a=1 g.b=2 middleware=true`)
}
