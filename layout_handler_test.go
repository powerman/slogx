package slogx_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestLayoutHandler(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer
	h := slogx.NewLayoutHandler(&buf, nil)
	t.Nil(slogtest.TestHandler(h, makeTextResults(t, &buf)))
}

func TestLayoutHandler_StdOptions(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer

	tests := []struct {
		addSource   bool
		level       slog.Leveler
		replaceAttr func(groups []string, a slog.Attr) slog.Attr
		want        string
	}{
		{true, slog.LevelInfo, nil, `^time=\S+ level=INFO source=/\S+/layout_handler_test.go:\d+ msg=test\n$`},
		{false, slog.LevelWarn, nil, `^$`},
		{false, nil, removeTime, `^level=INFO msg=test\n$`},
	}
	for _, tc := range tests {
		t.Run("", func(tt *testing.T) {
			t := check.T(tt)
			buf.Reset()
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				AddSource:   tc.addSource,
				Level:       tc.level,
				ReplaceAttr: tc.replaceAttr,
			}))
			logger.Info("test")
			got := buf.String()
			t.Match(got, tc.want)
		})
	}
}

func TestLayoutHandler_BadFormat(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	type F = map[string]string
	tests := []struct {
		name   string
		format F
	}{
		// Escaping % is allowed only as %%.
		{
			"single %",
			F{"bad": "%"},
		},
		{
			"odd number of %",
			F{"bad": "%%%"},
		},
		// Only allowed verbs is zero or one %v or %s.
		{
			"unknown verb",
			F{"bad": "%q"},
		},
		{
			"multiple mixed verbs",
			F{"bad": "%v%s"},
		},
		{
			"multiple v verbs",
			F{"bad": "%v%v"},
		},
		{
			"multiple s verbs",
			F{"bad": "%s%s"},
		},
		// Only allowed flags are - (left align) and .- (truncate from start).
		{
			"unknown flag +",
			F{"bad": "%+s"},
		},
		{
			"unknown flag space",
			F{"bad": "% s"},
		},
		{
			"multiple flags -",
			F{"bad": "%--s"},
		},
		{
			"multiple flags .-",
			F{"bad": "%.--s"},
		},
		// MinWidth and MaxWidth must be unsigned and fit in int.
		{
			"MinWidth=math.MaxInt64+1",
			F{"bad": "%9223372036854775808s"},
		},
		{
			"MaxWidth=math.MaxInt64+1",
			F{"bad": "%.9223372036854775808s"},
		},
		{
			"MinWidth=-1",
			F{"bad": "%--1s"},
		},
		{
			"3 widths",
			F{"bad": "%1.2.3s"},
		},
		{
			"bad width separator",
			F{"bad": "%1,2s"},
		},
		// Mix of valid and invalid formats.
		{
			"multiple bad",
			F{"bad": "%q", "also_bad": "%x"},
		},
		{
			"one bad in many",
			F{"a": "%v", "bad": "%", "c": "%s"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			t := check.T(tt)
			t.PanicMatch(func() {
				_ = slogx.NewLayoutHandler(io.Discard, &slogx.LayoutHandlerOptions{
					Format: tc.format,
				})
			}, "slogx: invalid attr format")
		})
	}
}

