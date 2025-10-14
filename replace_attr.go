package slogx

import "log/slog"

// ChainReplaceAttr returns a function suitable for using in [slog.HandlerOptions].ReplaceAttr
// which actually executes several such functions in a chain.
// All these functions will get same first arg, but second arg will be value returned by
// previous function in a chain.
//
// If one of chained functions will return zero [slog.Attr] or an attr with
// value of kind [slog.KindGroup] then it skips next functions in a chain and
// returns this attr.
func ChainReplaceAttr(fs ...func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
	if len(fs) == 0 {
		panic("arguments required")
	}

	return func(g []string, a slog.Attr) slog.Attr {
		for _, f := range fs {
			a = f(g, a)
			if a.Equal(slog.Attr{}) || a.Value.Kind() == slog.KindGroup {
				return a
			}
		}
		return a
	}
}
