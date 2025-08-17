# Golang extensions for log/slog

[![License MIT](https://img.shields.io/badge/license-MIT-royalblue.svg)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/powerman/slogx?color=blue)](https://go.dev/)
[![Test](https://img.shields.io/github/actions/workflow/status/powerman/slogx/test.yml?label=test)](https://github.com/powerman/slogx/actions/workflows/test.yml)
[![Coverage Status](https://raw.githubusercontent.com/powerman/slogx/gh-badges/coverage.svg)](https://github.com/powerman/slogx/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/powerman/slogx)](https://goreportcard.com/report/github.com/powerman/slogx)
[![Release](https://img.shields.io/github/v/release/powerman/slogx?color=blue)](https://github.com/powerman/slogx/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/powerman/slogx.svg)](https://pkg.go.dev/github.com/powerman/slogx)

# Recommendations

## Using CtxHandler with linter

Disable non-Context slog functions (e.g. slog.Info) and methods using linter.

Example in golangci-lint config:

```
linters-settings:
  ...
  forbidigo:
    ...
    forbid:
      # slogx.CtxHandler support:
      - p: ^slog\.(Logger\.)?Error$
        msg: Use ErrorContext to support slogx.CtxHandler
      - p: ^slog\.(Logger\.)?Warn$
        msg: Use WarnContext to support slogx.CtxHandler
      - p: ^slog\.(Logger\.)?Info$
        msg: Use InfoContext to support slogx.CtxHandler
      - p: ^slog\.(Logger\.)?Debug$
        msg: Use DebugContext to support slogx.CtxHandler
    analyze-types: true
```
