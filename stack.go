package slogx

import (
	"bytes"
	"log/slog"
	"runtime"
)

// StackKey is the key used by the [Stack], [StackSkip] and [ErrorStack]
// for the stack trace attribute.
const StackKey = "stack"

const stackSize = 64 << 10 // 64 KB should be enough.

// ErrorStack returns a stack trace formatted as panic output.
//
// It formats a stack trace when called, so it is not a good choice for using in log calls
// that are disabled by the current log level because of the performance penalty
// (in this case use [Stack] instead).
// ErrorStack is intended to be used as a parameter to [NewError] or [NewErrorAttrs]
// to capture a stack trace at the point where an error happens.
//
// It excludes a call of ErrorStack itself from the stack trace.
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
	buf := make([]byte, stackSize)
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

type stackValue struct {
	skip int // Extra stack frames to skip.
}

// Stack is a pre-defined attribute that resolves to a stack trace formatted as panic output.
//
// Unlike [ErrorStack] it captures the stack trace when calling slogx.Stack.Value.Resolve(),
// which usually happens during log call processing.
// This makes Stack suitable for use in log calls that may be disabled by the current log level,
// but it also means that the stack trace will point to the log call location,
// not to the location where Stack was referenced (as in ErrorStack case).
//
// It excludes stack frames up to and including the last log/slog call from the stack trace.
//
// Example usage:
//
//	// Will format the stack trace only if Debug log level is enabled.
//	slog.Debug("something", slogx.Stack)
var Stack = StackSkip(0) // Const.

// StackSkip returns an attribute that resolves to a stack trace formatted as panic output.
//
// It works like [Stack], but skips the given number of extra stack frames.
// Negative skip values are treated as zero.
//
// Example usage:
//
//	func LogHelper(msg string, args ...any) {
//		const skip = 1 // LogHelper added one extra stack frame.
//		ctx := context.Background()
//		h := slog.Default().Handler()
//		// Skip LogHelper frame from stack.
//		args = append(args, slogx.StackSkip(skip))
//		// Skip LogHelper frame from source.
//		slogx.LogSkip(ctx, skip, h, slog.LevelInfo, msg, args...)
//	}
func StackSkip(skip int) slog.Attr {
	return slog.Any(StackKey, stackValue{skip: max(0, skip)})
}

func (s stackValue) LogValue() slog.Value {
	buf := make([]byte, stackSize)
	buf = buf[:runtime.Stack(buf, false)]

	// First lines look like:
	//	goroutine 7 [running]:
	//	github.com/powerman/slogx.stackValue.LogValue({})
	//		/path/to/slogx/stack.go:71 +0x3a
	//	log/slog.Value.Resolve({{}, 0x89db12?, {0x7773a0?, 0xb27e00?}})
	//		/path/to/log/slog/value.go:512 +0x9f
	//	... and if Resolve wasn't called manually, then
	//	... more lines ending with the log method call like:
	//	log/slog.(*Logger).Info(...)
	//		/path/to/log/slog/logger.go:209
	//	... or log function call like:
	//	log/slog.Info(...)
	//		/path/to/log/slog/logger.go:291
	// To skip lines from 2 to the line with last slog call, move line 1 forward.
	loggerPos := bytes.LastIndex(buf, []byte("\ngithub.com/powerman/slogx.LogSkip("))
	if loggerPos < 0 {
		loggerPos = bytes.LastIndex(buf, []byte("\ngithub.com/powerman/slogx.LogAttrsSkip("))
	}
	if loggerPos < 0 {
		loggerPos = bytes.LastIndex(buf, []byte("\nlog/slog."))
	}
	if loggerPos >= 0 {
		loggerPos++ // Move to line start.
		lenLine1 := 1 + bytes.IndexRune(buf, '\n')
		lenLoggerLines := loggerPos - lenLine1
		lenLoggerLines += 1 + bytes.IndexRune(buf[lenLine1+lenLoggerLines:], '\n')
		lenLoggerLines += 1 + bytes.IndexRune(buf[lenLine1+lenLoggerLines:], '\n')

		// Skip extra frames requested by caller.
		lenExtraLines := 0
		for range s.skip {
			lenExtraLines += 1 + bytes.IndexRune(buf[lenLine1+lenLoggerLines+lenExtraLines:], '\n')
			lenExtraLines += 1 + bytes.IndexRune(buf[lenLine1+lenLoggerLines+lenExtraLines:], '\n')
		}

		copy(buf[lenLoggerLines+lenExtraLines:], buf[:lenLine1])
		buf = buf[lenLoggerLines+lenExtraLines:]
	}

	// Remove trailing newline added by runtime.Stack.
	buf = buf[:len(buf)-1]

	return slog.StringValue(string(buf))
}
