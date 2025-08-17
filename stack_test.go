package slogx_test

import (
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestStack(tt *testing.T) {
	t := check.T(tt)

	stack := slogx.Stack()
	t.DeepEqual(stack.Key, slogx.StackKey)
	t.HasPrefix(stack.Value, "goroutine")
	t.NotMatch(stack.Value, "github.com/powerman/slogx.Stack()")
	t.NotMatch(stack.Value, "/slogx/stack.go:")
	t.Match(stack.Value, "github.com/powerman/slogx_test.TestStack")
	t.Match(stack.Value, "/slogx/stack_test.go:")
	t.NotHasSuffix(stack.Value, "\n")
}
