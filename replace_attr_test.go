package slogx_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestReplaceAttr(tt *testing.T) {
	t := check.T(tt)

	var (
		id     = "ID"
		userID = "UserID"

		checkGroups = func(g []string, a slog.Attr) slog.Attr {
			if len(g) > 0 && g[0] == "g" {
				return slog.Attr{}
			}
			return a
		}
		modifyAttrValue = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == id {
				a.Value = slog.StringValue("REDACTED")
			}
			return a
		}
		modifyAttrKey = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == id {
				a.Key = userID
			}
			return a
		}
		returnZeroAttr = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
		modifyAttrTime = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.AnyValue(time.Now().Round(time.Hour))
			}
			return a
		}
	)

	t.Panic(func() { slogx.ChainReplaceAttr() })

	fn := slogx.ChainReplaceAttr(checkGroups, modifyAttrValue, modifyAttrKey, returnZeroAttr, modifyAttrTime)
	t.DeepEqual(fn([]string{"g"}, slog.Attr{Key: id, Value: slog.IntValue(325)}), slog.Attr{})
	t.DeepEqual(fn([]string{}, slog.Attr{Key: id, Value: slog.IntValue(325)}), slog.Attr{Key: userID, Value: slog.StringValue("REDACTED")})
	t.DeepEqual(fn([]string{}, slog.Attr{Key: slog.TimeKey, Value: slog.AnyValue(time.Now())}), slog.Attr{})
}
