package slogx

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/powerman/slogx/internal"
)

type errorNoAttrs struct { //nolint:errname // Custom naming.
	err error
	msg string
}

// newErrorNoAttrs returns err wrapped in errorNoAttrs.
// If err wraps other errors then their messages are removed from the end of err's message.
func newErrorNoAttrs(err error) error {
	msg := err.Error()
	for e := err; e != nil; {
		e2, ok := e.(interface{ Unwrap() []error })
		if !ok {
			e = errors.Unwrap(e)
			continue
		}

		es := e2.Unwrap()
		for i := len(es) - 1; i > 0; i-- {
			if before, ok0 := strings.CutSuffix(msg, "\n"+es[i].Error()); ok0 {
				msg = before
			}
		}

		if len(es) > 0 {
			e = es[0]
		} else {
			e = nil
		}
	}
	return errorNoAttrs{err: err, msg: msg}
}

// Error implements error interface.
func (e errorNoAttrs) Error() string { return e.msg }

// Unwrap returns wrapped error.
func (e errorNoAttrs) Unwrap() error { return e.err }

type errorAttrs struct { //nolint:errname // Custom naming.
	err   error
	attrs *[]slog.Attr // Pointer to make struct comparable (for tests).
}

// Error implements error interface.
func (e errorAttrs) Error() string { return e.err.Error() }

// Unwrap returns wrapped error.
func (e errorAttrs) Unwrap() error { return e.err }

// NewError returns err with attached slog attrs specified by args.
//
// You should use [slog.HandlerOptions].ReplaceAttr function returned by [ErrorAttrs]
// to make slog log these attrs.
//
// If err is nil then returns nil.
func NewError(err error, args ...any) error {
	return NewErrorAttrs(err, internal.ArgsToAttrSlice(args)...)
}

// NewErrorAttrs returns err with attached slog attrs.
//
// You should use [slog.HandlerOptions].ReplaceAttr function returned by [ErrorAttrs]
// to make slog log these attrs.
//
// If err is nil then returns nil.
func NewErrorAttrs(err error, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	return errorAttrs{err: err, attrs: &attrs}
}

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

// ErrorAttrsOption is an option for [ErrorAttrs].
type ErrorAttrsOption func(*errorAttrsConfig)

// GroupTopErrorAttrs makes error attrs to be grouped at top level (when groups is empty).
func GroupTopErrorAttrs() ErrorAttrsOption {
	return func(cfg *errorAttrsConfig) {
		cfg.groupTopErrorAttrs = true
	}
}

// InlineSubErrorAttrs makes error attrs to be inlined at sub levels (when groups is not empty).
func InlineSubErrorAttrs() ErrorAttrsOption {
	return func(cfg *errorAttrsConfig) {
		cfg.inlineSubErrorAttrs = true
	}
}

// ErrorAttrs returns an [slog.HandlerOptions].ReplaceAttr function
// that will replace attr's Value of error type
// with [slog.GroupValue] containing all attrs attached (by [NewError] or [NewErrorAttrs])
// to any of recursively unwrapped errors
// plus attr with error (stripped of attached attrs) itself at the end.
//
// By default returned attr's Key depends on groups:
// if groups are empty then Key will be empty, otherwise Key will be attr's Key.
// In other words, error attrs are inlined at top level and grouped at sub levels.
// This behaviour may be changed by given options.
//
// If attr's Value is not of error type or error has no attached attrs then returns original attr.
func ErrorAttrs(opts ...ErrorAttrsOption) func(groups []string, attr slog.Attr) slog.Attr {
	var cfg errorAttrsConfig
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

		attrs, rest := getErrorAttrs(nil, a.Key, err)
		if len(attrs)+len(rest) == 0 {
			return a
		}
		attrs = append(attrs, slog.Any(a.Key, newErrorNoAttrs(err)))
		attrs = append(attrs, rest...)

		return slog.Attr{Key: cfg.key(a.Key, groups), Value: slog.GroupValue(attrs...)}
	}
}

// getErrorAttrs returns all slog attrs attached to err and its wrapped errors,
// in order from outer to inner.
func getErrorAttrs(errorNum *int, key string, err error) (first []slog.Attr, rest []slog.Attr) {
	if errorNum == nil {
		num := 1
		errorNum = &num
	}

	switch err2 := err.(type) { //nolint:errorlint // We want to check for specific types.
	case nil:
		return nil, nil
	case errorNoAttrs:
		return nil, nil
	case errorAttrs:
		first, rest = getErrorAttrs(errorNum, key, errors.Unwrap(err))
		return append(*err2.attrs, first...), rest
	case interface{ Unwrap() []error }:
		errs := err2.Unwrap()
		if len(errs) == 0 {
			return nil, nil
		}
		first, rest = getErrorAttrs(errorNum, key, errs[0])
		for i := 1; i < len(errs); i++ {
			subFirst, subRest := getErrorAttrs(errorNum, key, errs[i])
			*errorNum++
			subFirst = append(subFirst, slog.Any(key, newErrorNoAttrs(errs[i])))
			rest = append(rest, slog.Attr{
				Key:   fmt.Sprintf("%s-%d", key, *errorNum),
				Value: slog.GroupValue(append(subFirst, subRest...)...),
			})
		}
		return first, rest
	default:
		return getErrorAttrs(errorNum, key, errors.Unwrap(err))
	}
}
