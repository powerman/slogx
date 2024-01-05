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

type errorAttrsOption func(*errorAttrsConfig)

// NewError returns err with attached slog Attrs specified by args.
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

// NewErrorNoAttrs returns error. This type signalize
// to stop recursive unwrapping and checking for attrs.
func NewErrorNoAttrs(err error) error {
	if err == nil {
		return nil
	}
	return errorNoAttrs{err: err}
}

// ErrorAttrs returns an slog.ReplaceAttr function that will replace attr's Value of error type
// with slog.GroupValue containing all attrs attached to any of recursively unwrapped errors
// plus original attr's Value.
//
// By default returned attr's Key depends on groups:
// if groups are empty then Key will be empty, otherwise Key will be attr's Key.
// This behaviour may be changed by given options.
//
// If attr's Value is not of error type or error has no attached attrs then returns original attr.
func ErrorAttrs(opts ...errorAttrsOption) func(groups []string, attr slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Value.Kind() != slog.KindAny {
			return a
		}
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}

		attrs := getAllAttrs(err)
		if len(attrs) == 0 {
			return a
		}
		attrs = append(attrs, slog.Any(a.Key, errorNoAttrs{err: err}))

		cfg := &errorAttrsConfig{}
		for _, opt := range opts {
			opt(cfg)
		}
		return slog.Attr{Key: key(a.Key, groups, cfg), Value: slog.GroupValue(attrs...)}
	}
}

func GroupTopErrorAttrs() errorAttrsOption { //nolint:revive // By design.
	return func(cfg *errorAttrsConfig) {
		cfg.groupTopErrorAttrs = true
	}
}

func InlineSubErrorAttrs() errorAttrsOption { //nolint:revive // By design.
	return func(cfg *errorAttrsConfig) {
		cfg.inlineSubErrorAttrs = true
	}
}

func getAllAttrs(err error) []slog.Attr {
	if err == nil {
		return nil
	}
	if _, ok := err.(errorNoAttrs); ok { //nolint:errorlint // Necessary type assertion.
		return nil
	}
	if errAttr, ok := err.(errorAttrs); ok { //nolint:errorlint // Necessary type assertion.
		return append(getAllAttrs(errors.Unwrap(err)), errAttr.attrs...)
	}
	return getAllAttrs(errors.Unwrap(err))
}

func key(key string, groups []string, cfg *errorAttrsConfig) string {
	groupsIsZero := len(groups) == 0
	switch {
	case groupsIsZero && cfg.groupTopErrorAttrs && cfg.inlineSubErrorAttrs:
		return key
	case groupsIsZero && cfg.groupTopErrorAttrs:
		return key
	case groupsIsZero && cfg.inlineSubErrorAttrs:
		return ""
	case !groupsIsZero && cfg.groupTopErrorAttrs && cfg.inlineSubErrorAttrs:
		return ""
	case !groupsIsZero && cfg.groupTopErrorAttrs:
		return key
	case !groupsIsZero && cfg.inlineSubErrorAttrs:
		return ""
	default:
		if groupsIsZero {
			return ""
		}
	}
	return key
}
