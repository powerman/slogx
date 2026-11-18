package slogx_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/slogtest"
	"testing/synctest"
	"time"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
	"github.com/powerman/slogx/internal/race"
)

const testTimeout = 10 * time.Second

// newTextHandler returns a text handler without time which writes to buf.
func newTextHandler(w io.Writer, level slog.Leveler) *slog.TextHandler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: removeTime})
}

func newJSONHandler(w io.Writer, level slog.Leveler) *slog.JSONHandler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: removeTime})
}

// withTimeout fails the test if f does not return in time.
func withTimeout(t *check.TB, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("timeout: deadlock or infinite recursion")
	}
}

func TestBootstrapHandlerConfig_Resolve(tt *testing.T) {
	tt.Parallel()
	t := check.T(tt)

	got, err := slogx.BootstrapHandlerConfig{}.Resolve()
	t.Nil(err)
	t.Equal(got.MaxQueued, 10000)
	t.Equal(got.DropReportLevel, slog.Leveler(slog.LevelWarn))
	t.False(got.NoDropReport)

	again, err := got.Resolve()
	t.Nil(err)
	t.DeepEqual(again, got)

	got, err = slogx.BootstrapHandlerConfig{NoDropReport: true}.Resolve()
	t.Nil(err)
	t.Nil(got.DropReportLevel)

	_, err = slogx.BootstrapHandlerConfig{MaxQueued: -1}.Resolve()
	t.Err(err, slogx.ErrBootstrapInvalidMaxQueued)

	got, err = slogx.BootstrapHandlerConfig{NoDropReport: true, DropReportLevel: slog.LevelError}.Resolve()
	t.Nil(err)
	t.Nil(got.DropReportLevel)

	invalid := slogx.BootstrapHandlerConfig{MaxQueued: -1}
	_, err = slogx.NewBootstrapHandler(invalid)
	t.Err(err, slogx.ErrBootstrapInvalidMaxQueued)
	t.PanicMatch(func() { slogx.MustBootstrapHandler(invalid) }, `MaxQueued`)
}

func TestBootstrapHandler_Order(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer
	level := new(slog.LevelVar)

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	log := slog.New(bootstrap)
	log.Debug("debug 1")
	log.Info("info 2")
	log.Warn("warn 3")
	log.Info("info 4")
	t.Equal(buf.String(), "")
	t.True(log.Enabled(t.Context(), slog.LevelDebug-10))

	level.Set(slog.LevelInfo)
	t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, level)))
	t.Equal(buf.String(), "level=INFO msg=\"info 2\"\nlevel=WARN msg=\"warn 3\"\nlevel=INFO msg=\"info 4\"\n")
	t.False(log.Enabled(t.Context(), slog.LevelDebug))
	t.True(log.Enabled(t.Context(), slog.LevelInfo))

	buf.Reset()
	log.Debug("debug 5")
	log.Info("info 6")
	t.Equal(buf.String(), "level=INFO msg=\"info 6\"\n")
}

