package slogx_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"
	"testing/slogtest"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestWrapHandler(tt *testing.T) {
	t := check.T(tt)
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
			t := check.T(tt)
			t.Parallel()
			var buf bytes.Buffer
			next := slog.NewTextHandler(&buf, nil)
			h := slogx.NewWrapHandler(next, tc)
			t.Nil(slogtest.TestHandler(h, makeTextResults(t, &buf)))
		})
	}
}
