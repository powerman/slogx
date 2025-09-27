// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.
// From version 1.25.1.

package internal

func ArgsToAttrSlice(args []any) []Attr {
	var (
		attr  Attr
		attrs []Attr
	)
	for len(args) > 0 {
		attr, args = argsToAttr(args)
		attrs = append(attrs, attr)
	}
	return attrs
}
