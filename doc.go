/*
Package slogx contains extensions for [log/slog].

# Handlers

  - [LayoutHandler] is a drop-in replacement for [slog.TextHandler]
    with configurable key formatting and ordering,
    designed for compact and easy to read output.
    See [LayoutHandlerOptions] for details and examples.
  - [ContextHandler] is a handler that delegates to another handler stored in context.
    This allows to add attributes and groups into context
    using [ContextWith], [ContextWithAttrs], and [ContextWithGroup] functions
    and have them automatically included in log records
    when using default logger functions like [slog.InfoContext] etc.
  - [WrapHandler] is a building block for handlers that want to wrap another handler.
    See [WrapHandlerConfig] for details.
*/
package slogx
