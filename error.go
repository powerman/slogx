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

type config struct {
	groupTopErrorAttrs  bool
	inlineSubErrorAttrs bool
}

// Error returns string value of errorAttrs error.
func (e errorAttrs) Error() string { return e.err.Error() }

// Unwrap returns errorAttrs error.
func (e errorAttrs) Unwrap() error { return e.err }

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

type errorNoAttrs struct { //nolint:errname // Custom naming.
	err error
}

// Error returns string value of errorNoAttrs error.
func (e errorNoAttrs) Error() string { return e.err.Error() }

// Unwrap returns errorNoAttrs error.
func (e errorNoAttrs) Unwrap() error { return e.err }

// NewErrorNoAttrs returns errorNoAttrs error that contains only given err.
// Such error signalize to stop recursive unwrapping and checking for attrs.
func NewErrorNoAttrs(err error) error {
	if err == nil {
		return nil
	}
	return errorNoAttrs{err: err}
}

// ErrorAttrs returns an slog.ReplaceAttr function that collects all attrs from
// wrapped errors, orders them as deeper errors come first, top level error come
// last and appends a.Key and a.Value (as errorNoAttrs type) to the end of
// accumulated attrs.
//
// Returned attr's Value is of slog.KindGroup. If groups is empty, Key will be empty,
// otherwise it will be a.Key.
//
// If no errorAttrs args found it returns a as is.
func ErrorAttrs(opts ...errorAttrsOption) func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if !(a.Value.Kind() == slog.KindAny) {
			return a
		}
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}

		var attrs []slog.Attr
		attrs = getAttrs(attrs, err)
		if len(attrs) == 0 {
			return a
		}
		attrs = append(attrs, slog.Any(a.Key, errorNoAttrs{err: err}))

		cfg := &config{}
		for _, opt := range opts {
			opt(cfg)
		}
		return slog.Attr{Key: key(a.Key, groups, cfg), Value: slog.GroupValue(attrs...)}
	}
}

func GroupTopErrorAttrs() errorAttrsOption { //nolint:revive // By design.
	return func(cfg *config) {
		cfg.groupTopErrorAttrs = true
	}
}

func InlineSubErrorAttrs() errorAttrsOption { //nolint:revive // By design.
	return func(cfg *config) {
		cfg.inlineSubErrorAttrs = true
	}
}

func getAttrs(attrs []slog.Attr, err error) []slog.Attr {
	if _, ok := err.(errorNoAttrs); ok { //nolint:errorlint // Necessary type assertion.
		return attrs
	}
	if errAttr, ok := err.(errorAttrs); ok { //nolint:errorlint // Necessary type assertion.
		attrs = getAttrs(attrs, errAttr.Unwrap())
		attrs = append(attrs, errAttr.attrs...)
	} else {
		if e := errors.Unwrap(err); e != nil {
			attrs = getAttrs(attrs, e)
		}
	}
	return attrs
}

func key(key string, groups []string, cfg *config) string {
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
