package slogx

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

const (
	KeyStack = "stack"
	trim     = 3
)

// Stack returns a stack trace formatted as panic output.
// It excludes a call of slogx.Stack() itself.
func Stack() slog.Attr {
	buf := make([]byte, 2048)
	runtime.Stack(buf, false)

	s := strings.Split(string(buf), "\n")
	var stack string
	if len(s) > trim {
		for _, line := range s[trim : len(s)-1] {
			stack += fmt.Sprintf("%s\n", line)
		}
		stack = stack[:len(stack)-1]
	}
	return slog.Attr{Key: KeyStack, Value: slog.StringValue(stack)}
}
