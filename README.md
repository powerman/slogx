# Golang extensions for log/slog

[![License MIT](https://img.shields.io/badge/license-MIT-royalblue.svg)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/powerman/slogx?color=blue)](https://go.dev/)
[![Test](https://img.shields.io/github/actions/workflow/status/powerman/slogx/test.yml?label=test)](https://github.com/powerman/slogx/actions/workflows/test.yml)
[![Coverage Status](https://raw.githubusercontent.com/powerman/slogx/gh-badges/coverage.svg)](https://github.com/powerman/slogx/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/powerman/slogx?color=blue)](https://github.com/powerman/slogx/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/powerman/slogx.svg)](https://pkg.go.dev/github.com/powerman/slogx)

![Linux | amd64 arm64 armv7 ppc64le s390x riscv64](https://img.shields.io/badge/Linux-amd64%20arm64%20armv7%20ppc64le%20s390x%20riscv64-royalblue)
![macOS | amd64 arm64](https://img.shields.io/badge/macOS-amd64%20arm64-royalblue)
![Windows | amd64 arm64](https://img.shields.io/badge/Windows-amd64%20arm64-royalblue)

## Features

- Zero required external dependencies.

### LayoutHandler

`LayoutHandler` is an alternative to `slog.TextHandler`
designed to make output easier to read with:

- Compact output for given attrs by replacing " key=" prefix before value.
- Reorder given attrs by moving them before message (prefix) or after other attrs (suffix).
- Vertical align for prefix attrs by enforcing min/max value width.
- Color output for given attrs.

### BootstrapHandler

`BootstrapHandler` holds log records in memory during the bootstrap of a process,
until the application knows what, how and where to log
(e.g. flags are parsed, so the level and the format are known).
Then `HandOffTo` replays held records through the real handler
and `BootstrapHandler` becomes a pass-through to it
(replacing `BootstrapHandler` by the real handler is optional:
it saves one extra call per record).

- Works with any logging style: default logger, logger in a variable, handler in a context.
- Logging from the real handler, `String` or `MarshalJSON` of a value can't deadlock it.

## Recommendations

### Use NewContextHandler with linter

Disable non-Context slog functions (e.g. `slog.Info`) and methods using linter.

Example golangci-lint v2 config:

```yaml
linters:
  settings:
    sloglint:
      context: all
```
