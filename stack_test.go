package slogx_test

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestStack(tt *testing.T) {
	t := check.T(tt)

	stack := fmt.Sprintf("%s\n", "github.com/powerman/slogx_test.TestStack.func1")
	stack += fmt.Sprintf("%s\n", "github.com/powerman/slogx_test.TestStack")
	stack += fmt.Sprintf("%s\n", "testing.tRunner")
	stack += fmt.Sprintf("%s", "runtime.goexit")

	t.DeepEqual(func() slog.Attr { return slogx.Stack() }(), slog.Attr{Key: "stack", Value: slog.StringValue(stack)})
}
