package slogx_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestContextHandler(tt *testing.T) {
	t := check.T(tt)

	t.Nil(slogx.HandlerFromContext(context.Background()))

	ctx := slogx.NewContextWithHandler(context.Background(), nil)
	t.Equal(slogx.HandlerFromContext(ctx), nil)

	handler := slog.NewTextHandler(os.Stdout, nil)
	ctx = slogx.NewContextWithHandler(context.Background(), handler)
	t.Equal(slogx.HandlerFromContext(ctx), handler)
}
