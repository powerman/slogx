package slogx_test

import (
	"context"
	"os"
	"testing"

	"github.com/powerman/check"
	"golang.org/x/exp/slog"

	"github.com/powerman/slogx"
)

func TestContext(tt *testing.T) {
	t := check.T(tt)

	t.Nil(slogx.FromContext(context.Background()))

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := slogx.NewContext(context.Background(), log)
	t.Equal(slogx.FromContext(ctx), log)
}
