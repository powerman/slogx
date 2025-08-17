package slogx

import (
	"bytes"
	"log/slog"
	"runtime"
)

// StackKey is the key used by the [Stack] for the stack trace
// of the log call. The associated value is a string.
const StackKey = "stack"

// Stack returns a stack trace formatted as panic output.
// It excludes a call of Stack() itself.
func Stack() slog.Attr {
	const size = 64 << 10
	buf := make([]byte, size)
	buf = buf[:runtime.Stack(buf, false)]

	line1 := bytes.IndexRune(buf, '\n')
	line2 := bytes.IndexRune(buf[line1+1:], '\n')
	line3 := bytes.IndexRune(buf[line1+1+line2+1:], '\n')
	copy(buf[line2+1+line3+1:], buf[:line1+1])
	buf = buf[line2+1+line3+1:]

	return slog.Attr{Key: StackKey, Value: slog.StringValue(string(buf[:len(buf)-1]))}
}
