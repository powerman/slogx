/*
Package slogx contains extensions for [log/slog].

# Handlers

  - [LayoutHandler] is a drop-in replacement for [slog.TextHandler]
    with configurable key formatting and ordering,
    designed for compact and easy to read output.
    See [LayoutHandlerOptions] for details and examples.
  - [NewContextHandler] creates a handler that delegates to another handler stored in a context.
    This allows to add attributes and groups into context
    using [ContextWith], [ContextWithAttrs], and [ContextWithGroup] functions
    and have them automatically included in log records
    when using default logger functions like [slog.InfoContext] etc.
    [NewDefaultContextLogger] provides a compatibility with third-party libraries
    which use non-Context-aware slog functions/methods.
    [NewContextMiddleware] provides a compatibility with [github.com/samber/slog-multi.Pipe].
  - [WrapHandler] is a building block for handlers that want to wrap another handler.
    See [WrapHandlerConfig] for details.
    [NewWrapMiddleware] provides a compatibility with [github.com/samber/slog-multi.Pipe].

# Helpers

  - [ChainReplaceAttr] allows to run multiple functions using [slog.HandlerOptions].ReplaceAttr.
  - [Stack] is a pre-defined attribute that resolves to a stack trace formatted as panic output.
  - [NewError] and [NewErrorAttrs] attach slog attributes to an error, to log them later
    (when the error is logged) using ReplaceAttr function returned by [ErrorAttrs].
    Use [ErrorStack] to add a stack trace to the error attributes.
  - [LogSkip] and [LogAttrsSkip] are building blocks for your own logging helpers,
    allowing to skip stack frames when reporting the caller by [slog.HandlerOptions].AddSource.
    Use [StackSkip] to add a stack trace in such helpers.
*/
package slogx