func TestLayoutHandler_Format(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer

	//nolint:gosmopolitan // Han script can't be enabled in config.
	tests := []struct {
		format string
		value  slog.Value
		want   string
	}{
		{"", slog.IntValue(5), `^$`},
		{"const", slog.IntValue(5), `^const$`},
		{"%v", slog.IntValue(5), `^5$`},
		{"%v", slog.StringValue(" "), `^" "$`},
		{"%v", slog.StringValue(""), `^""$`},
		{"%v", slog.AnyValue(""), `^""$`},
		{"%v", slog.AnyValue([]byte{}), `^""$`},
		{"%v", slog.AnyValue([]byte(nil)), `^""$`},
		{"%v", slog.AnyValue(nil), `^<nil>$`},
		{"%s", slog.IntValue(5), `^5$`},
		{"%s", slog.StringValue(" "), `^ $`},
		{"%s", slog.StringValue(""), `^$`},
		{"%s", slog.AnyValue(""), `^$`},
		{"%s", slog.AnyValue([]byte{}), `^$`},
		{"%s", slog.AnyValue([]byte(nil)), `^$`},
		{"%s", slog.AnyValue(nil), `^<nil>$`},
		{"prefix%v", slog.IntValue(5), `^prefix5$`},
		{"%vsuffix", slog.IntValue(5), `^5suffix$`},
		{"prefix%vsuffix", slog.IntValue(5), `^prefix5suffix$`},
		{"%%", slog.IntValue(5), `^%$`},
		{"%%%%%%", slog.IntValue(5), `^%%%$`},
		{"%%v", slog.IntValue(5), `^%v$`},
		{"%%%v%%", slog.IntValue(5), `^%5%$`},
		{"prefix%%suffix", slog.IntValue(5), `^prefix%suffix$`},
		{"prefix%vsuffix", slog.IntValue(5), `^prefix5suffix$`},
		{"%0v", slog.IntValue(5), `^5$`},
		{"%-0v", slog.IntValue(5), `^5$`},
		{"%1v", slog.IntValue(5), `^5$`},
		{"%-1v", slog.IntValue(5), `^5$`},
		{"%2v", slog.IntValue(5), `^ 5$`},
		{"%-2v", slog.IntValue(5), `^5 $`},
		{"%3v", slog.IntValue(5), `^  5$`},
		{"%-3v", slog.IntValue(5), `^5  $`},
		{"%03v", slog.IntValue(5), `^  5$`},
		{"%-03v", slog.IntValue(5), `^5  $`},
		{"%.v", slog.IntValue(5), `^$`},
		{"%-.v", slog.IntValue(5), `^$`},
		{"%.0v", slog.IntValue(5), `^$`},
		{"%-.0v", slog.IntValue(5), `^$`},
		{"%.1v", slog.IntValue(5), `^5$`},
		{"%-.1v", slog.IntValue(5), `^5$`},
		{"%.01v", slog.IntValue(5), `^5$`},
		{"%-.01v", slog.IntValue(5), `^5$`},
		{"%3.v", slog.IntValue(5), `^   $`},
		{"%-3.v", slog.IntValue(5), `^   $`},
		{"%3.0v", slog.IntValue(5), `^   $`},
		{"%-3.0v", slog.IntValue(5), `^   $`},
		{"%3.1v", slog.IntValue(5), `^  5$`},
		{"%-3.1v", slog.IntValue(5), `^5  $`},
		{"%3.5v", slog.IntValue(5), `^  5$`},
		{"%-3.5v", slog.IntValue(5), `^5  $`},
		{"%.-1v", slog.StringValue("abcde"), `^…$`},
		{"%.-2v", slog.StringValue("abcde"), `^…e$`},
		{"%.-3v", slog.StringValue("abcde"), `^…de$`},
		{"%.-4v", slog.StringValue("abcde"), `^…cde$`},
		{"%.1v", slog.StringValue("abcde"), `^…$`},
		{"%.2v", slog.StringValue("abcde"), `^a…$`},
		{"%.3v", slog.StringValue("abcde"), `^ab…$`},
		{"%.4v", slog.StringValue("abcde"), `^abc…$`},
		{"%.5v", slog.StringValue("abcde"), `^abcde$`},
		{"%.6v", slog.StringValue("abcde"), `^abcde$`},
		{"%.-1v quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%.-2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%.-3v quoted", slog.StringValue("ab=de"), `^"…" quoted$`},
		{"%.-4v quoted", slog.StringValue("ab=de"), `^"…e" quoted$`},
		{"%.-5v quoted", slog.StringValue("ab=de"), `^"…de" quoted$`},
		{"%.-6v quoted", slog.StringValue("ab=de"), `^"…=de" quoted$`},
		{"%.1v quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%.2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%.3v quoted", slog.StringValue("ab=de"), `^"…" quoted$`},
		{"%.4v quoted", slog.StringValue("ab=de"), `^"a…" quoted$`},
		{"%.5v quoted", slog.StringValue("ab=de"), `^"ab…" quoted$`},
		{"%.6v quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%.7v quoted", slog.StringValue("ab=de"), `^"ab=de" quoted$`},
		{"%.8v quoted", slog.StringValue("ab=de"), `^"ab=de" quoted$`},
		{"%.-1s unquoted", slog.StringValue("ab=de"), `^… unquoted$`},
		{"%.-2s unquoted", slog.StringValue("ab=de"), `^…e unquoted$`},
		{"%.-3s unquoted", slog.StringValue("ab=de"), `^…de unquoted$`},
		{"%.-4s unquoted", slog.StringValue("ab=de"), `^…=de unquoted$`},
		{"%.-5s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%.-6s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%.1s unquoted", slog.StringValue("ab=de"), `^… unquoted$`},
		{"%.2s unquoted", slog.StringValue("ab=de"), `^a… unquoted$`},
		{"%.3s unquoted", slog.StringValue("ab=de"), `^ab… unquoted$`},
		{"%.4s unquoted", slog.StringValue("ab=de"), `^ab=… unquoted$`},
		{"%.5s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%.6s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%1.-1v", slog.StringValue("abcde"), `^…$`},
		{"%-1.-1v", slog.StringValue("abcde"), `^…$`},
		{"%2.-1v", slog.StringValue("abcde"), `^ …$`},
		{"%-2.-1v", slog.StringValue("abcde"), `^… $`},
		{"%3.-1v", slog.StringValue("abcde"), `^  …$`},
		{"%-3.-1v", slog.StringValue("abcde"), `^…  $`},
		{"%1.-2v", slog.StringValue("abcde"), `^…e$`},
		{"%-1.-2v", slog.StringValue("abcde"), `^…e$`},
		{"%2.-2v", slog.StringValue("abcde"), `^…e$`},
		{"%-2.-2v", slog.StringValue("abcde"), `^…e$`},
		{"%3.-2v", slog.StringValue("abcde"), `^ …e$`},
		{"%-3.-2v", slog.StringValue("abcde"), `^…e $`},
		{"%4.-2v", slog.StringValue("abcde"), `^  …e$`},
		{"%-4.-2v", slog.StringValue("abcde"), `^…e  $`},
		{"%1.1v", slog.StringValue("abcde"), `^…$`},
		{"%-1.1v", slog.StringValue("abcde"), `^…$`},
		{"%2.1v", slog.StringValue("abcde"), `^ …$`},
		{"%-2.1v", slog.StringValue("abcde"), `^… $`},
		{"%3.1v", slog.StringValue("abcde"), `^  …$`},
		{"%-3.1v", slog.StringValue("abcde"), `^…  $`},
		{"%1.2v", slog.StringValue("abcde"), `^a…$`},
		{"%-1.2v", slog.StringValue("abcde"), `^a…$`},
		{"%2.2v", slog.StringValue("abcde"), `^a…$`},
		{"%-2.2v", slog.StringValue("abcde"), `^a…$`},
		{"%3.2v", slog.StringValue("abcde"), `^ a…$`},
		{"%-3.2v", slog.StringValue("abcde"), `^a… $`},
		{"%4.2v", slog.StringValue("abcde"), `^  a…$`},
		{"%-4.2v", slog.StringValue("abcde"), `^a…  $`},
		{"%4.5v", slog.StringValue("abcde"), `^abcde$`},
		{"%-4.5v", slog.StringValue("abcde"), `^abcde$`},
		{"%5.5v", slog.StringValue("abcde"), `^abcde$`},
		{"%-5.5v", slog.StringValue("abcde"), `^abcde$`},
		{"%6.5v", slog.StringValue("abcde"), `^ abcde$`},
		{"%-6.5v", slog.StringValue("abcde"), `^abcde $`},
		{"%4.6v", slog.StringValue("abcde"), `^abcde$`},
		{"%-4.6v", slog.StringValue("abcde"), `^abcde$`},
		{"%5.6v", slog.StringValue("abcde"), `^abcde$`},
		{"%-5.6v", slog.StringValue("abcde"), `^abcde$`},
		{"%6.6v", slog.StringValue("abcde"), `^ abcde$`},
		{"%-6.6v", slog.StringValue("abcde"), `^abcde $`},
		{"%1.-1v quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%-1.-1v quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%2.-1v quoted", slog.StringValue("ab=de"), `^ … quoted$`},
		{"%-2.-1v quoted", slog.StringValue("ab=de"), `^…  quoted$`},
		{"%3.-1v quoted", slog.StringValue("ab=de"), `^  … quoted$`},
		{"%-3.-1v quoted", slog.StringValue("ab=de"), `^…   quoted$`},
		{"%1.-2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%-1.-2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%2.-2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%-2.-2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%3.-2v quoted", slog.StringValue("ab=de"), `^ …… quoted$`},
		{"%-3.-2v quoted", slog.StringValue("ab=de"), `^……  quoted$`},
		{"%5.-6v quoted", slog.StringValue("ab=de"), `^"…=de" quoted$`},
		{"%-5.-6v quoted", slog.StringValue("ab=de"), `^"…=de" quoted$`},
		{"%6.-6v quoted", slog.StringValue("ab=de"), `^"…=de" quoted$`},
		{"%-6.-6v quoted", slog.StringValue("ab=de"), `^"…=de" quoted$`},
		{"%7.-6v quoted", slog.StringValue("ab=de"), `^ "…=de" quoted$`},
		{"%-7.-6v quoted", slog.StringValue("ab=de"), `^"…=de"  quoted$`},
		{"%1.-1s unquoted", slog.StringValue("ab=de"), `^… unquoted$`},
		{"%-1.-1s unquoted", slog.StringValue("ab=de"), `^… unquoted$`},
		{"%2.-1s unquoted", slog.StringValue("ab=de"), `^ … unquoted$`},
		{"%-2.-1s unquoted", slog.StringValue("ab=de"), `^…  unquoted$`},
		{"%3.-1s unquoted", slog.StringValue("ab=de"), `^  … unquoted$`},
		{"%-3.-1s unquoted", slog.StringValue("ab=de"), `^…   unquoted$`},
		{"%1.-2s unquoted", slog.StringValue("ab=de"), `^…e unquoted$`},
		{"%-1.-2s unquoted", slog.StringValue("ab=de"), `^…e unquoted$`},
		{"%2.-2s unquoted", slog.StringValue("ab=de"), `^…e unquoted$`},
		{"%-2.-2s unquoted", slog.StringValue("ab=de"), `^…e unquoted$`},
		{"%3.-2s unquoted", slog.StringValue("ab=de"), `^ …e unquoted$`},
		{"%-3.-2s unquoted", slog.StringValue("ab=de"), `^…e  unquoted$`},
		{"%5.-6s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%-5.-6s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%6.-6s unquoted", slog.StringValue("ab=de"), `^ ab=de unquoted$`},
		{"%-6.-6s unquoted", slog.StringValue("ab=de"), `^ab=de  unquoted$`},
		{"%7.-6s unquoted", slog.StringValue("ab=de"), `^  ab=de unquoted$`},
		{"%-7.-6s unquoted", slog.StringValue("ab=de"), `^ab=de   unquoted$`},
		{"%1.1v quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%-1.1v quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%2.1v quoted", slog.StringValue("ab=de"), `^ … quoted$`},
		{"%-2.1v quoted", slog.StringValue("ab=de"), `^…  quoted$`},
		{"%3.1v quoted", slog.StringValue("ab=de"), `^  … quoted$`},
		{"%-3.1v quoted", slog.StringValue("ab=de"), `^…   quoted$`},
		{"%1.2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%-1.2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%2.2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%-2.2v quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%3.2v quoted", slog.StringValue("ab=de"), `^ …… quoted$`},
		{"%-3.2v quoted", slog.StringValue("ab=de"), `^……  quoted$`},
		{"%5.6v quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%-5.6v quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%6.6v quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%-6.6v quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%7.6v quoted", slog.StringValue("ab=de"), `^ "ab=…" quoted$`},
		{"%-7.6v quoted", slog.StringValue("ab=de"), `^"ab=…"  quoted$`},
		{"%1.1s unquoted", slog.StringValue("ab=de"), `^… unquoted$`},
		{"%-1.1s unquoted", slog.StringValue("ab=de"), `^… unquoted$`},
		{"%2.1s unquoted", slog.StringValue("ab=de"), `^ … unquoted$`},
		{"%-2.1s unquoted", slog.StringValue("ab=de"), `^…  unquoted$`},
		{"%3.1s unquoted", slog.StringValue("ab=de"), `^  … unquoted$`},
		{"%-3.1s unquoted", slog.StringValue("ab=de"), `^…   unquoted$`},
		{"%1.2s unquoted", slog.StringValue("ab=de"), `^a… unquoted$`},
		{"%-1.2s unquoted", slog.StringValue("ab=de"), `^a… unquoted$`},
		{"%2.2s unquoted", slog.StringValue("ab=de"), `^a… unquoted$`},
		{"%-2.2s unquoted", slog.StringValue("ab=de"), `^a… unquoted$`},
		{"%3.2s unquoted", slog.StringValue("ab=de"), `^ a… unquoted$`},
		{"%-3.2s unquoted", slog.StringValue("ab=de"), `^a…  unquoted$`},
		{"%5.6s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%-5.6s unquoted", slog.StringValue("ab=de"), `^ab=de unquoted$`},
		{"%6.6s unquoted", slog.StringValue("ab=de"), `^ ab=de unquoted$`},
		{"%-6.6s unquoted", slog.StringValue("ab=de"), `^ab=de  unquoted$`},
		{"%7.6s unquoted", slog.StringValue("ab=de"), `^  ab=de unquoted$`},
		{"%-7.6s unquoted", slog.StringValue("ab=de"), `^ab=de   unquoted$`},
		{"%.-1v utf8", slog.StringValue("😄世ЮЯ界😊"), `^… utf8$`},
		{"%.-2v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…😊 utf8$`},
		{"%.-3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…界😊 utf8$`},
		{"%.-4v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…Я界😊 utf8$`},
		{"%.-5v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…ЮЯ界😊 utf8$`},
		{"%.1v utf8", slog.StringValue("😄世ЮЯ界😊"), `^… utf8$`},
		{"%.2v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄… utf8$`},
		{"%.3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%.4v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世Ю… utf8$`},
		{"%.5v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世ЮЯ… utf8$`},
		{"%.6v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世ЮЯ界😊 utf8$`},
		{"%.7v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世ЮЯ界😊 utf8$`},
		{"%2.-3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…界😊 utf8$`},
		{"%-2.-3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…界😊 utf8$`},
		{"%3.-3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…界😊 utf8$`},
		{"%-3.-3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…界😊 utf8$`},
		{"%4.-3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^ …界😊 utf8$`},
		{"%-4.-3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^…界😊  utf8$`},
		{"%2.3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%-2.3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%3.3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%-3.3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%4.3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^ 😄世… utf8$`},
		{"%-4.3v utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世…  utf8$`},
		{"%.-1v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^… utf8 quoted$`},
		{"%.-2v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^…… utf8 quoted$`},
		{"%.-3v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…" utf8 quoted$`},
		{"%.-4v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…😊" utf8 quoted$`},
		{"%.-5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…界😊" utf8 quoted$`},
		{"%.-6v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…Я界😊" utf8 quoted$`},
		{"%.-7v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…=Я界😊" utf8 quoted$`},
		{"%.-8v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…Ю=Я界😊" utf8 quoted$`},
		{"%.-1s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^… utf8 unquoted$`},
		{"%.-2s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…😊 utf8 unquoted$`},
		{"%.-3s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…界😊 utf8 unquoted$`},
		{"%.-4s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…Я界😊 utf8 unquoted$`},
		{"%.-5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…=Я界😊 utf8 unquoted$`},
		{"%.-6s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…Ю=Я界😊 utf8 unquoted$`},
		{"%.-7s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=Я界😊 utf8 unquoted$`},
		{"%.-8s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=Я界😊 utf8 unquoted$`},
		{"%.1v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^… utf8 quoted$`},
		{"%.2v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^…… utf8 quoted$`},
		{"%.3v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…" utf8 quoted$`},
		{"%.4v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄…" utf8 quoted$`},
		{"%.5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%.6v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю…" utf8 quoted$`},
		{"%.7v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=…" utf8 quoted$`},
		{"%.8v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=Я…" utf8 quoted$`},
		{"%.9v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=Я界😊" utf8 quoted$`},
		{"%.10v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=Я界😊" utf8 quoted$`},
		{"%.1s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^… utf8 unquoted$`},
		{"%.2s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄… utf8 unquoted$`},
		{"%.3s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世… utf8 unquoted$`},
		{"%.4s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю… utf8 unquoted$`},
		{"%.5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=… utf8 unquoted$`},
		{"%.6s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=Я… utf8 unquoted$`},
		{"%.7s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=Я界😊 utf8 unquoted$`},
		{"%.8s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=Я界😊 utf8 unquoted$`},
		{"%4.-5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…界😊" utf8 quoted$`},
		{"%-4.-5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…界😊" utf8 quoted$`},
		{"%5.-5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…界😊" utf8 quoted$`},
		{"%-5.-5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…界😊" utf8 quoted$`},
		{"%6.-5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^ "…界😊" utf8 quoted$`},
		{"%-6.-5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…界😊"  utf8 quoted$`},
		{"%4.5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%-4.5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%5.5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%-5.5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%6.5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^ "😄世…" utf8 quoted$`},
		{"%-6.5v utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…"  utf8 quoted$`},
		{"%4.-5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…=Я界😊 utf8 unquoted$`},
		{"%-4.-5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…=Я界😊 utf8 unquoted$`},
		{"%5.-5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…=Я界😊 utf8 unquoted$`},
		{"%-5.-5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…=Я界😊 utf8 unquoted$`},
		{"%6.-5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^ …=Я界😊 utf8 unquoted$`},
		{"%-6.-5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^…=Я界😊  utf8 unquoted$`},
		{"%4.5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=… utf8 unquoted$`},
		{"%-4.5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=… utf8 unquoted$`},
		{"%5.5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=… utf8 unquoted$`},
		{"%-5.5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=… utf8 unquoted$`},
		{"%6.5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^ 😄世Ю=… utf8 unquoted$`},
		{"%-6.5s utf8 unquoted", slog.StringValue("😄世Ю=Я界😊"), `^😄世Ю=…  utf8 unquoted$`},
	}
	reNoAlternate := regexp.MustCompile(`^%-?\d*[.]\d*[vs]`)
	for _, tc := range tests {
		formats := []string{tc.format}
		if reNoAlternate.MatchString(tc.format) && !strings.Contains(tc.want, "…") {
			idx := strings.Index(tc.format, ".")
			formats = append(formats, tc.format[:idx+1]+"-"+tc.format[idx+1:])
		}
		for _, format := range formats {
			t.Run(format, func(tt *testing.T) {
				t := check.T(tt)
				buf.Reset()
				logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
					Format: map[string]string{
						slog.TimeKey:    "",
						slog.LevelKey:   "",
						slog.MessageKey: "",
						"value":         format,
					},
				}))
				logger.Info("test", "value", tc.value)
				got := buf.String()
				t.Must(t.NotEqual(got, ""))
				t.Must(t.Equal(got[len(got)-1], byte('\n')))
				t.Match(got[:len(got)-1], tc.want)
			})
		}
	}
}

func TestLayoutHandler_FormatSpecial(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer

	type F = map[string]string
	tests := []struct {
		format F
		level  slog.Level
		want   string
	}{
		{nil, -2, "time=2006-01-02T15:04:05.789+01:00 level=DEBUG+2 msg=test"},
		{nil, 4, "time=2006-01-02T15:04:05.789+01:00 level=WARN msg=test"},
		{nil, 6, "time=2006-01-02T15:04:05.789+01:00 level=WARN+2 msg=test"},
		{nil, 8, "time=2006-01-02T15:04:05.789+01:00 level=ERROR msg=test"},
		{F{"level": " %.3v"}, -2, "time=2006-01-02T15:04:05.789+01:00 DE… msg=test"},
		{F{"level": " %.3v"}, 4, "time=2006-01-02T15:04:05.789+01:00 WA… msg=test"},
		{F{"level": " %.3v"}, 6, "time=2006-01-02T15:04:05.789+01:00 WA… msg=test"},
		{F{"level": " %.3v"}, 8, "time=2006-01-02T15:04:05.789+01:00 ER… msg=test"},
		{F{"level": " %3v"}, -2, "time=2006-01-02T15:04:05.789+01:00 DEBUG+2 msg=test"},
		{F{"level": " %3v"}, 4, "time=2006-01-02T15:04:05.789+01:00 WARN msg=test"},
		{F{"level": " %3v"}, 6, "time=2006-01-02T15:04:05.789+01:00 WARN+2 msg=test"},
		{F{"level": " %3v"}, 8, "time=2006-01-02T15:04:05.789+01:00 ERROR msg=test"},
		{F{"level": " %4.4v"}, -2, "time=2006-01-02T15:04:05.789+01:00 DEB… msg=test"},
		{F{"level": " %4.4v"}, 4, "time=2006-01-02T15:04:05.789+01:00 WARN msg=test"},
		{F{"level": " %4.4v"}, 6, "time=2006-01-02T15:04:05.789+01:00 WAR… msg=test"},
		{F{"level": " %4.4v"}, 8, "time=2006-01-02T15:04:05.789+01:00 ERR… msg=test"},
		{F{"level": " %3.3v"}, -2, "time=2006-01-02T15:04:05.789+01:00 D+2 msg=test"},
		{F{"level": " %3.3v"}, 4, "time=2006-01-02T15:04:05.789+01:00 WRN msg=test"},
		{F{"level": " %3.3v"}, 6, "time=2006-01-02T15:04:05.789+01:00 W+2 msg=test"},
		{F{"level": " %3.3v"}, 8, "time=2006-01-02T15:04:05.789+01:00 ERR msg=test"},
		{F{"level": " %-3.3v"}, -2, "time=2006-01-02T15:04:05.789+01:00 D+2 msg=test"},
		{F{"level": " %-3.3v"}, 4, "time=2006-01-02T15:04:05.789+01:00 WRN msg=test"},
		{F{"level": " %-3.3v"}, 6, "time=2006-01-02T15:04:05.789+01:00 W+2 msg=test"},
		{F{"level": " %-3.3v"}, 8, "time=2006-01-02T15:04:05.789+01:00 ERR msg=test"},
		{F{"level": " %2.2v"}, -2, "time=2006-01-02T15:04:05.789+01:00 D… msg=test"},
		{F{"level": " %2.2v"}, 4, "time=2006-01-02T15:04:05.789+01:00 W… msg=test"},
		{F{"level": " %2.2v"}, 6, "time=2006-01-02T15:04:05.789+01:00 W… msg=test"},
		{F{"level": " %2.2v"}, 8, "time=2006-01-02T15:04:05.789+01:00 E… msg=test"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s %+v", tc.level, tc.format), func(tt *testing.T) {
			t := check.T(tt)
			buf.Reset()
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				Format: tc.format,
				Level:  slog.LevelDebug,
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == slog.TimeKey && len(groups) == 0 {
						now, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.789123456+01:00")
						a.Value = slog.TimeValue(now)
					}
					return a
				},
			}))
			logger.Log(t.Context(), tc.level, "test")
			t.Equal(buf.String(), tc.want+"\n")
		})
	}
}

func TestLayoutHandler_Layout(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer

	type F = map[string]string
	type L = []string
	tests := []struct {
		name       string
		format     F
		prefixKeys L
		suffixKeys L
		want       string
	}{
		// Corner cases.
		{
			"all nil",
			nil,
			nil,
			nil,
			`^time=\S+ level=INFO msg=test a=1 b=2 c=3 d=4 e=5$`,
		},
		{
			"all empty",
			make(F),
			L{},
			L{},
			`^time=\S+ level=INFO msg=test a=1 b=2 c=3 d=4 e=5$`,
		},
		{
			"nothing",
			F{"time": "", "level": "", "msg": "", "a": "", "b": "", "c": "", "d": "", "e": ""},
			nil,
			nil,
			`^$`,
		},
		{
			"format everything",
			F{"time": "", "level": "%v", "msg": "%v", "a": "%v", "b": "%v", "c": "%v", "d": "%v", "e": "%v"},
			nil,
			nil,
			`^INFOtest12345$`,
		},
		{
			"format everything reordered",
			F{"time": "", "level": "%v", "msg": "%v", "a": "%v", "b": "%v", "c": "%v", "d": "%v", "e": "%v"},
			L{"e", "d"},
			L{"b", "a"},
			`^INFO54test321$`,
		},
		{
			"format everything no std",
			F{"time": "", "level": "", "msg": "", "a": "%v", "b": "%v", "c": "%v", "d": "%v", "e": "%v"},
			L{"e", "d"},
			L{"b", "a"},
			`^54321$`,
		},
		// Excluding keys.
		{
			"all except time",
			F{"time": ""},
			nil,
			nil,
			`^level=INFO msg=test a=1 b=2 c=3 d=4 e=5$`,
		},
		{
			"only level",
			F{"time": "", "msg": "", "a": "", "b": "", "c": "", "d": "", "e": ""},
			nil,
			nil,
			`^level=INFO$`,
		},
		{
			"only msg",
			F{"time": "", "level": "", "a": "", "b": "", "c": "", "d": "", "e": ""},
			nil,
			nil,
			`^msg=test$`,
		},
		{
			"only a",
			F{"time": "", "level": "", "msg": "", "b": "", "c": "", "d": "", "e": ""},
			nil,
			nil,
			`^a=1$`,
		},
		{
			"only level and msg",
			F{"time": "", "a": "", "b": "", "c": "", "d": "", "e": ""},
			nil,
			nil,
			`^level=INFO msg=test$`,
		},
		{
			"only time and a",
			F{"level": "", "msg": "", "b": "", "c": "", "d": "", "e": ""},
			nil,
			nil,
			`^time=\S+ a=1$`,
		},
		{
			"only msg and a",
			F{"time": "", "level": "", "b": "", "c": "", "d": "", "e": ""},
			nil,
			nil,
			`^msg=test a=1$`,
		},
		{
			"only a and c",
			F{"time": "", "level": "", "msg": "", "b": "", "d": "", "e": ""},
			nil,
			nil,
			`^a=1 c=3$`,
		},
		// Ordering keys.
		{
			"prefix c",
			nil,
			L{"c"},
			nil,
			`^time=\S+ level=INFO c=3 msg=test a=1 b=2 d=4 e=5$`,
		},
		{
			"suffix c",
			nil,
			nil,
			L{"c"},
			`^time=\S+ level=INFO msg=test a=1 b=2 d=4 e=5 c=3$`,
		},
		{
			"prefix b suffix d",
			nil,
			L{"b"},
			L{"d"},
			`^time=\S+ level=INFO b=2 msg=test a=1 c=3 e=5 d=4$`,
		},
		{
			"prefix e d suffix b a",
			nil,
			L{"e", "d"},
			L{"b", "a"},
			`^time=\S+ level=INFO e=5 d=4 msg=test c=3 b=2 a=1$`,
		},
		{
			"prefix e d c b a",
			nil,
			L{"e", "d", "c", "b", "a"},
			nil,
			`^time=\S+ level=INFO e=5 d=4 c=3 b=2 a=1 msg=test$`,
		},
		{
			"suffix e d c b a",
			nil,
			nil,
			L{"e", "d", "c", "b", "a"},
			`^time=\S+ level=INFO msg=test e=5 d=4 c=3 b=2 a=1$`,
		},
		{
			"prefix suffix duplicates and missing",
			nil,
			L{"e", "d", "b", "bad", "bad1"},
			L{"d", "b", "a", "bad", "bad2"},
			`^time=\S+ level=INFO e=5 d=4 b=2 msg=test c=3 a=1$`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			t := check.T(tt)

			opts1 := slogx.LayoutHandlerOptions{
				Format:     tc.format,
				PrefixKeys: tc.prefixKeys,
				SuffixKeys: tc.suffixKeys,
			}
			opts2 := optsFormatToReplaceAttr(opts1)
			logger1 := slog.New(slogx.NewLayoutHandler(&buf, &opts1))
			logger2 := slog.New(slogx.NewLayoutHandler(&buf, &opts2))

			for i, logger := range []*slog.Logger{logger1, logger2} {
				buf.Reset()
				logger.Info("test", "a", 1, "b", 2, "c", 3, "d", 4, "e", 5)
				got := buf.String()
				t.Must(t.NotEqual(got, ""))
				t.Must(t.Equal(got[len(got)-1], byte('\n')))
				t.Match(got[:len(got)-1], tc.want, "logger%d", i+1)
			}
		})
	}
}

func TestLayoutHandler_LayoutWith(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey:  "",
			slog.LevelKey: "",
			"g.p":         " G.p=%v",
			"g.s":         " G.s=%v",
			"g.g2.p":      " g.G2.p=%v",
			"g.g2.s":      " g.G2.s=%v",
		},
		PrefixKeys: []string{"g.p", "p", "g.g2.p"},
		SuffixKeys: []string{"g.s", "s", "g.g2.s"},
	}))
	tests := []struct {
		name string
		f    func()
		want string
	}{
		{
			"replace",
			func() {
				logger.With("p", -100, "s", 42, "a", 10).
					With("s", 2, "s", 3, "a", 20, "a", 30).
					Info("Test", "p", -3, "a", 40)
			},
			`^p=-3 msg=Test a=10 a=20 a=30 a=40 s=3$`,
		},
		{
			"replace in group",
			func() {
				logger.With(slog.Group("g", "p", -1, "s", 1, "a", "A")).
					Info("Test", slog.Group("g", "p", -2, "s", 2))
			},
			`^ G.p=-2 msg=Test g.a=A G.s=2$`,
		},
		{
			"WithGroup",
			func() {
				logger.With("p", -1, "s", 1).
					WithGroup("g").
					With("p", -2, "s", 2).
					WithGroup("g2").
					Info("Test", "p", -3, "s", 3, "a", 0)
			},
			`^ G.p=-2 p=-1 g.G2.p=-3 msg=Test g.g2.a=0 G.s=2 s=1 g.G2.s=3$`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			t := check.T(tt)
			buf.Reset()
			tc.f()
			got := buf.String()
			t.Must(t.NotEqual(got, ""))
			t.Must(t.Equal(got[len(got)-1], byte('\n')))
			t.Match(got[:len(got)-1], tc.want)
		})
	}
}

func TestLayoutHandler_AttrSep(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	const (
		attrStd    = 1 << iota // time, level
		attrPrefix             // attrs listed in PrefixKeys and added using slog.With or slog.Info
		attrMsg                // msg
		attrWith               // preformatted attrs added using slog.With not listed in prefix or suffix
		attrNormal             // normal attrs passed to slog.Info not listed in prefix or suffix
		attrSuffix             // attrs listed in SuffixKeys and added using slog.With or slog.Info
		attrMax
	)
	const (
		amountOne  = "one"
		amountMany = "many"
	)

	for attrMask := range attrMax {
		opts1 := slogx.LayoutHandlerOptions{
			Format:     make(map[string]string),
			PrefixKeys: []string{"pre1", "pre2"},
			SuffixKeys: []string{"suf1", "suf2"},
		}
		if attrMask&attrStd == 0 {
			opts1.Format[slog.TimeKey] = ""
			opts1.Format[slog.LevelKey] = ""
		}
		if attrMask&attrPrefix == 0 {
			opts1.Format["pre1"] = ""
			opts1.Format["pre2"] = ""
		}
		if attrMask&attrMsg == 0 {
			opts1.Format[slog.MessageKey] = ""
		}
		if attrMask&attrWith == 0 {
			opts1.Format["with1"] = ""
			opts1.Format["with2"] = ""
		}
		if attrMask&attrNormal == 0 {
			opts1.Format["norm1"] = ""
			opts1.Format["norm2"] = ""
		}
		if attrMask&attrSuffix == 0 {
			opts1.Format["suf1"] = ""
			opts1.Format["suf2"] = ""
		}
		opts2 := opts1
		opts2.Format = maps.Clone(opts1.Format)
		for _, key := range []string{
			slog.TimeKey, slog.LevelKey,
			"pre1", "pre2",
			slog.MessageKey,
			"with1", "with2",
			"norm1", "norm2",
			"suf1", "suf2",
		} {
			if _, ok := opts2.Format[key]; !ok {
				opts2.Format[key] = " " + key + "=%v"
			}
		}
		opts3 := optsFormatToReplaceAttr(opts1)
		opts4 := optsFormatToReplaceAttr(opts2)
		for i, opts := range []slogx.LayoutHandlerOptions{opts1, opts2, opts3, opts4} {
			for _, amount := range []string{amountOne, amountMany} {
				// Test all combinations of:
				// - presence/absence of each attribute group (attrMask)
				//   (std, prefix, msg, with, normal, suffix)
				// - default or custom format (opts1/3 vs opts2/4)
				// - removal by format "" vs replaceAttr (opts1/2 vs opts3/4)
				// - single vs multiple attributes in each group (amount)
				t.Run(fmt.Sprintf("%06b opts%d %s", attrMask, i, amount), func(tt *testing.T) {
					t := check.T(tt)
					t.Parallel()
					var buf bytes.Buffer
					logger := slog.New(slogx.NewLayoutHandler(&buf, &opts))

					wants := []string{}
					if i == 1 || i == 3 {
						wants = append(wants, ``)
					}
					if attrMask&attrStd != 0 {
						wants = append(wants, `time=\S+`, `level=INFO`)
					}
					if attrMask&attrPrefix != 0 {
						wants = append(wants, `pre1=PRE1`)
						if amount == amountMany {
							wants = append(wants, `pre2=PRE2`)
						}
					}
					if attrMask&attrMsg != 0 {
						wants = append(wants, `msg=test`)
					}
					if attrMask&attrWith != 0 {
						wants = append(wants, `with1=WITH1`)
						if amount == amountMany {
							wants = append(wants, `with2=WITH2`)
						}
					}
					if attrMask&attrNormal != 0 {
						wants = append(wants, `norm1=NORM1`)
						if amount == amountMany {
							wants = append(wants, `norm2=NORM2`)
						}
					}
					if attrMask&attrSuffix != 0 {
						wants = append(wants, `suf1=SUF1`)
						if amount == amountMany {
							wants = append(wants, `suf2=SUF2`)
						}
					}
					want := "^" + strings.Join(wants, " ") + "$"

					withAttrs := []any{
						slog.String("with1", "WITH1"),
					}
					normAttrs := []any{
						slog.String("norm1", "NORM1"),
						slog.String("pre1", "PRE1"),
						slog.String("suf1", "SUF1"),
					}
					if amount == amountMany {
						withAttrs = append(withAttrs,
							slog.String("with2", "WITH2"))
						normAttrs = append(normAttrs,
							slog.String("norm2", "NORM2"),
							slog.String("pre2", "PRE2"),
							slog.String("suf2", "SUF2"))
					}
					logger.With(withAttrs...).Info("test", normAttrs...)

					got := buf.String()
					t.Must(t.NotEqual(got, ""))
					t.Must(t.Equal(got[len(got)-1], byte('\n')))
					t.Match(got[:len(got)-1], want)
				})
			}
		}
	}
}

func optsFormatToReplaceAttr(opts slogx.LayoutHandlerOptions) slogx.LayoutHandlerOptions {
	excludedKeys := []string{}
	format := make(map[string]string)
	for k, v := range opts.Format {
		if v == "" {
			excludedKeys = append(excludedKeys, k)
		} else {
			format[k] = v
		}
	}
	replaceAttr := func(groups []string, a slog.Attr) slog.Attr {
		key := a.Key
		if len(groups) > 0 {
			key = strings.Join(groups, ".") + "." + a.Key
		}
		if slices.Contains(excludedKeys, key) {
			return slog.Attr{}
		}
		return a
	}
	if len(excludedKeys) == 0 {
		replaceAttr = nil
	}
	if opts.ReplaceAttr != nil {
		if replaceAttr == nil {
			replaceAttr = opts.ReplaceAttr
		} else {
			replaceAttr = slogx.ChainReplaceAttr(opts.ReplaceAttr, replaceAttr)
		}
	}
	return slogx.LayoutHandlerOptions{
		AddSource:   opts.AddSource,
		Level:       opts.Level,
		ReplaceAttr: replaceAttr,
		Format:      format,
		PrefixKeys:  opts.PrefixKeys,
		SuffixKeys:  opts.SuffixKeys,
	}
}

func TestLayoutHandler_TimeFormat(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()
	var buf bytes.Buffer

	tests := []struct {
		recordTimeFormat string
		timeFormat       string
		want             string
	}{
		{
			"", "",
			"time=2006-01-02T15:04:05.789+01:00 t=2006-01-02T15:04:05.789+01:00",
		},
		{
			time.RFC3339, "",
			"time=2006-01-02T15:04:05+01:00 t=2006-01-02T15:04:05.789+01:00",
		},
		{
			"", time.RFC3339,
			"time=2006-01-02T15:04:05.789+01:00 t=2006-01-02T15:04:05+01:00",
		},
		{
			time.RFC3339, time.RFC3339,
			"time=2006-01-02T15:04:05+01:00 t=2006-01-02T15:04:05+01:00",
		},
		{
			time.RFC3339Nano, time.Kitchen,
			"time=2006-01-02T15:04:05.789123456+01:00 t=3:04PM",
		},
		{
			time.DateOnly, time.TimeOnly,
			"time=2006-01-02 t=15:04:05",
		},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s %s", tc.recordTimeFormat, tc.timeFormat), func(tt *testing.T) {
			t := check.T(tt)
			buf.Reset()
			now, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.789123456+01:00")
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				Format: map[string]string{
					slog.LevelKey:   "",
					slog.MessageKey: "",
				},
				RecordTimeFormat: tc.recordTimeFormat,
				TimeFormat:       tc.timeFormat,
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == slog.TimeKey && len(groups) == 0 {
						a.Value = slog.TimeValue(now)
					}
					return a
				},
			}))
			logger.Info("test", "t", now)
			t.Equal(buf.String(), tc.want+"\n")
		})
	}
}

func BenchmarkLayout(b *testing.B) {
	opts := slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.LevelKey: " %-5v",
			// prefix
			"app":        " %12.12v:",
			"pkg":        " %9.9v:",
			"server":     " [%v]",
			"remoteIP":   " %-15v",
			"requestID":  " %v",
			"grpcCode":   " %-16.16v",
			"httpCode":   " %3v",
			"httpMethod": "      %7v",
			"handler":    " %v:",
			"op":         " %v:",
			"service":    " %v",
			"method":     " %v:",
			// normal
			"addr":    " %v",
			"host":    " %v",
			"port":    ":%v",
			"version": " version %v",
			"offset":  " page=%3v",
			"limit":   "+%v",
			"err":     " err: %v",
			// suffix
			"userID":    " @%v",
			"accountID": ":%v",
		},
		PrefixKeys: []string{
			"app",
			"pkg",
			"server",
			"remoteIP",
			"requestID",
			"grpcCode",
			"httpCode",
			"httpMethod",
			"handler",
			"op",
			"service",
			"method",
		},
		SuffixKeys: []string{
			"userID",
			"accountID",
			slog.SourceKey,
			slogx.StackKey,
		},
	}
	for _, handler := range []struct {
		name string
		h    slog.Handler
	}{
		{"all-opts", slogx.NewLayoutHandler(io.Discard, &opts)},
		{"no-opts", slogx.NewLayoutHandler(io.Discard, nil)},
		{"std-text", slog.NewTextHandler(io.Discard, nil)}, //nolint:sloglint // Benchmark.
	} {
		logger := slog.New(handler.h)
		b.Run(handler.name, func(b *testing.B) {
			for _, call := range []struct {
				name string
				f    func()
			}{
				{"msg", func() {
					logger.Info("test")
				}},
				{"http", func() {
					logger2 := logger.
						// set in main()
						With("app", "myapp").
						// set in HTTP middleware
						With(
							"server", "HTTP",
							"remoteIP", "127.0.0.1",
							"httpCode", "", // placeholder
							"httpMethod", "GET",
							"handler", "/v1/thing",
						).
						// set in auth middleware
						With(
							"userID", "user-1234",
							"accountID", "account-5678",
						)
					logger2.Warn("something happened",
						"pkg", "something",
						"method", "doSomething",
						"err", io.EOF)
					logger2.Info("handled request",
						"pkg", "mypkg",
						"httpCode", 200)
				}},
			} {
				b.Run(call.name, func(b *testing.B) {
					b.ReportAllocs()
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							call.f()
						}
					})
				})
			}
		})
	}
}
