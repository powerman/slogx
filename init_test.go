package slogx_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/powerman/check"
)

func removeTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.Attr{}
	}
	return a
}

func makeTextResults(t *check.C, buf *bytes.Buffer) func() []map[string]any {
	t.Helper()
	ident := `("[^"]*"|[^"]\S*)`
	attr := fmt.Sprintf(`%s=%s`, ident, ident)
	attrRe := regexp.MustCompile(`^` + attr + `(?: |$)`)
	return func() []map[string]any {
		var ms []map[string]any
		for line := range strings.SplitSeq(buf.String(), "\n") {
			if line == "" {
				continue
			}
			m := make(map[string]any)
			for line != "" {
				match := attrRe.FindStringSubmatch(line)
				t.Must(t.Len(match, 3))
				line = line[len(match[0]):]
				for i := range 2 {
					if match[i+1][0] == '"' {
						var err error
						match[i+1], err = strconv.Unquote(match[i+1])
						t.Nil(err)
					}
				}
				keyElems := strings.Split(match[1], ".")
				key := keyElems[len(keyElems)-1]
				m2 := m
				for _, g := range keyElems[:len(keyElems)-1] {
					if _, ok := m2[g]; !ok {
						m2[g] = make(map[string]any)
					}
					m2 = m2[g].(map[string]any)
				}
				m2[key] = match[2]
			}
			ms = append(ms, m)
		}
		return ms
	}
}
