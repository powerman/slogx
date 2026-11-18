package slogx

import (
	"iter"
	"log/slog"
	"slices"

	"github.com/powerman/slogx/internal"
)

// GroupOrAttrs holds a sequence of WithGroup and WithAttrs calls.
//
// It is a useful helper for implementing [slog.Handler].
//
// Zero value and nil are valid and represent no groups or attrs.
type GroupOrAttrs struct {
	group     string      // Group name if non-empty.
	attrs     []slog.Attr // Attrs if non-empty.
	total     int         // Total number of attrs and groups in this and all prev.
	hasGroups bool        // Whether this or any prev is a group.
	prev      *GroupOrAttrs
}

// Total returns the total number of attrs and groups added to g.
func (g *GroupOrAttrs) Total() int {
	if g == nil {
		return 0
	}
	return g.total
}

// WithAttrs returns a GroupOrAttrs that includes the given attrs.
// If there are no attrs or all attrs are empty groups, g is returned unchanged.
func (g *GroupOrAttrs) WithAttrs(attrs []slog.Attr) *GroupOrAttrs {
	if internal.CountEmptyGroups(attrs) == len(attrs) {
		return g
	}
	if g.Total() == 0 {
		g = nil
	}
	return &GroupOrAttrs{
		attrs:     attrs,
		total:     g.Total() + len(attrs),
		hasGroups: g != nil && g.hasGroups,
		prev:      g,
	}
}

// WithGroup returns a GroupOrAttrs that includes the given group.
// If name is empty, g is returned unchanged.
func (g *GroupOrAttrs) WithGroup(name string) *GroupOrAttrs {
	if name == "" {
		return g
	}
	if g.Total() == 0 {
		g = nil
	}
	return &GroupOrAttrs{
		group:     name,
		total:     g.Total() + 1,
		hasGroups: true,
		prev:      g,
	}
}

// Record returns a record that includes all groups and attrs in g and r.
func (g *GroupOrAttrs) Record(r slog.Record) slog.Record {
	if g.Total() == 0 {
		return r
	}

	if !g.hasGroups { // Optimize for the common case of no groups.
		return g.recordWithoutGroups(r)
	}

	attrs := make([]slog.Attr, 0, r.NumAttrs()+g.Total())
	r.Attrs(func(a slog.Attr) bool { attrs = append(attrs, a); return true })
	for ; g != nil; g = g.prev {
		if g.group == "" {
			// Prepend attrs in reverse order to preserve their original order.
			attrs = attrs[:len(attrs)+len(g.attrs)]
			copy(attrs[len(g.attrs):], attrs)
			copy(attrs, g.attrs)
		} else {
			groupAttrs := attrs
			attrs = attrs[len(attrs) : len(attrs)+1]
			attrs[0] = slog.Attr{Key: g.group, Value: slog.GroupValue(groupAttrs...)}
		}
	}

	r2 := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r2.AddAttrs(attrs...)
	return r2
}

// recordWithoutGroups is Record for g which has no groups.
//
// While the attrs fit into the stack buffer,
// the only allocation is the overflow slice of the record (needed for more than 5 attrs).
func (g *GroupOrAttrs) recordWithoutGroups(r slog.Record) slog.Record {
	const stackAttrs = 16 // The number of attrs collected without allocating.
	var buf [stackAttrs]slog.Attr

	attrs := buf[:0]
	n := g.total + r.NumAttrs()
	if n > len(buf) {
		attrs = make([]slog.Attr, 0, n)
	}
	attrs = attrs[:g.total]

	// The chain is walked from the newest to the oldest, so fill attrs from the end.
	i := len(attrs)
	for cur := g; cur != nil; cur = cur.prev {
		i -= len(cur.attrs)
		copy(attrs[i:], cur.attrs)
	}
	r.Attrs(func(a slog.Attr) bool { attrs = append(attrs, a); return true })

	r2 := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r2.AddAttrs(attrs...)
	return r2
}

// All yields all groups and attrs in the order they were added.
// It yields either an attr with an empty group name, or a group name with an empty attr.
// If there are no groups or attrs, All yields nothing.
func (g *GroupOrAttrs) All() iter.Seq2[string, slog.Attr] {
	return func(yield func(string, slog.Attr) bool) {
		// Collect groups and attrs in reverse order.
		// We will yield them in reverse order again to restore the original order.
		for _, v := range slices.Backward(g.reverse()) {
			if v.group != "" {
				if !yield(v.group, slog.Attr{}) {
					return
				}
			} else {
				for _, a := range v.attrs {
					if !yield("", a) {
						return
					}
				}
			}
		}
	}
}

// Handler returns next with all groups and attrs of g applied in the order they were added.
// It does not modify g.
func (g *GroupOrAttrs) Handler(next slog.Handler) slog.Handler {
	for _, v := range slices.Backward(g.reverse()) {
		if v.group != "" {
			next = next.WithGroup(v.group)
		} else {
			next = next.WithAttrs(slices.Clone(v.attrs))
		}
	}
	return next
}

// Reverse returns a slice of GroupOrAttrs in reverse order.
// The slice contains either a group with an empty attrs, or attrs with an empty group.
// If there are no groups or attrs, reverse returns nil.
// Returned values are copies of g's values with prev set to nil,
// total set to 0 and hasGroups set to false.
func (g *GroupOrAttrs) reverse() []GroupOrAttrs {
	if g.Total() == 0 {
		return nil
	}
	reverse := make([]GroupOrAttrs, 0, g.total)
	for cur := g; cur != nil; cur = cur.prev {
		if cur.group != "" {
			reverse = append(reverse, GroupOrAttrs{group: cur.group})
		} else {
			reverse = append(reverse, GroupOrAttrs{attrs: cur.attrs})
		}
	}
	return reverse
}
