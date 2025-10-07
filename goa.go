package slogx

import (
	"iter"
	"log/slog"

	"github.com/powerman/slogx/internal"
)

// GroupOrAttrs holds a sequence of WithGroup and WithAttrs calls.
//
// It is a useful helper for implementing [slog.Handler].
//
// Zero value and nil are valid and represent no groups or attrs.
type GroupOrAttrs struct {
	group string      // Group name if non-empty.
	attrs []slog.Attr // Attrs if non-empty.
	total int         // Total number of attrs and groups in this and all prev.
	prev  *GroupOrAttrs
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
func (g *GroupOrAttrs) WithAttrs(as []slog.Attr) *GroupOrAttrs {
	if internal.CountEmptyGroups(as) == len(as) {
		return g
	}
	if g.Total() == 0 {
		g = nil
	}
	return &GroupOrAttrs{
		attrs: as,
		total: g.Total() + len(as),
		prev:  g,
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
		group: name,
		total: g.Total() + 1,
		prev:  g,
	}
}

// Record returns a record that includes all groups and attrs in g and r.
func (g *GroupOrAttrs) Record(r slog.Record) slog.Record {
	if g.Total() == 0 {
		return r
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

// All yields all groups and attrs in the order they were added.
// It yields either an attr with an empty group name, or a group name with an empty attr.
// If there are no groups or attrs, All yields nothing.
func (g *GroupOrAttrs) All() iter.Seq2[string, slog.Attr] {
	return func(yield func(string, slog.Attr) bool) {
		// Collect groups and attrs in reverse order.
		// We will yield them in reverse order again to restore the original order.
		reverse := g.reverse()
		for i := len(reverse) - 1; i >= 0; i-- {
			cur := reverse[i]
			if cur.group != "" {
				if !yield(cur.group, slog.Attr{}) {
					return
				}
			} else {
				for _, a := range cur.attrs {
					if !yield("", a) {
						return
					}
				}
			}
		}
	}
}

// Reverse returns a slice of GroupOrAttrs in reverse order.
// The slice contains either a group with an empty attrs, or attrs with an empty group.
// If there are no groups or attrs, reverse returns nil.
// Returned values are copies of g's values with prev set to nil and total set to 0.
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
