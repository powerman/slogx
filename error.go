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

type errorAttrsConfig struct{}

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

// ErrorAttrs returns an slog.ReplaceAttr function that will replace attr's Value of error type
// with slog.GroupValue containing all attrs attached to any of recursively unwrapped errors
// plus original attr's Value.
//
// By default returned attr's Key depends on groups:
// if groups are empty then Key will be empty, otherwise Key will be attr's Key.
// This behaviour may be changed by given options.
//
// If attr's Value is not of error type or error has no attached attrs then returns original attr.
func ErrorAttrs(_ ...errorAttrsOption) func(groups []string, attr slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Value.Kind() != slog.KindAny {
			return a
		}
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}

		var attrs []slog.Attr
		attrs = getAllAttrs(attrs, err)
		if len(attrs) == 0 {
			return a
		}
		attrs = append(attrs, slog.String(a.Key, a.Value.String()))

		var key string
		if len(groups) > 0 {
			key = a.Key
		}
		return slog.Attr{Key: key, Value: slog.GroupValue(attrs...)}
	}
}

func getAllAttrs(attrs []slog.Attr, err error) []slog.Attr {
	if errAttr, ok := err.(errorAttrs); ok { //nolint:errorlint // Necessary type assertion.
		attrs = getAllAttrs(attrs, errAttr.Unwrap())
		attrs = append(attrs, errAttr.attrs...)
	} else {
		if e := errors.Unwrap(err); e != nil {
			attrs = getAllAttrs(attrs, e)
		}
	}
	return attrs
}