func TestBootstrapHandler_Groups(tt *testing.T) {
	tt.Parallel()

	chains := map[string]func(*slog.Logger) *slog.Logger{
		"none":              func(l *slog.Logger) *slog.Logger { return l },
		"attrs":             func(l *slog.Logger) *slog.Logger { return l.With("a", 1, "b", "x") },
		"group+attrs":       func(l *slog.Logger) *slog.Logger { return l.WithGroup("g").With("a", 1) },
		"attrs+group+attrs": func(l *slog.Logger) *slog.Logger { return l.With("a", 1).WithGroup("g").With("b", 2) },
		"nested groups":     func(l *slog.Logger) *slog.Logger { return l.WithGroup("g1").With("a", 1).WithGroup("g2").With("b", 2) },
		"group only":        func(l *slog.Logger) *slog.Logger { return l.WithGroup("g") },
		"empty name":        func(l *slog.Logger) *slog.Logger { return l.WithGroup("").With("a", 1) },
		"empty groups":      func(l *slog.Logger) *slog.Logger { return l.With(slog.Group("e")).WithGroup("g").With(slog.Group("e")) },
		"group attr":        func(l *slog.Logger) *slog.Logger { return l.With(slog.Group("g", "a", 1, "b", 2)) },
		"two attrs":         func(l *slog.Logger) *slog.Logger { return l.With("a", 1).With("b", 2) },
	}
	makers := map[string]func(*bytes.Buffer) slog.Handler{
		"text": func(buf *bytes.Buffer) slog.Handler { return newTextHandler(buf, nil) },
		"json": func(buf *bytes.Buffer) slog.Handler { return newJSONHandler(buf, nil) },
	}

	for hname, mk := range makers {
		for cname, chain := range chains {
			tt.Run(hname+"/"+cname, func(tt *testing.T) {
				tt.Parallel()
				t := check.Must(tt)
				ctx := t.Context()

				var want bytes.Buffer
				chain(slog.New(mk(&want))).LogAttrs(ctx, slog.LevelInfo, "msg", slog.Int("r", 1))

				// Derived before the handoff, logged before it (replayed).
				var buf1 bytes.Buffer
				bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
				log := chain(slog.New(bootstrap))
				log.LogAttrs(ctx, slog.LevelInfo, "msg", slog.Int("r", 1))
				t.Equal(buf1.String(), "")
				t.Nil(bootstrap.HandOffTo(mk(&buf1)))
				t.Equal(buf1.String(), want.String())

				// Derived before the handoff, logged after it.
				var buf2 bytes.Buffer
				bootstrap = slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
				log = chain(slog.New(bootstrap))
				t.Nil(bootstrap.HandOffTo(mk(&buf2)))
				log.LogAttrs(ctx, slog.LevelInfo, "msg", slog.Int("r", 1))
				t.Equal(buf2.String(), want.String())

				// Derived and logged after the handoff.
				var buf3 bytes.Buffer
				bootstrap = slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
				t.Nil(bootstrap.HandOffTo(mk(&buf3)))
				chain(slog.New(bootstrap)).LogAttrs(ctx, slog.LevelInfo, "msg", slog.Int("r", 1))
				t.Equal(buf3.String(), want.String())
			})
		}
	}
}

func TestBootstrapHandler_WithAfterHandOff(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer
	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	old := bootstrap.WithAttrs([]slog.Attr{slog.Int("a", 1)})
	t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))

	for _, h := range []slog.Handler{bootstrap, old} {
		_, ok := h.WithAttrs([]slog.Attr{slog.Int("b", 2)}).(*slogx.BootstrapHandler)
		t.False(ok)
		_, ok = h.WithGroup("g").(*slogx.BootstrapHandler)
		t.False(ok)
	}
}

func TestBootstrapHandler_Slogtest(tt *testing.T) {
	tt.Parallel()

	tt.Run("handed off before", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		h := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		t.Nil(h.HandOffTo(slog.NewTextHandler(&buf, nil)))
		t.Nil(slogtest.TestHandler(h, makeTextResults(t, &buf)))
	})
	tt.Run("handed off in results", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		h := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		results := makeTextResults(t, &buf)
		t.Nil(slogtest.TestHandler(h, func() []map[string]any {
			err := h.HandOffTo(slog.NewTextHandler(&buf, nil))
			if err != nil && !errors.Is(err, slogx.ErrBootstrapped) {
				t.Fatal(err)
			}
			return results()
		}))
	})
	tt.Run("handed off after first Handle", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		var once sync.Once
		h := slogx.NewWrapHandler(bootstrap, slogx.WrapHandlerConfig{
			ProxyWith: true,
			Handle: func(ctx context.Context, r slog.Record, _ *slogx.GroupOrAttrs, next slog.Handler) error {
				err := next.Handle(ctx, r)
				once.Do(func() { t.Nil(bootstrap.HandOffTo(slog.NewTextHandler(&buf, nil))) })
				return err
			},
		})
		t.Nil(slogtest.TestHandler(h, makeTextResults(t, &buf)))
	})
}

func TestBootstrapHandler_HandOffTo(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	node := bootstrap.WithAttrs([]slog.Attr{slog.Int("a", 1)})
	t.PanicMatch(func() { _ = bootstrap.HandOffTo(nil) }, `nil Handler`)
	t.PanicMatch(func() { _ = bootstrap.HandOffTo(bootstrap) }, `itself`)
	t.PanicMatch(func() { _ = bootstrap.HandOffTo(node) }, `itself`)
	t.PanicMatch(func() { _ = node.(*slogx.BootstrapHandler).HandOffTo(bootstrap) }, `itself`)
	t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))
	t.Equal(bootstrap.HandOffTo(newTextHandler(&buf, nil)), slogx.ErrBootstrapped)
	t.Equal(node.(*slogx.BootstrapHandler).HandOffTo(newTextHandler(&buf, nil)), slogx.ErrBootstrapped)
}

