package slogx_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
	"github.com/powerman/slogx/internal/race"
)

func TestLayoutHandler(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	var buf bytes.Buffer
	h := slogx.NewLayoutHandler(&buf, nil)
	t.Nil(slogtest.TestHandler(h, makeTextResults(t, &buf)))
}

func TestLayoutHandler_WithEmpty(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)
	h := slogx.NewLayoutHandler(io.Discard, nil)
	t.True(h.WithAttrs(nil) == slog.Handler(h))
	t.True(h.WithAttrs([]slog.Attr{slog.Group("g")}) == slog.Handler(h))
	t.True(h.WithGroup("") == slog.Handler(h))
	t.True(h.WithGroup("g") != slog.Handler(h))
}

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestLayoutHandler_StdOptions(t *testing.T) {
	var buf bytes.Buffer

	tests := []struct {
		addSource   bool
		level       slog.Leveler
		replaceAttr func(groups []string, a slog.Attr) slog.Attr
		want        string
	}{
		{true, slog.LevelInfo, nil, `^time=\S+ level=INFO source=\S+/layout_handler_test.go:\d+ msg=test\n$`},
		{false, slog.LevelWarn, nil, `^$`},
		{false, nil, removeTime, `^level=INFO msg=test\n$`},
	}
	for _, tc := range tests {
		t.Run("", func(tt *testing.T) {
			t := check.Must(tt)
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

func TestLayoutHandler_BadFormat(t *testing.T) {
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
			tt.Parallel()
			t := check.Must(tt)
			t.PanicMatch(func() {
				_ = slogx.NewLayoutHandler(io.Discard, &slogx.LayoutHandlerOptions{
					Format: tc.format,
				})
			}, "slogx: invalid attr format")
		})
	}
}

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestLayoutHandler_Format(t *testing.T) {
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
				t := check.Must(tt)
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

// Each byte of invalid UTF-8 counts as one rune for width and truncation.
// Not part of TestLayoutHandler_Format because a regexp can't contain invalid UTF-8.
func TestLayoutHandler_FormatInvalidUTF8(t *testing.T) {
	t.Parallel()
	const value = "a\xffb\xe4\xb8c\xf0\x9f\x98"
	tests := []struct {
		format string
		want   string
	}{
		{"%.-3s", "…\x9f\x98"},
		{"%.-5s", "…c\xf0\x9f\x98"},
		{"%.3s", "a\xff…"},
		{"%.5s", "a\xffb\xe4…"},
		{"%-9.-5s", "…c\xf0\x9f\x98    "},
		{"%.-6v", `"…x98"`},
		{"%.6v", `"a\x…"`},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			var buf bytes.Buffer
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				Format: map[string]string{
					slog.TimeKey:    "",
					slog.LevelKey:   "",
					slog.MessageKey: "",
					"value":         tc.format,
				},
			}))
			logger.Info("test", "value", value)
			t.Equal(buf.String(), tc.want+"\n")
		})
	}
}

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestLayoutHandler_FormatSpecial(t *testing.T) {
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
			t := check.Must(tt)
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

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestLayoutHandler_Layout(t *testing.T) {
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
		// Built-in keys without Format.
		{
			"suffix time",
			nil,
			nil,
			L{"time"},
			`^level=INFO msg=test a=1 b=2 c=3 d=4 e=5 time=\S+$`,
		},
		{
			"suffix level",
			nil,
			nil,
			L{"level"},
			`^time=\S+ msg=test a=1 b=2 c=3 d=4 e=5 level=INFO$`,
		},
		{
			"prefix level time",
			nil,
			L{"level", "time"},
			nil,
			`^level=INFO time=\S+ msg=test a=1 b=2 c=3 d=4 e=5$`,
		},
		{
			"prefix time suffix level",
			nil,
			L{"time"},
			L{"level"},
			`^time=\S+ msg=test a=1 b=2 c=3 d=4 e=5 level=INFO$`,
		},
		{
			"prefix level suffix time",
			nil,
			L{"level"},
			L{"time"},
			`^level=INFO msg=test a=1 b=2 c=3 d=4 e=5 time=\S+$`,
		},
		{
			"suffix level time with prefix attr",
			nil,
			L{"e"},
			L{"level", "time", "a"},
			`^e=5 msg=test b=2 c=3 d=4 level=INFO time=\S+ a=1$`,
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
			t := check.Must(tt)

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

// A built-in attr which is a layout key must not leak into the next record,
// even when that record has no attrs and no value for this built-in attr.
func TestLayoutHandler_BuiltInLayoutKeyNotShared(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	var buf bytes.Buffer
	h := slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
		Format:     map[string]string{slog.TimeKey: "[%v]", slog.LevelKey: " %v"},
		PrefixKeys: []string{slog.TimeKey, slog.LevelKey},
	})
	ctx := tt.Context()

	withTime := slog.NewRecord(time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC), slog.LevelInfo, "first", 0)
	t.Nil(h.Handle(ctx, withTime))
	t.Match(buf.String(), `^\[2006-01-02T15:04:05\.000Z\] INFO msg=first\n$`)

	buf.Reset()
	noTime := slog.NewRecord(time.Time{}, slog.LevelInfo, "second", 0)
	t.Nil(h.Handle(ctx, noTime))
	t.Equal(buf.String(), " INFO msg=second\n")
}

