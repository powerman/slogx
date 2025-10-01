Based on log/slog from Go sources version 1.25.1.

Modified to implement `LayoutHandler` (based on `commonHandler`) to ensure 100% compatibility
with `slog.TextHandler` and comparable performance.

The `text_handler.go` is needed just to run standard tests with `LayoutHandler` as a backend.

Tests for new features added in `LayoutHandler` are in parent package.
