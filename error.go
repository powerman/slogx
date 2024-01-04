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

type errorAttrsOption func(*config)

// NewError returns err with attached slog Attrs specified by args.
func NewError(err error, args ...any) error {
	if err == nil {
		return nil
	}
	e := errorAttrs{err: err}
	if len(args) > 0 {
		e.attrs = argsToAttrSlice(args)
	}
	return e
}

// NewErrorAttrs returns errorAttrs error that contains given err and attrs.
func NewErrorAttrs(err error, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	e := errorAttrs{err: err}
	if len(attrs) > 0 {
		e.attrs = attrs
	}
	return e
}

// ErrorAttrs returns an slog.ReplaceAttr function that collects all attrs from
// wrapped errors, orders them as deeper errors come first, top level error come
// last and appends slog.String(a.Key, a.Value.String()) to the end of accumulated
// attrs.
//
// Returned attr's Value is of slog.KindGroup. If groups is empty, Key will be empty,
// otherwise it will be a.Key.
//
// If no errorAttrs args found it returns a as is.
func ErrorAttrs(_ ...errorAttrsOption) func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if !(a.Value.Kind() == slog.KindAny) {
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
