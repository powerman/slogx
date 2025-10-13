package slogx

import (
	"bytes"
	"log/slog"
	"runtime"
)

// StackKey is the key used by the [ErrorStack] for the stack trace
// of the log call. The associated value is a string.
const StackKey = "stack"

// ErrorStack returns a stack trace formatted as panic output.
//
// It formats a stack trace when called, so it is not a good choice for using in log calls
// that are disabled by the current log level because of the performance penalty.
// ErrorStack is intended to be used as a parameter to [NewError] or [NewErrorAttrs]
// to capture a stack trace at the point where an error happens.
//
// It excludes a call of ErrorStack() itself from the stack trace.
//
// Example usage:
//
//	err := doOperation()
//	if err != nil {
//		return slogx.NewErrorAttrs(err, slogx.ErrorStack())
//	}
//
//	// Later in the call chain, higher up the stack:
//	slog.Error("operation failed", "err", err) // Will include stack trace from return line.
func ErrorStack() slog.Attr {
	const size = 64 << 10 // 64 KB should be enough.
	buf := make([]byte, size)
	buf = buf[:runtime.Stack(buf, false)]

	// First three lines look like:
	//	goroutine 7 [running]:
	//	github.com/powerman/slogx.ErrorStack
	//		/path/to/slogx/stack.go:34 +0x4c
	// To skip lines 2 and 3, move line 1 forward.
	lenLine1 := 1 + bytes.IndexRune(buf, '\n')
	lenLine2 := 1 + bytes.IndexRune(buf[lenLine1:], '\n')
	lenLine3 := 1 + bytes.IndexRune(buf[lenLine1+lenLine2:], '\n')
	copy(buf[lenLine2+lenLine3:], buf[:lenLine1])
	buf = buf[lenLine2+lenLine3:]
	// Remove trailing newline added by runtime.Stack.
	buf = buf[:len(buf)-1]

	return slog.Attr{Key: StackKey, Value: slog.StringValue(string(buf))}
}