// blockingHandler blocks the first Handle until unblock is closed.
type blockingHandler struct {
	slog.Handler

	entered chan struct{}
	unblock chan struct{}
	once    *sync.Once
}

func (h blockingHandler) Handle(ctx context.Context, r slog.Record) error {
	h.once.Do(func() { close(h.entered) })
	<-h.unblock
	return h.Handler.Handle(ctx, r)
}

func TestBootstrapHandler_ConcurrentHandOff(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	slog.New(bootstrap).Info("held")
	next := blockingHandler{
		Handler: newTextHandler(&buf, nil),
		entered: make(chan struct{}),
		unblock: make(chan struct{}),
		once:    new(sync.Once),
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Go(func() { errs <- bootstrap.HandOffTo(next) })
	<-next.entered // The first HandOffTo is in the middle of the replay.
	wg.Go(func() { errs <- bootstrap.HandOffTo(next) })

	select {
	case err := <-errs:
		t.Fatalf("HandOffTo returned before pass-through: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(next.unblock)
	withTimeout(t, wg.Wait)
	close(errs)

	var nils, handedOff int
	for err := range errs {
		switch {
		case err == nil:
			nils++
		case errors.Is(err, slogx.ErrBootstrapped):
			handedOff++
		}
	}
	t.Equal(nils, 1)
	t.Equal(handedOff, 1)
}

func TestBootstrapHandler_HandOffPanic(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	slog.New(bootstrap).Info("held")
	next := slogx.NewWrapHandler(slog.DiscardHandler, slogx.WrapHandlerConfig{
		Enabled: func(context.Context, slog.Level, *slogx.GroupOrAttrs, slog.Handler) bool { return true },
		Handle: func(context.Context, slog.Record, *slogx.GroupOrAttrs, slog.Handler) error {
			panic("boom")
		},
	})
	t.PanicMatch(func() { _ = bootstrap.HandOffTo(next) }, `boom`)
	// The handler stays in the handing-off phase: nobody must see it as handed off.
	t.PanicMatch(func() { _ = bootstrap.HandOffTo(next) }, `boom`)
	_, ok := bootstrap.WithGroup("g").(*slogx.BootstrapHandler)
	t.True(ok)
}

type ctxKey struct{}

// ctxHandler adds value of ctxKey to each record.
type ctxHandler struct{ slog.Handler }

func (h ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.Any("ctx", ctx.Value(ctxKey{})))
	return h.Handler.Handle(ctx, r)
}

func TestBootstrapHandler_Ctx(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	log := slog.New(bootstrap)
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), ctxKey{}, "val"))
	log.InfoContext(ctx, "first")
	cancel() // Cancellation must not affect logging.
	t.Nil(bootstrap.HandOffTo(ctxHandler{newTextHandler(&buf, nil)}))
	t.Equal(buf.String(), "level=INFO msg=first ctx=val\n")
}

func TestBootstrapHandler_EnabledBeforeHandOff(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer
	ctx := t.Context()

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	derived := bootstrap.WithAttrs([]slog.Attr{slog.Int("a", 1)})
	t.True(bootstrap.Enabled(ctx, slog.LevelDebug)) // True only because of holding.
	t.True(derived.Enabled(ctx, slog.LevelDebug))
	t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, slog.LevelInfo)))

	// A logger checked Enabled before the end of HandOffTo and logs after it.
	r := slog.NewRecord(time.Time{}, slog.LevelDebug, "debug", 0)
	t.Nil(bootstrap.Handle(ctx, r))
	t.Nil(derived.Handle(ctx, r))
	t.Equal(buf.String(), "")
}

func TestBootstrapHandler_Stack(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	slog.New(bootstrap).LogAttrs(t.Context(), slog.LevelInfo, "msg", slogx.Stack)
	t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))
	// Values are not touched until the handoff, so Stack points to HandOffTo.
	t.Match(buf.String(), `\(\*BootstrapHandler\)\.HandOffTo`)
}

func TestBootstrapHandler_ErrorStack(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	slog.New(bootstrap).LogAttrs(t.Context(), slog.LevelInfo, "msg", slogx.ErrorStack())
	t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))
	t.Match(buf.String(), `TestBootstrapHandler_ErrorStack`)
	t.NotMatch(buf.String(), `HandOffTo`)
}

// logOnce calls log on the first call only.
type logOnce struct {
	slog.Handler

	once sync.Once
	log  func()
}