// A record attr which overrides a layout attr added by With
// must not modify the value used by the next records.
func TestLayoutHandler_WithLayoutKeyNotShared(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	var buf bytes.Buffer
	logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
		Format:     map[string]string{slog.TimeKey: "", slog.LevelKey: ""},
		PrefixKeys: []string{"p"},
		SuffixKeys: []string{"s"},
	})).With("p", "WITH", "s", "WITH")

	logger.Info("1", "p", "REC", "s", "REC")
	logger.Info("2")
	t.Equal(buf.String(), "p=REC msg=1 s=REC\np=WITH msg=2 s=WITH\n")
}

// Layout position depends only on the key,
// so built-in attrs and other attrs with the same key are handled the same way:
// the output must match the output of slog.TextHandler
// with the last attr of each layout key moved to its position.
func TestLayoutHandler_BuiltInLayoutKeysMatchTextHandler(tt *testing.T) {
	tt.Parallel()

	var pcs [1]uintptr
	runtime.Callers(1, pcs[:])
	ts := time.Date(2006, time.January, 2, 15, 4, 5, 789000000, time.UTC)

	layouts := []struct {
		prefixKeys []string
		suffixKeys []string
	}{
		{nil, nil},
		{[]string{slog.TimeKey, slog.LevelKey, slog.SourceKey}, nil},
		{nil, []string{slog.LevelKey, slog.SourceKey, slog.TimeKey}},
		{[]string{slog.LevelKey}, []string{slog.TimeKey, slog.SourceKey}},
	}
	records := []struct {
		name string
		time time.Time
		with []slog.Attr
		args []any
	}{
		{"no attrs", ts, nil, nil},
		{"zero time", time.Time{}, []slog.Attr{slog.String("time", "WITH")}, []any{"time", "REC"}},
		{"With", ts, []slog.Attr{slog.String("level", "WITH"), slog.String("time", "WITH")}, nil},
		{"record", ts, nil, []any{"time", "REC", "level", "REC", "source", "REC"}},
		{
			"With and record",
			ts,
			[]slog.Attr{slog.String("level", "WITH"), slog.String("source", "WITH")},
			[]any{"level", "REC", "time", "REC"},
		},
	}
	for _, layout := range layouts {
		for _, pc := range []uintptr{pcs[0], 0} {
			for _, addSource := range []bool{false, true} {
				for _, replace := range []bool{false, true} {
					for _, rec := range records {
						name := fmt.Sprintf("%v %v pc=%v source=%v replace=%v %s",
							layout.prefixKeys, layout.suffixKeys, pc != 0, addSource, replace, rec.name)
						tt.Run(name, func(tt *testing.T) {
							tt.Parallel()
							t := check.Must(tt)

							var replaceAttr func([]string, slog.Attr) slog.Attr
							if replace { // Disables the fast path for built-in attrs.
								replaceAttr = func(_ []string, a slog.Attr) slog.Attr { return a }
							}
							var textBuf, layoutBuf bytes.Buffer
							textHandler := slog.NewTextHandler(&textBuf, &slog.HandlerOptions{
								AddSource:   addSource,
								ReplaceAttr: replaceAttr,
							})
							layoutHandler := slogx.NewLayoutHandler(&layoutBuf, &slogx.LayoutHandlerOptions{
								AddSource:   addSource,
								ReplaceAttr: replaceAttr,
								PrefixKeys:  layout.prefixKeys,
								SuffixKeys:  layout.suffixKeys,
							})
							r := slog.NewRecord(rec.time, slog.LevelInfo, "msg", pc)
							r.Add(rec.args...)
							for _, h := range []slog.Handler{textHandler, layoutHandler} {
								t.Nil(h.WithAttrs(rec.with).Handle(tt.Context(), r))
								t.Nil(h.WithGroup("g").WithAttrs(rec.with).Handle(tt.Context(), r))
							}

							want := applyLayout(textBuf.String(), layout.prefixKeys, layout.suffixKeys)
							t.Equal(layoutBuf.String(), want)
						})
					}
				}
			}
		}
	}
}

