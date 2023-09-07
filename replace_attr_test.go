package slogx_test

import (
	"log/slog"
	"net/http"
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
		req    = "req"

		groupAttr = slog.Group(req,
			slog.String("method", http.MethodPut),
			slog.String("url", "localhost"))
		groupAttrModified = slog.Group(req,
			slog.String("method", http.MethodPost),
			slog.String("url", "localhost"))
		groupAttrIgnored = slog.Group(req,
			slog.String("method", http.MethodDelete),
			slog.String("url", "localhost"))
		modifyAttrValue = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == id {
				a.Value = slog.StringValue("REDACTED")
			}
			return a
		}
		modifyAttrKey = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == id {
				a.Key = userID
			}
			return a
		}
		returnZeroAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
		ignoreAfterZeroAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.AnyValue(time.Now().Round(time.Hour))
			}
			return a
		}
		withGroupAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == req {
				a = groupAttrModified
			}
			return a
		}
		ignoreAfterGroupAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == req {
				a = groupAttrIgnored
			}
			return a
		}
	)

	t.Panic(func() { slogx.ChainReplaceAttr() })

	fn := slogx.ChainReplaceAttr(modifyAttrValue, modifyAttrKey, returnZeroAttr, ignoreAfterZeroAttr)
	t.DeepEqual(fn([]string{}, slog.Attr{Key: id, Value: slog.IntValue(325)}), slog.Attr{Key: userID, Value: slog.StringValue("REDACTED")})
	t.DeepEqual(fn([]string{}, slog.Attr{Key: slog.TimeKey, Value: slog.AnyValue(time.Now())}), slog.Attr{})

	fn = slogx.ChainReplaceAttr(withGroupAttr, ignoreAfterGroupAttr)
	t.DeepEqual(fn([]string{}, groupAttr), groupAttrModified)
}