func (h *logOnce) Handle(ctx context.Context, r slog.Record) error {
	h.once.Do(h.log)
	return h.Handler.Handle(ctx, r)
}

type onHandle struct {
	slog.Handler

	fn func()
}

func (h *onHandle) Handle(ctx context.Context, r slog.Record) error {
	h.fn()
	return h.Handler.Handle(ctx, r)
}

type loggingValue struct{ log func() }

func (v loggingValue) String() string { v.log(); return "value" }

func (v loggingValue) MarshalText() ([]byte, error) { v.log(); return []byte("value"), nil }

func TestBootstrapHandler_Reentrancy(tt *testing.T) {
	tt.Parallel()

	tt.Run("Handle of next logs", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		log := slog.New(bootstrap)
		next := &logOnce{Handler: newTextHandler(&buf, nil)}
		next.log = func() { log.Info("from next") }
		log.Info("first")
		withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(next)) })
		t.Equal(buf.String(), "level=INFO msg=first\nlevel=INFO msg=\"from next\"\n")
	})

	tt.Run("String of an attr logs", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		log := slog.New(bootstrap)
		var once sync.Once
		v := loggingValue{func() { once.Do(func() { log.Info("from String") }) }}
		log.Info("first", "v", v)
		withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil))) })
		t.Match(buf.String(), `msg=first v=value`)
		t.Match(buf.String(), `msg="from String"`)
		t.Equal(strings.Count(buf.String(), "\n"), 2)
	})

	for name, mk := range map[string]func(*bytes.Buffer) slog.Handler{
		"text": func(buf *bytes.Buffer) slog.Handler { return newTextHandler(buf, nil) },
		"json": func(buf *bytes.Buffer) slog.Handler { return newJSONHandler(buf, nil) },
	} {
		tt.Run("With attr which logs while derived/"+name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			var buf bytes.Buffer
			bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
			var log *slog.Logger // Logs through the node which is being derived.
			var once sync.Once
			v := loggingValue{func() { once.Do(func() { log.Info("from value") }) }}
			log = slog.New(bootstrap).With("app", v) // Next handler derives it at the handoff.
			log.Info("first")
			withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(mk(&buf))) })
			t.Match(buf.String(), `first`)
			t.Match(buf.String(), `from value`)
		})
	}

	tt.Run("derived handler is created while the handoff derives", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		var late atomic.Pointer[slog.Logger]
		var once sync.Once
		v := loggingValue{func() { once.Do(func() { late.Store(slog.New(bootstrap).With("late", 1)) }) }}
		slog.New(bootstrap).With("app", v).Info("first")
		withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil))) })

		late.Load().Info("after")
		t.Equal(buf.String(), "level=INFO msg=first app=value\nlevel=INFO msg=after late=1\n")
	})

	tt.Run("ReplaceAttr of next logs", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		log := slog.New(bootstrap)
		var once sync.Once
		next := slog.NewTextHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == "x" {
					once.Do(func() { log.Info("from ReplaceAttr") })
				}
				return removeTime(groups, a)
			},
		})
		log.Info("first", "x", 1)
		withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(next)) })
		t.Match(buf.String(), `msg=first x=1`)
		t.Match(buf.String(), `msg="from ReplaceAttr"`)
	})
}

// levelByAttrHandler lowers its level to debug if it gets attr "debug" by WithAttrs.
type levelByAttrHandler struct {
	slog.Handler

	level slog.Level
}

func (h levelByAttrHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h levelByAttrHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	level := h.level
	for _, a := range attrs {
		if a.Key == "debug" {
			level = slog.LevelDebug
		}
	}
	return levelByAttrHandler{h.Handler.WithAttrs(attrs), level}
}

func (h levelByAttrHandler) WithGroup(name string) slog.Handler {
	return levelByAttrHandler{h.Handler.WithGroup(name), h.level}
}

func TestBootstrapHandler_EnabledDependsOnWith(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer
	ctx := t.Context()

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	debug := slog.New(bootstrap.WithAttrs([]slog.Attr{slog.Bool("debug", true)}))
	plain := slog.New(bootstrap)
	debug.Debug("held debug")
	plain.Debug("held plain")

	t.Nil(bootstrap.HandOffTo(levelByAttrHandler{newTextHandler(&buf, slog.LevelDebug), slog.LevelInfo}))
	t.Equal(buf.String(), "level=DEBUG msg=\"held debug\" debug=true\n")

	buf.Reset()
	t.True(debug.Enabled(ctx, slog.LevelDebug))
	t.False(plain.Enabled(ctx, slog.LevelDebug))
	debug.Debug("late debug")
	plain.Debug("late plain")
	t.Equal(buf.String(), "level=DEBUG msg=\"late debug\" debug=true\n")
}

