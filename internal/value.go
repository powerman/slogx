// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.

package internal

import (
	"fmt"
	"strconv"
)

// CountEmptyGroups returns the number of empty group values in its argument.
func CountEmptyGroups(as []Attr) int {
	n := 0
	for _, a := range as {
		if isEmptyGroupValue(a.Value) {
			n++
		}
	}
	return n
}

// isEmptyGroupValue reports whether v is a group that has no attributes.
func isEmptyGroupValue(v Value) bool {
	if v.Kind() != KindGroup {
		return false
	}
	// We do not need to recursively examine the group's Attrs for emptiness,
	// because GroupValue removed them when the group was constructed, and
	// groups are immutable.
	return len(v.Group()) == 0
}

// appendValue appends a text representation of v to dst.
// v is formatted as with fmt.Sprint.
func appendValue(v Value, dst []byte) []byte {
	switch v.Kind() {
	case KindString:
		return append(dst, v.String()...)
	case KindInt64:
		return strconv.AppendInt(dst, v.Int64(), 10)
	case KindUint64:
		return strconv.AppendUint(dst, v.Uint64(), 10)
	case KindFloat64:
		return strconv.AppendFloat(dst, v.Float64(), 'g', -1, 64)
	case KindBool:
		return strconv.AppendBool(dst, v.Bool())
	case KindDuration:
		return append(dst, v.Duration().String()...)
	case KindTime:
		return append(dst, v.Time().String()...)
	case KindGroup:
		return fmt.Append(dst, v.Group())
	case KindAny, KindLogValuer:
		return fmt.Append(dst, v.Any())
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
}
