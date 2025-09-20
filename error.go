package slogx

import (
	"errors"
	"log/slog"
)

type errorAttrs struct { //nolint:errname // Custom naming.
	err   error
	attrs []slog.Attr
}

// Error implements error interface.
func (e errorAttrs) Error() string { return e.err.Error() }

// Unwrap returns wrapped error.
func (e errorAttrs) Unwrap() error { return e.err }

type errorAttrsConfig struct {
	groupTopErrorAttrs  bool
	inlineSubErrorAttrs bool
}

func (cfg errorAttrsConfig) key(key string, groups []string) string {
	hasGroups := len(groups) != 0
	switch {
	case !hasGroups && cfg.groupTopErrorAttrs:
		return key
	case hasGroups && cfg.inlineSubErrorAttrs:
		return ""
	case !hasGroups:
		return ""
	default:
		return key
	}
}

// ErrorAttrsOption is an option for ErrorAttrs.
type ErrorAttrsOption func(*errorAttrsConfig)

// GroupTopErrorAttrs is an option for ErrorAttrs.
//
// By default error attrs are inlined at top level and grouped at sub levels.
// This option makes attrs to be grouped at top level (when groups is empty).
func GroupTopErrorAttrs() ErrorAttrsOption {
	return func(cfg *errorAttrsConfig) {
		cfg.groupTopErrorAttrs = true
	}
}

// InlineSubErrorAttrs is an option for ErrorAttrs.
//
// By default error attrs are inlined at top level and grouped at sub levels.
// This option makes attrs to be inlined at sub levels (when groups is not empty).
func InlineSubErrorAttrs() ErrorAttrsOption {
	return func(cfg *errorAttrsConfig) {
		cfg.inlineSubErrorAttrs = true
	}
}

// NewError returns err with attached slog attrs specified by args.
func NewError(err error, args ...any) error {
	return NewErrorAttrs(err, argsToAttrSlice(args)...)
}

// NewErrorAttrs returns err with attached slog attrs.
func NewErrorAttrs(err error, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	return errorAttrs{err: err, attrs: attrs}
}

type errorNoAttrs struct { //nolint:errname // Custom naming.
	err error
}

// Error implements error interface.
func (e errorNoAttrs) Error() string { return e.err.Error() }

// Unwrap returns wrapped error.
func (e errorNoAttrs) Unwrap() error { return e.err }

// ErrorAttrs returns an slog.ReplaceAttr function that will replace attr's Value of error type
// with slog.GroupValue containing all attrs attached to any of recursively unwrapped errors
// plus original attr's Value (error).
//
// By default returned attr's Key depends on groups:
// if groups are empty then Key will be empty, otherwise Key will be attr's Key.
// This behaviour may be changed by given options.
//
// If attr's Value is not of error type or error has no attached attrs then returns original attr.
func ErrorAttrs(opts ...ErrorAttrsOption) func(groups []string, attr slog.Attr) slog.Attr {
	cfg := errorAttrsConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Value.Kind() != slog.KindAny {
			return a
		}
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}

		attrs := getErrorAttrs(err)
		if len(attrs) == 0 {
			return a
		}
		attrs = append(attrs, slog.Any(a.Key, errorNoAttrs{err: err}))

		return slog.Attr{Key: cfg.key(a.Key, groups), Value: slog.GroupValue(attrs...)}
	}
}

// getErrorAttrs returns all slog attrs attached to err and its wrapped errors,
// in order from outer to inner.
func getErrorAttrs(err error) []slog.Attr {
	switch err2 := err.(type) { //nolint:errorlint // We want to check for specific types.
	case nil:
		return nil
	case errorNoAttrs:
		return nil
	case errorAttrs:
		return append(err2.attrs, getErrorAttrs(errors.Unwrap(err))...)
	default:
		return getErrorAttrs(errors.Unwrap(err))
	}
}