// countWithHandler counts calls of WithAttrs.
type countWithHandler struct {
	slog.Handler

	n *atomic.Int32
}

func (h countWithHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.n.Add(1)
	return countWithHandler{h.Handler.WithAttrs(attrs), h.n}
}

func TestBootstrapHandler_DeriveOnce(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var n atomic.Int32

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	old := slog.New(bootstrap.WithAttrs([]slog.Attr{slog.Int("a", 1)}))
	t.Nil(bootstrap.HandOffTo(countWithHandler{newTextHandler(io.Discard, nil), &n}))
	t.Equal(n.Load(), int32(1)) // Derived by HandOffTo, although nothing was logged through it.

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				old.Info("msg")
				_ = old.Handler().WithGroup("g")
			}
		})
	}
	wg.Wait()
	t.Equal(n.Load(), int32(1))
}

func TestBootstrapHandler_Limit(tt *testing.T) {
	tt.Parallel()

	tt.Run("report", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		var dropped []string
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{
			MaxQueued: 3,
			OnDrop: func(_ context.Context, r slog.Record) {
				dropped = append(dropped, r.Message)
			},
		})
		log := slog.New(bootstrap)
		for i := range 10 {
			log.Info(fmt.Sprint("msg", i))
		}
		t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))
		t.DeepEqual(dropped, []string{"msg3", "msg4", "msg5", "msg6", "msg7", "msg8", "msg9"})
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		t.Len(lines, 4)
		t.Equal(lines[0], "level=INFO msg=msg0")
		t.Equal(lines[2], "level=INFO msg=msg2")
		t.Equal(lines[3], `level=WARN msg="slogx: BootstrapHandler: dropped records" dropped=7`)
	})

	tt.Run("report level", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{
			MaxQueued:       1,
			DropReportLevel: slog.LevelError,
		})
		log := slog.New(bootstrap)
		log.Info("a")
		log.Info("b")
		t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))
		t.Match(buf.String(), `level=ERROR msg="slogx: BootstrapHandler: dropped records" dropped=1`)
	})

	tt.Run("report off", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{
			MaxQueued:    1,
			NoDropReport: true,
		})
		log := slog.New(bootstrap)
		log.Info("a")
		log.Info("b")
		t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))
		t.Equal(buf.String(), "level=INFO msg=a\n")
	})

	tt.Run("drops while handing off", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{MaxQueued: 2})
		log := slog.New(bootstrap)
		next := &logOnce{Handler: newTextHandler(&buf, nil)}
		next.log = func() {
			for i := range 5 {
				log.Info(fmt.Sprint("late", i))
			}
		}
		log.Info("first")
		withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(next)) })
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		t.DeepEqual(lines, []string{
			`level=INFO msg=first`,
			`level=INFO msg=late0`,
			`level=INFO msg=late1`,
			`level=WARN msg="slogx: BootstrapHandler: dropped records" dropped=3`,
		})
	})

	tt.Run("new queue is free once the handoff takes it", func(tt *testing.T) {
		tt.Parallel()
		t := check.Must(tt)
		var buf bytes.Buffer
		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{MaxQueued: 3})
		log := slog.New(bootstrap)
		var calls int
		next := &onHandle{Handler: newTextHandler(&buf, nil)}
		next.fn = func() {
			calls++
			if calls == 1 { // The queue is taken by the handoff: only the report is in the new one.
				for i := range 4 {
					log.Info(fmt.Sprint("x", i))
				}
			}
		}
		for i := range 5 {
			log.Info(fmt.Sprint("m", i))
		}
		withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(next)) })
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		t.DeepEqual(lines, []string{
			`level=INFO msg=m0`,
			`level=INFO msg=m1`,
			`level=INFO msg=m2`,
			`level=WARN msg="slogx: BootstrapHandler: dropped records" dropped=2`,
			`level=INFO msg=x0`,
			`level=INFO msg=x1`,
			`level=WARN msg="slogx: BootstrapHandler: dropped records" dropped=2`,
		})
	})
}

