package slogx_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/powerman/slogx"
)

// Deferred functions run before the stack is unwound,
// so slogx.Stack resolved inside a recover-defer shows the panic point.
// The helper frame between the deferred closure and the log call appears above the panic.
func ExampleStack_panicRecovery() {
	defer slog.SetDefault(slog.Default())

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
		Format: map[string]string{slogx.StackKey: "\n%s"},
	})))

	func() {
		defer func() {
			if p := recover(); p != nil {
				observe(p)
			}
		}()
		businessLogic()
	}()

	// Print only function names of the frames: file paths and addresses are not stable.
	for line := range strings.SplitSeq(buf.String(), "\n") {
		if name, _, ok := strings.Cut(line, "("); ok && !strings.ContainsAny(name, " \t=") {
			fmt.Println(name)
			if strings.HasSuffix(name, "slogx_test.ExampleStack_panicRecovery") {
				break
			}
		}
	}

	// Output:
	// github.com/powerman/slogx_test.observe
	// github.com/powerman/slogx_test.ExampleStack_panicRecovery.func1.1
	// panic
	// github.com/powerman/slogx_test.deepInside
	// github.com/powerman/slogx_test.businessLogic
	// github.com/powerman/slogx_test.ExampleStack_panicRecovery.func1
	// github.com/powerman/slogx_test.ExampleStack_panicRecovery
}

func observe(p any) {
	slog.Error("panic", "err", p, slogx.Stack) //nolint:loggercheck // By design.
}

func businessLogic() { deepInside() }

func deepInside() { panic("oops") }
