package slogx

import "log/slog"

func ChainReplaceAttr(fn ...func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
	if len(fn) == 0 {
		panic("arguments required")
	}

	return func(g []string, a slog.Attr) slog.Attr {
		attr := a
		for _, f := range fn {
			attr = f(g, attr)
			if attr.Equal(slog.Attr{}) || attr.Value.Kind() == slog.KindGroup {
				return attr
			}
		}
		return attr
	}
}
