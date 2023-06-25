package slogx_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestContext(tt *testing.T) {
	t := check.T(tt)

	t.Nil(slogx.FromContext(context.Background()))

	var log *slog.Logger
	ctx := slogx.NewContext(context.Background(), nil)
	t.Equal(slogx.FromContext(ctx), log)

	log = slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx = slogx.NewContext(context.Background(), log)
	t.Equal(slogx.FromContext(ctx), log)
}
