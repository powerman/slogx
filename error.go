package slogx

import (
	"errors"
	"log/slog"
)

const KeyBadKey = "!BADKEY"

type errorAttrs struct { //nolint:errname // Custom naming.
	err   error
	attrs []slog.Attr
}

// Error implements error interface.
func (e errorAttrs) Error() string { return e.err.Error() }

// Unwrap returns errorAttrs error.
func (e errorAttrs) Unwrap() error { return e.err }

type config struct{}

type errorAttrsOption func(*config)

// NewError returns errorAttrs error that contains given err and args,
// modified to []slog.Attr.
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

func argsToAttrSlice(args []any) []slog.Attr {
	var (
		attr  slog.Attr
		attrs []slog.Attr
	)
	for len(args) > 0 {
		attr, args = argsToAttr(args)
		attrs = append(attrs, attr)
	}
	return attrs
}

// argsToAttr turns a prefix of the nonempty args slice into an Attr
// and returns the unconsumed portion of the slice.
// If args[0] is an Attr, it returns it.
// If args[0] is a string, it treats the first two elements as
// a key-value pair.
// Otherwise, it treats args[0] as a value with a missing key.
func argsToAttr(args []any) (slog.Attr, []any) { // Probably will be add with CtxHandler for common use.
	switch x := args[0].(type) {
	case string:
		if len(args) == 1 {
			return slog.String(KeyBadKey, x), nil
		}
		return slog.Any(x, args[1]), args[2:]

	case slog.Attr:
		return x, args[1:]

	default:
		return slog.Any(KeyBadKey, x), args[1:]
	}
}
