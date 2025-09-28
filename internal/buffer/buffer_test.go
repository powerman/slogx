// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buffer

import (
	"testing"

	"github.com/powerman/slogx/internal/race"
)

func Test(t *testing.T) {
	b := New()
	defer b.Free()
	b.WriteString("hello")
	b.WriteByte(',')
	b.Write([]byte(" world"))

	got := b.String()
	want := "hello, world"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAlloc(t *testing.T) {
	if race.Enabled {
		t.Skip("skipping test in race mode")
	}
	got := int(testing.AllocsPerRun(5, func() {
		b := New()
		defer b.Free()
		b.WriteString("not 1K worth of bytes")
	}))
	if got != 0 {
		t.Errorf("got %d allocs, want 0", got)
	}
}

func TestSetLen(t *testing.T) {
	b := New()
	b.WriteString("hello, world")
	b.SetLen(5)

	got := b.String()
	want := "hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	*b = (*b)[:0:3]
	b.SetLen(5)

	got = b.String()
	want = "hel\000\000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
