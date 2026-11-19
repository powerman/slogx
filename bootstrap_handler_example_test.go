//nolint:unparam // Example.
package slogx_test

import (
	"context"
	"log/slog"
	"os"

	slogformatter "github.com/samber/slog-formatter"
	slogmulti "github.com/samber/slog-multi"

	"github.com/powerman/slogx"
)

// A CLI creates its bootstrap handler before parsing flags,
// because code which runs while flags are parsed (hooks, validation) logs too,
// but the level and format (text/json) is known only after parsing.
//
// If parsing failed the level is not known,
// so hand off to a handler which prints only errors:
// held errors are printed before the final one.
//
// Replacing bootstrapHandler by the real handler after HandOffTo is optional,
// see the block at the end of the example.
func ExampleBootstrapHandler() {
	defer slog.SetDefault(slog.Default())

	// Phase 1: Setup bootstrap logger for phase 2.
	bootstrapHandler := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	logger := slog.New(bootstrapHandler).With("app", "example")
	slog.SetDefault(logger)

	var level slog.LevelVar

	// Phase 2: Parse os.Args, env and config.
	err := func() error {
		logger.Info("parsing flags") // Held.
		slog.Warn("deprecated flag") // Held.
		slog.Debug("hook of a flag") // Held.
		level.Set(slog.LevelInfo)    // The level is known now (--verbose).
		return nil
	}()
	if err != nil {
		_ = bootstrapHandler.HandOffTo(newTextHandler(os.Stdout, slog.LevelError))
		logger.Error("failed to start", "err", err)
		os.Exit(2)
	}

	// Phase 3: Hand off held records to the real handler, it is known now.
	_ = bootstrapHandler.HandOffTo(newTextHandler(os.Stdout, &level))

	// Optional: replace bootstrapHandler by the real handler to save one extra call per record.
	// Code which has captured the logger during phase 2 keeps working through bootstrapHandler.
	logger = slog.New(logger.Handler().WithAttrs(nil)) // Keeps "app".
	slog.SetDefault(logger)

	slog.Info("started")
	// Output:
	// level=INFO msg="parsing flags" app=example
	// level=WARN msg="deprecated flag" app=example
	// level=INFO msg=started app=example
}

// A handler stored in a context (see [slogx.NewContextHandler]) works the same way:
// replacing it by the real one after the handoff is optional.
func ExampleBootstrapHandler_contextHandler() {
	defer slog.SetDefault(slog.Default())

	// Phase 1: Setup bootstrap logger for phase 2.
	bootstrapHandler := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	ctx := slogx.SetDefaultContextHandler(context.Background(), bootstrapHandler)
	ctx = slogx.ContextWith(ctx, "app", "example")

	var level slog.LevelVar

	// Phase 2: Parse os.Args, env and config.
	err := func() error {
		slog.InfoContext(ctx, "parsing flags")   // Held.
		slog.WarnContext(ctx, "deprecated flag") // Held.
		slog.DebugContext(ctx, "hook of a flag") // Held.
		level.Set(slog.LevelInfo)                // The level is known now (--verbose).
		return nil
	}()
	if err != nil {
		_ = bootstrapHandler.HandOffTo(newTextHandler(os.Stdout, slog.LevelError))
		slog.ErrorContext(ctx, "failed to start", "err", err)
		os.Exit(2)
	}

	// Phase 3: Hand off held records to the real handler, it is known now.
	_ = bootstrapHandler.HandOffTo(newTextHandler(os.Stdout, &level))

	// Optional: replace bootstrapHandler by the real handler to save one extra call per record.
	// A context which has been passed to other code during phase 2
	// keeps working through bootstrapHandler.
	handler := slogx.HandlerFromContext(ctx).WithAttrs(nil) // Keeps "app".
	ctx = slogx.SetDefaultContextHandler(ctx, handler)

	slog.InfoContext(ctx, "started")
	// Output:
	// level=INFO msg="parsing flags" app=example
	// level=WARN msg="deprecated flag" app=example
	// level=INFO msg=started app=example
}

// In a chain of middleware BootstrapHandler is the last handler.
// Middleware which transforms records (e.g. redacts secrets) goes before it.
// Sinks (e.g. slogmulti.Fanout) are known only after parsing: they are passed to HandOffTo.
// So the chain of the application is the same chain with the sinks instead of BootstrapHandler.
func ExampleBootstrapHandler_middleware() {
	defer slog.SetDefault(slog.Default())

	redact := slogformatter.NewFormatterMiddleware(
		slogformatter.FormatByKey("password", func(slog.Value) slog.Value {
			return slog.StringValue("***")
		}))

	// Phase 1: Setup bootstrap logger for phase 2.
	bootstrapHandler := slogx.MustBootstrapHandler(slogx.BootstrapHandlerConfig{})
	logger := slog.New(slogmulti.Pipe(redact).Handler(bootstrapHandler)).With("app", "example")
	slog.SetDefault(logger)

	var level slog.LevelVar

	// Phase 2: Parse os.Args, env and config.
	err := func() error {
		logger.Info("parsing flags")              // Held.
		slog.Warn("deprecated flag")              // Held.
		slog.Debug("hook of a flag")              // Held.
		slog.Info("config", "password", "secret") // Redacted, then held.
		level.Set(slog.LevelInfo)                 // The level is known now (--verbose).
		return nil
	}()
	if err != nil {
		_ = bootstrapHandler.HandOffTo(newTextHandler(os.Stdout, slog.LevelError))
		logger.Error("failed to start", "err", err)
		os.Exit(2)
	}

	// Phase 3: Hand off held records to the sinks, they are known now.
	_ = bootstrapHandler.HandOffTo(slogmulti.Fanout(
		newTextHandler(os.Stdout, &level),
		newJSONHandler(os.Stdout, &level),
	))

	// Optional: replace bootstrapHandler by the real handler to save one extra call per record.
	// Code which has captured the logger during phase 2 keeps working through bootstrapHandler.
	logger = slog.New(logger.Handler().WithAttrs(nil)) // Keeps "app".
	slog.SetDefault(logger)

	slog.Info("started")
	// Output:
	// level=INFO msg="parsing flags" app=example
	// {"level":"INFO","msg":"parsing flags","app":"example"}
	// level=WARN msg="deprecated flag" app=example
	// {"level":"WARN","msg":"deprecated flag","app":"example"}
	// level=INFO msg=config app=example password=***
	// {"level":"INFO","msg":"config","app":"example","password":"***"}
	// level=INFO msg=started app=example
	// {"level":"INFO","msg":"started","app":"example"}
}