// applyLayout moves the last attr of each of prefixKeys and suffixKeys
// in output of slog.TextHandler to its layout position and removes other attrs with these keys.
// It expects attr values without spaces.
func applyLayout(output string, prefixKeys, suffixKeys []string) string {
	var want strings.Builder
	for line := range strings.Lines(output) {
		last := make(map[string]string)
		var attrs []string
		for attr := range strings.SplitSeq(strings.TrimSuffix(line, "\n"), " ") {
			key, _, _ := strings.Cut(attr, "=")
			if slices.Contains(prefixKeys, key) || slices.Contains(suffixKeys, key) {
				last[key] = attr
			} else {
				attrs = append(attrs, attr)
			}
		}
		var prefix, suffix []string
		for _, k := range prefixKeys {
			if attr, ok := last[k]; ok {
				prefix = append(prefix, attr)
			}
		}
		for _, k := range suffixKeys {
			if attr, ok := last[k]; ok {
				suffix = append(suffix, attr)
			}
		}
		want.WriteString(strings.Join(slices.Concat(prefix, attrs, suffix), " ") + "\n")
	}
	return want.String()
}

// Format and layout position depend only on the key,
// so they apply the same way to built-in attrs and other attrs with the same key.
// Only the last attr with a layout key is output, but all attrs with other keys are output.
func TestLayoutHandler_BuiltInKeyFormat(tt *testing.T) {
	tt.Parallel()

	tests := []struct {
		name       string
		format     string
		prefixKeys []string
		suffixKeys []string
		with       string
		args       []any
		want       string
	}{
		{"no layout", "[%v]", nil, nil, "WITH", []any{"level", "REC"}, "[INFO] msg=msg[WITH][REC]\n"},
		{"prefix With", "[%v]", []string{slog.LevelKey}, nil, "WITH", nil, "[WITH] msg=msg\n"},
		{"prefix record", "[%v]", []string{slog.LevelKey}, nil, "WITH", []any{"level", "REC"}, "[REC] msg=msg\n"},
		{"suffix With", "[%v]", nil, []string{slog.LevelKey}, "WITH", nil, "msg=msg[WITH]\n"},
		{"suffix record", "[%v]", nil, []string{slog.LevelKey}, "WITH", []any{"level", "REC"}, "msg=msg[REC]\n"},
		{"suffix empty With", "%s", nil, []string{slog.LevelKey}, "", nil, "msg=msg\n"},
		{"suffix empty record", "%s", nil, []string{slog.LevelKey}, "WITH", []any{"level", ""}, "msg=msg\n"},
	}
	for _, tc := range tests {
		tt.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)

			var buf bytes.Buffer
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				Format:     map[string]string{slog.TimeKey: "", slog.LevelKey: tc.format},
				PrefixKeys: tc.prefixKeys,
				SuffixKeys: tc.suffixKeys,
			}))
			logger.With("level", tc.with).Info("msg", tc.args...)
			t.Equal(buf.String(), tc.want)
		})
	}
}

