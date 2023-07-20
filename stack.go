package slogx

import (
	"bytes"
	"log/slog"
	"runtime"
)

const KeyStack = "stack"

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

	return slog.Attr{Key: KeyStack, Value: slog.StringValue(string(buf[:len(buf)-1]))}
}
