# Golang extensions for log/slog

[![Go Reference](https://pkg.go.dev/badge/github.com/powerman/slogx.svg)](https://pkg.go.dev/github.com/powerman/slogx)
[![CI/CD](https://github.com/powerman/slogx/workflows/CI/CD/badge.svg?event=push)](https://github.com/powerman/slogx/actions?query=workflow%3ACI%2FCD)
[![Coverage Status](https://coveralls.io/repos/github/powerman/slogx/badge.svg?branch=master)](https://coveralls.io/github/powerman/slogx?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/powerman/slogx)](https://goreportcard.com/report/github.com/powerman/slogx)
[![Release](https://img.shields.io/github/v/release/powerman/slogx)](https://github.com/powerman/slogx/releases/latest)

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