func TestLayoutHandler_BuiltInLayoutKeyRecordTimeFormat(tt *testing.T) {
	tt.Parallel()

	for _, replace := range []bool{false, true} {
		tt.Run(fmt.Sprintf("replace=%v", replace), func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)

			var replaceAttr func([]string, slog.Attr) slog.Attr
			if replace { // Disables the fast path for built-in attrs.
				replaceAttr = func(_ []string, a slog.Attr) slog.Attr { return a }
			}
			var buf bytes.Buffer
			h := slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				ReplaceAttr:      replaceAttr,
				RecordTimeFormat: time.DateOnly,
				SuffixKeys:       []string{slog.TimeKey},
			})
			r := slog.NewRecord(time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC), slog.LevelInfo, "msg", 0)
			t.Nil(h.Handle(tt.Context(), r))
			t.Equal(buf.String(), "level=INFO msg=msg time=2006-01-02\n")
		})
	}
}

// Moving built-in attrs (fast path), With attrs and record attrs to layout positions
// must not allocate.
//
//nolint:paralleltest // testing.AllocsPerRun panics in parallel tests.
func TestLayoutHandler_LayoutAllocs(tt *testing.T) {
	if race.Enabled {
		tt.Skip("sync.Pool drops items randomly under -race")
	}
	t := check.Must(tt)

	h := slogx.NewLayoutHandler(io.Discard, &slogx.LayoutHandlerOptions{
		PrefixKeys: []string{"a", "b"},
		SuffixKeys: []string{slog.LevelKey, slog.TimeKey},
	}).WithAttrs([]slog.Attr{slog.Int("b", 2)})
	ctx := tt.Context()
	r := slog.NewRecord(time.Now(), slog.LevelDebug, "msg", 0)
	r.AddAttrs(slog.Int("a", 1))
	allocs := testing.AllocsPerRun(100, func() {
		_ = h.Handle(ctx, r)
	})
	t.Zero(allocs)
}

func TestLayoutHandler_ConcurrentHandle(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	const goroutines, iterations = 8, 200

	var buf syncBuffer
	logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
		Format:     map[string]string{slog.TimeKey: "[%v]", slog.LevelKey: " %v"},
		PrefixKeys: []string{slog.TimeKey, slog.LevelKey},
	}))

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range iterations {
				logger.Info("msg")
			}
		})
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	t.Len(lines, goroutines*iterations)
	for _, line := range lines {
		t.Match(line, `^\[\S+\] INFO msg=msg$`)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

//nolint:paralleltest // Subtests share buf and logger, must run sequentially.
func TestLayoutHandler_LayoutWith(t *testing.T) {
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
		{
			"parallel With",
			func() {
				logger1 := logger.With("p", "abcde", "s", "ABCDE", "a", "12345")
				logger2 := logger.With("p", "xyz", "s", "XYZ", "a", "789")
				logger1.Info("Test1")
				logger2.Info("Test2")
			},
			`^` +
				`p=abcde msg=Test1 a=12345 s=ABCDE\n` +
				`p=xyz msg=Test2 a=789 s=XYZ$`,
		},
		{
			"tree With",
			func() {
				logger1 := logger.With("p", -1, "s", 1, "a", 10)
				logger2 := logger1.With("p", -2, "s", 2, "a", 20)
				logger3 := logger1.With("p", -3, "s", 3, "a", 30)
				logger1.Info("Test1")
				logger2.Info("Test2")
				logger3.Info("Test3")
			},
			`^` +
				`p=-1 msg=Test1 a=10 s=1\n` +
				`p=-2 msg=Test2 a=10 a=20 s=2\n` +
				`p=-3 msg=Test3 a=10 a=30 s=3$`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			t := check.Must(tt)
			buf.Reset()
			tc.f()
			got := buf.String()
			t.Must(t.NotEqual(got, ""))
			t.Must(t.Equal(got[len(got)-1], byte('\n')))
			t.Match(got[:len(got)-1], tc.want)
		})
	}
}

