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

	t.Nil(slogx.FromContext(context.Background()))

	ctx := slogx.NewContext(context.Background(), nil)
	t.Equal(slogx.FromContext(ctx), nil)

	handler := slog.NewTextHandler(os.Stdout, nil)
	ctx = slogx.NewContext(context.Background(), handler)
	t.Equal(slogx.FromContext(ctx), handler)
}

func TestContextLogger(tt *testing.T) {
	t := check.T(tt)

	t.Nil(slogx.LoggerFromContext(context.Background()))

	var log *slog.Logger
	ctx := slogx.NewContextWithLogger(context.Background(), nil)
	t.Equal(slogx.LoggerFromContext(ctx), log)

	log = slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx = slogx.NewContextWithLogger(context.Background(), log)
	t.Equal(slogx.LoggerFromContext(ctx), log)
}
