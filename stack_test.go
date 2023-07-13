package slogx_test

import (
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestStack(tt *testing.T) {
	t := check.T(tt)

	stack := slogx.Stack()
	t.DeepEqual(stack.Key, slogx.KeyStack)
	t.Match(stack.Value, "github.com/powerman/slogx_test.TestStack")
	t.Match(stack.Value, "/slogx/stack_test.go:14")
	t.Match(stack.Value, "testing.tRunner")
	t.Match(stack.Value, "/testing/testing.go:1595")
	t.Match(stack.Value, "created by testing.(.*).Run in goroutine 1")
	t.Match(stack.Value, "/testing/testing.go:1648")
}