func TestLayoutHandler_AttrSep(t *testing.T) {
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
					tt.Parallel()
					t := check.Must(tt)
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

func TestLayoutHandler_AttrSepAfterFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format     map[string]string
		prefixKeys []string
		msg        string
		a          string
		want       string
	}{
		{map[string]string{"a": "%s"}, nil, "hi", "", "level=INFO msg=hi b=1\n"},
		{map[string]string{"a": "%v"}, nil, "hi", "", "level=INFO msg=hi\"\" b=1\n"},
		{map[string]string{"a": "%3s"}, nil, "hi", "", "level=INFO msg=hi    b=1\n"},
		{map[string]string{"a": " %s"}, nil, "hi", "", "level=INFO msg=hi  b=1\n"},
		{map[string]string{"a": "%s"}, nil, "hi", "x", "level=INFO msg=hix b=1\n"},
		{map[string]string{"a": "%s", slog.LevelKey: "", slog.MessageKey: ""}, nil, "", "", "b=1\n"},
		// Prefix attrs are inserted before the message, which has empty Format output here.
		{map[string]string{slog.LevelKey: "", slog.MessageKey: "%s"}, []string{"a"}, "", "x", "a=x b=1\n"},
		// Prefix attrs are inserted after built-ins, so they do not start the line.
		{map[string]string{"a": " a=%s"}, []string{"a"}, "hi", "x", "level=INFO a=x msg=hi b=1\n"},
		{map[string]string{slog.LevelKey: "%s"}, []string{"a"}, "hi", "x", "INFO a=x msg=hi b=1\n"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%v %q", tc.format, tc.a), func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			var buf bytes.Buffer
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				ReplaceAttr: removeTime,
				Format:      tc.format,
				PrefixKeys:  tc.prefixKeys,
			}))

			logger.With("a", tc.a, "b", 1).Info(tc.msg)
			t.Equal(buf.String(), tc.want, "With(a, b).Info()")
			buf.Reset()
			logger.Info(tc.msg, "a", tc.a, "b", 1)
			t.Equal(buf.String(), tc.want, "Info(a, b)")
			buf.Reset()
			logger.With("a", tc.a).Info(tc.msg, "b", 1)
			t.Equal(buf.String(), tc.want, "With(a).Info(b)")
		})
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

//nolint:paralleltest // Subtests share buf and must run sequentially.
func TestLayoutHandler_TimeFormat(t *testing.T) {
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
			t := check.Must(tt)
			buf.Reset()
			now, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.789123456+01:00")
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == slog.TimeKey && len(groups) == 0 {
						a.Value = slog.TimeValue(now)
					}
					return a
				},
				RecordTimeFormat: tc.recordTimeFormat,
				TimeFormat:       tc.timeFormat,
				Format: map[string]string{
					slog.LevelKey:   "",
					slog.MessageKey: "",
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
	optsSource := opts
	optsSource.AddSource = true
	for _, handler := range []struct {
		name string
		h    slog.Handler
	}{
		{"all-opts", slogx.NewLayoutHandler(io.Discard, &opts)},
		{"all-opts-source", slogx.NewLayoutHandler(io.Discard, &optsSource)},
		{"no-opts", slogx.NewLayoutHandler(io.Discard, nil)},
		{"no-opts-source", slogx.NewLayoutHandler(io.Discard, &slogx.LayoutHandlerOptions{AddSource: true})},
		{"std-text", slog.NewTextHandler(io.Discard, nil)},
		{"std-text-source", slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: true})},
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
