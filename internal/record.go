// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.

package internal

import "runtime"

const badKey = "!BADKEY"

// argsToAttr turns a prefix of the nonempty args slice into an Attr
// and returns the unconsumed portion of the slice.
// If args[0] is an Attr, it returns it.
// If args[0] is a string, it treats the first two elements as
// a key-value pair.
// Otherwise, it treats args[0] as a value with a missing key.
func argsToAttr(args []any) (Attr, []any) {
	switch x := args[0].(type) {
	case string:
		if len(args) == 1 {
			return String(badKey, x), nil
		}
		return Any(x, args[1]), args[2:]

	case Attr:
		return x, args[1:]

	default:
		return Any(badKey, x), args[1:]
	}
}

// sourceIsEmpty returns whether the Source struct is nil or only contains zero fields.
//
// Same as (*Source).isEmpty.
func sourceIsEmpty(s *Source) bool { return s == nil || *s == Source{} }

// Source returns a new Source for the log event using r's PC.
// If the PC field is zero, meaning the Record was created without the necessary information
// or the location is unavailable, then nil is returned.
//
// Same as Record.Source (added in Go 1.25 and copied here for compatibility with 1.24.6).
func recordSource(r Record) *Source {
	if r.PC == 0 {
		return nil
	}

	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	return &Source{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}
}