// TestBootstrapHandler_PassthroughAllocs is not parallel:
// AllocsPerRun counts allocations of the whole process.
func TestBootstrapHandler_PassthroughAllocs(tt *testing.T) { //nolint:paralleltest // See above.
	if race.Enabled {
		tt.Skip("skipping test in race, asan, and msan modes")
	}
	t := check.Must(tt)
	ctx := t.Context()
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.Int("r", 1))

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	t.Nil(bootstrap.HandOffTo(nopHandler{}))

	t.Zero(testing.AllocsPerRun(100, func() { bootstrap.Enabled(ctx, slog.LevelDebug) }))
	t.Zero(testing.AllocsPerRun(100, func() { _ = bootstrap.Handle(ctx, r) }))
}

func TestBootstrapHandler_Concurrency(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	const (
		goroutines = 8
		records    = 500
	)
	var buf bytes.Buffer
	var bufMu sync.Mutex
	next := slogx.NewWrapHandler(newTextHandler(&buf, nil), slogx.WrapHandlerConfig{
		ProxyWith: true,
		Handle: func(ctx context.Context, r slog.Record, _ *slogx.GroupOrAttrs, next slog.Handler) error {
			bufMu.Lock()
			defer bufMu.Unlock()
			return next.Handle(ctx, r)
		},
	})

	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{MaxQueued: goroutines * records * 2})
	def := slog.New(bootstrap)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := range goroutines {
		captured := def.With("g", g) // Created before the handoff, used during it.
		wg.Go(func() {
			<-start
			for i := range records {
				switch i % 4 {
				case 0, 2:
					captured.Info("m", "i", i)
				case 1:
					def.With("g", g).Info("m", "i", i) // Created while the handoff is running or after it.
				default:
					def.Info("m", "g", g, "i", i)
				}
			}
		})
	}
	close(start)
	runtime.Gosched()
	withTimeout(t, func() { t.Nil(bootstrap.HandOffTo(next)) })
	slog.New(next).Info("main")
	wg.Wait()
	// A record of main logged after HandOffTo must come after all records logged before it,
	// but writers are still running, so check only the complete picture below.

	bufMu.Lock()
	out := buf.String()
	bufMu.Unlock()
	last := make(map[int]int)
	count := 0
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "level=INFO msg=main" {
			continue
		}
		var g, i int
		_, err := fmt.Sscanf(line, "level=INFO msg=m g=%d i=%d", &g, &i)
		t.Must(t.Nil(err))
		if prev, ok := last[g]; ok {
			t.Greater(i, prev)
		}
		last[g] = i
		count++
	}
	t.Equal(count, goroutines*records)
}

func TestBootstrapHandler_MainAfterHandOff(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer
	bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	log := slog.New(bootstrap)
	for i := range 100 {
		log.Info("before", "i", i)
	}
	t.Nil(bootstrap.HandOffTo(newTextHandler(&buf, nil)))
	log.Info("after")
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	t.Len(lines, 101)
	t.Equal(lines[100], "level=INFO msg=after")
}

func TestBootstrapHandler_HandOffReentrant(tt *testing.T) {
	tt.Parallel()
	synctest.Test(tt, func(tt *testing.T) {
		t := check.Must(tt)

		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		slog.New(bootstrap).Info("held")
		next := &onHandle{Handler: newTextHandler(io.Discard, nil)}
		next.fn = func() { _ = bootstrap.HandOffTo(next) } // A deadlock.
		t.PanicMatch(func() { _ = bootstrap.HandOffTo(next) }, `possible deadlock on re-entering HandOffTo`)
		t.PanicMatch(func() { _ = bootstrap.HandOffTo(next) }, `possible deadlock on re-entering HandOffTo`)
	})
}

func TestBootstrapHandler_HandOffSlowIsNotStuck(tt *testing.T) {
	tt.Parallel()
	synctest.Test(tt, func(tt *testing.T) {
		t := check.Must(tt)

		bootstrap := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
		log := slog.New(bootstrap)
		for range 5 {
			log.Info("held")
		}
		started := make(chan struct{})
		var once sync.Once
		next := &onHandle{Handler: newTextHandler(io.Discard, nil)}
		next.fn = func() { // The whole handoff takes much longer than the stuck timeout.
			once.Do(func() { close(started) })
			time.Sleep(6 * time.Second)
		}
		first := make(chan error, 1)
		go func() { first <- bootstrap.HandOffTo(next) }()
		<-started
		t.Err(bootstrap.HandOffTo(next), slogx.ErrBootstrapped)
		t.Nil(<-first)
	})
}
