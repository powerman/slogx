package slogx

import "log/slog"

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
