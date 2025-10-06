// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.

package internal

import (
	"runtime"
	"testing"

	"github.com/powerman/slogx/internal/race"
)

// callerPC returns the program counter at the given stack depth.
func callerPC(depth int) uintptr {
	var pcs [1]uintptr
	runtime.Callers(depth, pcs[:])
	return pcs[0]
}

func wantAllocs(t *testing.T, want int, f func()) {
	if race.Enabled {
		t.Skip("skipping test in race, asan, and msan modes")
	}
	t.Helper()
	got := int(testing.AllocsPerRun(5, f))
	if got != want {
		t.Errorf("got %d allocs, want %d", got, want)
	}
}
