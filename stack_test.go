package slogx_test

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestErrorStack(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	reStack := regexp.MustCompile(`^goroutine \d+ \[\S+\]:\ngithub\.com/powerman/slogx_test\.TestErrorStack[(](.|\n)*[^\n]$`)

	stack := slogx.ErrorStack()
	t.Equal(stack.Key, slogx.StackKey)
	t.Match(stack.Value, reStack)
}

func TestStack(tt *testing.T) {
	t := check.T(tt)
	// Do not run in parallel because it modifies the default logger.
	reStack := regexp.MustCompile(`^goroutine \d+ \[\S+\]:\ngithub\.com/powerman/slogx_test\.TestStack\((.|\n)*[^\n]$`)
	reLog := regexp.MustCompile(`^time=\S+ level=INFO msg=Test\ngoroutine \d+ \[\S+\]:\ngithub\.com/powerman/slogx_test\.TestStack\((.|\n)*[^\n]\n$`)

	t.Equal(slogx.Stack.Key, slogx.StackKey)
	t.Match(slogx.Stack.Value.Resolve(), reStack)

	var buf bytes.Buffer
	log := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
		Format: map[string]string{slogx.StackKey: "\n%s"},
	}))
	log.Info("Test", slogx.Stack) //nolint:loggercheck // By design.
	t.Match(buf.String(), reLog)

	buf.Reset()
	slog.SetDefault(log)
	slog.Info("Test", slogx.Stack) //nolint:loggercheck // By design.
	t.Match(buf.String(), reLog)
}

func TestStackSkip(tt *testing.T) {
	t := check.T(tt)
	// Do not run in parallel because it modifies the default logger.
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
		Format: map[string]string{slogx.StackKey: "\n%s"},
	})))
	reStack := regexp.MustCompile(`^goroutine \d+ \[\S+\]:\ngithub\.com/powerman/slogx_test\.TestStackSkip[(](.|\n)*[^\n]$`)
	reLog := regexp.MustCompile(`^time=\S+ level=INFO msg=Test\ngoroutine \d+ \[\S+\]:\ngithub\.com/powerman/slogx_test\.TestStackSkip\((.|\n)*[^\n]\n$`)

	stack := slogx.StackSkip(0)
	t.DeepEqual(stack.Key, slogx.StackKey)
	t.Match(stack.Value.Resolve(), reStack)
	t.NotMatch(resolveValue(stack.Value), reStack)

	slog.Info("Test", slogx.StackSkip(0))
	t.Match(buf.String(), reLog)

	stack = slogx.StackSkip(1)
	t.NotMatch(stack.Value.Resolve(), reStack)
	t.Match(resolveValue(stack.Value), reStack)

	buf.Reset()
	slog.Info("Test", slogx.StackSkip(1))
	t.NotMatch(buf.String(), reLog)

	buf.Reset()
	logSkipHelper("Test")
	t.Match(buf.String(), reLog)
}

func resolveValue(v slog.Value) slog.Value {
	return v.Resolve()
}

func logSkipHelper(msg string, args ...any) {
	const skip = 1
	ctx := context.Background()
	h := slog.Default().Handler()
	args = append(args, slogx.StackSkip(skip))
	slogx.LogSkip(ctx, skip, h, slog.LevelInfo, msg, args...)
}
