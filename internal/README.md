Based on log/slog from Go sources version 1.27.3.

Files marked `vendored` in `.gitattributes` are kept close to upstream,
so syncing them with a new Go version means applying the diff between Go versions.

The `layout_handler.go` implements `LayoutHandler` derived from `commonHandler` (`handler.go`)
to ensure 100% compatibility with `slog.TextHandler` and comparable performance.
Its code no longer follows upstream,
so changes of upstream `handler.go` have to be ported by hand.

The `text_handler.go` is needed just to run standard tests with `LayoutHandler` as a backend.

Tests for new features added in `LayoutHandler` are in parent package.
