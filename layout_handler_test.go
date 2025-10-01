package slogx_test

import (
	"bytes"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

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
		// Only allowed verbs is zero or one %s.
		{
			"unknown verb",
			F{"bad": "%v"},
		},
		{
			"multiple verbs",
			F{"bad": "%s%s"},
		},
		// Only allowed flags is - (left align).
		{
			"unknown flag +",
			F{"bad": "%+s"},
		},
		{
			"unknown flag #",
			F{"bad": "%#s"},
		},
		{
			"unknown flag space",
			F{"bad": "% s"},
		},
		{
			"multiple flags",
			F{"bad": "%--s"},
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
			"MaxWidth=-1",
			F{"bad": "%.-1s"},
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
			F{"a": "%s", "bad": "%", "c": "%s"},
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
		{"%s", slog.IntValue(5), `^5$`},
		{"%s", slog.StringValue(" "), `^" "$`},
		{"%s", slog.StringValue(""), `^""$`},
		{"%s", slog.AnyValue(""), `^""$`},
		{"%s", slog.AnyValue([]byte{}), `^""$`},
		{"%s", slog.AnyValue([]byte(nil)), `^""$`},
		{"%s", slog.AnyValue(nil), `^<nil>$`},
		{"prefix%s", slog.IntValue(5), `^prefix5$`},
		{"%ssuffix", slog.IntValue(5), `^5suffix$`},
		{"prefix%ssuffix", slog.IntValue(5), `^prefix5suffix$`},
		{"%%", slog.IntValue(5), `^%$`},
		{"%%%%%%", slog.IntValue(5), `^%%%$`},
		{"%%s", slog.IntValue(5), `^%s$`},
		{"%%%s%%", slog.IntValue(5), `^%5%$`},
		{"prefix%%suffix", slog.IntValue(5), `^prefix%suffix$`},
		{"prefix%ssuffix", slog.IntValue(5), `^prefix5suffix$`},
		{"%0s", slog.IntValue(5), `^5$`},
		{"%-0s", slog.IntValue(5), `^5$`},
		{"%1s", slog.IntValue(5), `^5$`},
		{"%-1s", slog.IntValue(5), `^5$`},
		{"%2s", slog.IntValue(5), `^ 5$`},
		{"%-2s", slog.IntValue(5), `^5 $`},
		{"%3s", slog.IntValue(5), `^  5$`},
		{"%-3s", slog.IntValue(5), `^5  $`},
		{"%03s", slog.IntValue(5), `^  5$`},
		{"%-03s", slog.IntValue(5), `^5  $`},
		{"%.0s", slog.IntValue(5), `^$`},
		{"%-.0s", slog.IntValue(5), `^$`},
		{"%.1s", slog.IntValue(5), `^5$`},
		{"%-.1s", slog.IntValue(5), `^5$`},
		{"%.01s", slog.IntValue(5), `^5$`},
		{"%-.01s", slog.IntValue(5), `^5$`},
		{"%3.1s", slog.IntValue(5), `^  5$`},
		{"%-3.1s", slog.IntValue(5), `^5  $`},
		{"%3.5s", slog.IntValue(5), `^  5$`},
		{"%-3.5s", slog.IntValue(5), `^5  $`},
		{"%.1s", slog.StringValue("abcde"), `^…$`},
		{"%.2s", slog.StringValue("abcde"), `^a…$`},
		{"%.3s", slog.StringValue("abcde"), `^ab…$`},
		{"%.4s", slog.StringValue("abcde"), `^abc…$`},
		{"%.5s", slog.StringValue("abcde"), `^abcde$`},
		{"%.6s", slog.StringValue("abcde"), `^abcde$`},
		{"%.1s quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%.2s quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%.3s quoted", slog.StringValue("ab=de"), `^"…" quoted$`},
		{"%.4s quoted", slog.StringValue("ab=de"), `^"a…" quoted$`},
		{"%.5s quoted", slog.StringValue("ab=de"), `^"ab…" quoted$`},
		{"%.6s quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%.7s quoted", slog.StringValue("ab=de"), `^"ab=de" quoted$`},
		{"%.8s quoted", slog.StringValue("ab=de"), `^"ab=de" quoted$`},
		{"%1.1s", slog.StringValue("abcde"), `^…$`},
		{"%-1.1s", slog.StringValue("abcde"), `^…$`},
		{"%2.1s", slog.StringValue("abcde"), `^ …$`},
		{"%-2.1s", slog.StringValue("abcde"), `^… $`},
		{"%3.1s", slog.StringValue("abcde"), `^  …$`},
		{"%-3.1s", slog.StringValue("abcde"), `^…  $`},
		{"%1.2s", slog.StringValue("abcde"), `^a…$`},
		{"%-1.2s", slog.StringValue("abcde"), `^a…$`},
		{"%2.2s", slog.StringValue("abcde"), `^a…$`},
		{"%-2.2s", slog.StringValue("abcde"), `^a…$`},
		{"%3.2s", slog.StringValue("abcde"), `^ a…$`},
		{"%-3.2s", slog.StringValue("abcde"), `^a… $`},
		{"%4.2s", slog.StringValue("abcde"), `^  a…$`},
		{"%-4.2s", slog.StringValue("abcde"), `^a…  $`},
		{"%4.5s", slog.StringValue("abcde"), `^abcde$`},
		{"%-4.5s", slog.StringValue("abcde"), `^abcde$`},
		{"%5.5s", slog.StringValue("abcde"), `^abcde$`},
		{"%-5.5s", slog.StringValue("abcde"), `^abcde$`},
		{"%6.5s", slog.StringValue("abcde"), `^ abcde$`},
		{"%-6.5s", slog.StringValue("abcde"), `^abcde $`},
		{"%4.6s", slog.StringValue("abcde"), `^abcde$`},
		{"%-4.6s", slog.StringValue("abcde"), `^abcde$`},
		{"%5.6s", slog.StringValue("abcde"), `^abcde$`},
		{"%-5.6s", slog.StringValue("abcde"), `^abcde$`},
		{"%6.6s", slog.StringValue("abcde"), `^ abcde$`},
		{"%-6.6s", slog.StringValue("abcde"), `^abcde $`},
		{"%1.1s quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%-1.1s quoted", slog.StringValue("ab=de"), `^… quoted$`},
		{"%2.1s quoted", slog.StringValue("ab=de"), `^ … quoted$`},
		{"%-2.1s quoted", slog.StringValue("ab=de"), `^…  quoted$`},
		{"%3.1s quoted", slog.StringValue("ab=de"), `^  … quoted$`},
		{"%-3.1s quoted", slog.StringValue("ab=de"), `^…   quoted$`},
		{"%1.2s quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%-1.2s quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%2.2s quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%-2.2s quoted", slog.StringValue("ab=de"), `^…… quoted$`},
		{"%3.2s quoted", slog.StringValue("ab=de"), `^ …… quoted$`},
		{"%-3.2s quoted", slog.StringValue("ab=de"), `^……  quoted$`},
		{"%5.6s quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%-5.6s quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%6.6s quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%-6.6s quoted", slog.StringValue("ab=de"), `^"ab=…" quoted$`},
		{"%7.6s quoted", slog.StringValue("ab=de"), `^ "ab=…" quoted$`},
		{"%-7.6s quoted", slog.StringValue("ab=de"), `^"ab=…"  quoted$`},
		{"%.1s utf8", slog.StringValue("😄世ЮЯ界😊"), `^… utf8$`},
		{"%.2s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄… utf8$`},
		{"%.3s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%.4s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世Ю… utf8$`},
		{"%.5s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世ЮЯ… utf8$`},
		{"%.6s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世ЮЯ界😊 utf8$`},
		{"%.7s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世ЮЯ界😊 utf8$`},
		{"%2.3s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%-2.3s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%3.3s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%-3.3s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世… utf8$`},
		{"%4.3s utf8", slog.StringValue("😄世ЮЯ界😊"), `^ 😄世… utf8$`},
		{"%-4.3s utf8", slog.StringValue("😄世ЮЯ界😊"), `^😄世…  utf8$`},
		{"%.1s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^… utf8 quoted$`},
		{"%.2s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^…… utf8 quoted$`},
		{"%.3s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"…" utf8 quoted$`},
		{"%.4s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄…" utf8 quoted$`},
		{"%.5s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%.6s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю…" utf8 quoted$`},
		{"%.7s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=…" utf8 quoted$`},
		{"%.8s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=Я…" utf8 quoted$`},
		{"%.9s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=Я界😊" utf8 quoted$`},
		{"%.10s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世Ю=Я界😊" utf8 quoted$`},
		{"%4.5s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%-4.5s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%5.5s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%-5.5s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…" utf8 quoted$`},
		{"%6.5s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^ "😄世…" utf8 quoted$`},
		{"%-6.5s utf8 quoted", slog.StringValue("😄世Ю=Я界😊"), `^"😄世…"  utf8 quoted$`},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(tt *testing.T) {
			t := check.T(tt)
			buf.Reset()
			logger := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				Format: map[string]string{
					slog.TimeKey:    "",
					slog.LevelKey:   "",
					slog.MessageKey: "",
					"value":         tc.format,
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
			F{"time": "", "level": "%s", "msg": "%s", "a": "%s", "b": "%s", "c": "%s", "d": "%s", "e": "%s"},
			nil,
			nil,
			`^INFOtest12345$`,
		},
		{
			"format everything reordered",
			F{"time": "", "level": "%s", "msg": "%s", "a": "%s", "b": "%s", "c": "%s", "d": "%s", "e": "%s"},
			L{"e", "d"},
			L{"b", "a"},
			`^INFO54test321$`,
		},
		{
			"format everything no std",
			F{"time": "", "level": "", "msg": "", "a": "%s", "b": "%s", "c": "%s", "d": "%s", "e": "%s"},
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
		// Prefix before message at start of line.
		{
			"prefix excluded",
			F{"time": "", "level": "", "a": "", "b": ""},
			L{"a", "b"},
			nil,
			`^msg=test c=3 d=4 e=5$`,
		},
		{
			"prefix no format",
			F{"time": "", "level": ""},
			L{"a"},
			nil,
			`^a=1 msg=test b=2 c=3 d=4 e=5$`,
		},
		{
			"prefix format",
			F{"time": "", "level": "", "a": "%s"},
			L{"a"},
			nil,
			`^1 msg=test b=2 c=3 d=4 e=5$`,
		},
		// Prefix before missing message.
		{
			"prefix excluded no msg",
			F{"time": "", "level": "", "msg": "", "a": "", "b": ""},
			L{"a", "b"},
			nil,
			`^c=3 d=4 e=5$`,
		},
		{
			"prefix no format no msg",
			F{"time": "", "level": "", "msg": ""},
			L{"a"},
			nil,
			`^a=1 b=2 c=3 d=4 e=5$`,
		},
		{
			"prefix format no msg",
			F{"time": "", "level": "", "msg": "", "a": "%s"},
			L{"a"},
			nil,
			`^1 b=2 c=3 d=4 e=5$`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			t := check.T(tt)

			logger1 := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				Format:     tc.format,
				PrefixKeys: tc.prefixKeys,
				SuffixKeys: tc.suffixKeys,
			}))

			excludedKeys := []string{}
			format := make(F)
			for k, v := range tc.format {
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
			logger2 := slog.New(slogx.NewLayoutHandler(&buf, &slogx.LayoutHandlerOptions{
				ReplaceAttr: replaceAttr,
				Format:      format,
				PrefixKeys:  tc.prefixKeys,
				SuffixKeys:  tc.suffixKeys,
			}))

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
