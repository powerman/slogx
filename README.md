# Golang extensions for log/slog

[![License MIT](https://img.shields.io/badge/license-MIT-royalblue.svg)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/powerman/slogx?color=blue)](https://go.dev/)
[![Test](https://img.shields.io/github/actions/workflow/status/powerman/slogx/test.yml?label=test)](https://github.com/powerman/slogx/actions/workflows/test.yml)
[![Coverage Status](https://raw.githubusercontent.com/powerman/slogx/gh-badges/coverage.svg)](https://github.com/powerman/slogx/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/powerman/slogx)](https://goreportcard.com/report/github.com/powerman/slogx)
[![Release](https://img.shields.io/github/v/release/powerman/slogx?color=blue)](https://github.com/powerman/slogx/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/powerman/slogx.svg)](https://pkg.go.dev/github.com/powerman/slogx)

![Linux | amd64 arm64 armv7 ppc64le s390x riscv64](https://img.shields.io/badge/Linux-amd64%20arm64%20armv7%20ppc64le%20s390x%20riscv64-royalblue)
![macOS | amd64 arm64](https://img.shields.io/badge/macOS-amd64%20arm64-royalblue)
![Windows | amd64 arm64](https://img.shields.io/badge/Windows-amd64%20arm64-royalblue)

## Features

### LayoutHandler

`LayoutHandler` is an alternative to `slog.TextHandler`
designed to make output easier to read with:

- Compact output for given attrs by replacing " key=" prefix before value.
- Reorder given attrs by moving them before message (prefix) or after other attrs (suffix).
- Vertical align for prefix attrs by enforcing min/max value width.
- Color output for given attrs.

## Recommendations

### Using CtxHandler with linter

Disable non-Context slog functions (e.g. slog.Info) and methods using linter.

Example in golangci-lint config:

```yaml
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
