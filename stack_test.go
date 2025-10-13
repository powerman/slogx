package slogx_test

import (
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestErrorStack(tt *testing.T) {
	t := check.T(tt)

	stack := slogx.ErrorStack()
	t.DeepEqual(stack.Key, slogx.StackKey)
	t.HasPrefix(stack.Value, "goroutine")
	t.NotMatch(stack.Value, "github.com/powerman/slogx.ErrorStack()")
	t.NotMatch(stack.Value, "/stack.go:")
	t.Match(stack.Value, "github.com/powerman/slogx_test.TestErrorStack")
	t.Match(stack.Value, "/stack_test.go:")
	t.NotHasSuffix(stack.Value, "\n")
}
