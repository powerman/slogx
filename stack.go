package slogx

import (
	"fmt"
	"log/slog"
	"runtime"
)

func Stack() slog.Attr {
	var stack string
	pcs := make([]uintptr, 10)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		stack += fmt.Sprintf("%s\n", frame.Function)
		if !more {
			break
		}
	}
	return slog.Attr{Key: "stack", Value: slog.StringValue(stack[:len(stack)-1])}
}
