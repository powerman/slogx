package slogx_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/powerman/check"
	"go.uber.org/mock/gomock"

	"github.com/powerman/slogx"
	"github.com/powerman/slogx/internal/race"
)

func TestGroupOrAttrs_Total(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goa  *slogx.GroupOrAttrs
		want int
	}{
		{
			"nil",
			nil,
			0,
		},
		{
			"zero value",
			&slogx.GroupOrAttrs{},
			0,
		},
		{
			"single attr",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			1,
		},
		{
			"multiple attrs",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1), slog.String("b", "2")}),
			2,
		},
		{
			"single group",
			new(slogx.GroupOrAttrs).WithGroup("g1"),
			1,
		},
		{
			"multiple groups",
			new(slogx.GroupOrAttrs).WithGroup("g1").WithGroup("g2"),
			2,
		},
		{
			"mixed",
			new(slogx.GroupOrAttrs).
				WithAttrs([]slog.Attr{slog.Int("a", 1)}).
				WithGroup("g1").
				WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Int("c", 3)}).
				WithGroup("g2"),
			5,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			t.Equal(tc.goa.Total(), tc.want)
		})
	}
}

func TestGroupOrAttrs_WithAttrs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goa   *slogx.GroupOrAttrs
		attrs []slog.Attr
		want  int
	}{
		{
			"nil goa, empty attrs",
			nil,
			[]slog.Attr{},
			0,
		},
		{
			"nil goa, single attr",
			nil,
			[]slog.Attr{slog.Int("a", 1)},
			1,
		},
		{
			"zero goa, single attr",
			&slogx.GroupOrAttrs{},
			[]slog.Attr{slog.Int("a", 1)},
			1,
		},
		{
			"existing goa, add attrs",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			[]slog.Attr{slog.String("b", "2")},
			2,
		},
		{
			"empty attrs",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			[]slog.Attr{},
			1,
		},
		{
			"only empty groups",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			[]slog.Attr{slog.Group("empty")},
			1,
		},
		{
			"group with attrs",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			[]slog.Attr{slog.Group("g", "k", "v")},
			2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			result := tc.goa.WithAttrs(tc.attrs)
			t.Equal(result.Total(), tc.want)
		})
	}
}

func TestGroupOrAttrs_WithGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goa   *slogx.GroupOrAttrs
		group string
		want  int
	}{
		{
			"nil goa, empty name",
			nil,
			"",
			0,
		},
		{
			"nil goa, named group",
			nil,
			"g1",
			1,
		},
		{
			"zero goa, named group",
			&slogx.GroupOrAttrs{},
			"g1",
			1,
		},
		{
			"existing goa, add group",
			new(slogx.GroupOrAttrs).WithGroup("g1"),
			"g2",
			2,
		},
		{
			"empty name returns same",
			new(slogx.GroupOrAttrs).WithGroup("g1"),
			"",
			1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			result := tc.goa.WithGroup(tc.group)
			t.Equal(result.Total(), tc.want)
		})
	}
}

func TestGroupOrAttrs_Record(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goa  *slogx.GroupOrAttrs
		r    slog.Record
		want []slog.Attr
	}{
		{
			"nil goa",
			nil,
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Int("a", 1)},
		},
		{
			"zero goa",
			&slogx.GroupOrAttrs{},
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Int("a", 1)},
		},
		{
			"single attr",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("b", 2)}),
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Int("b", 2), slog.Int("a", 1)},
		},
		{
			"multiple attrs",
			new(slogx.GroupOrAttrs).
				WithAttrs([]slog.Attr{slog.Int("b", 2)}).
				WithAttrs([]slog.Attr{slog.Int("c", 3)}),
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Int("b", 2), slog.Int("c", 3), slog.Int("a", 1)},
		},
		{
			"empty groups are dropped",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Group("e")}),
			makeRecord("msg", slog.Group("e"), slog.Int("a", 1)),
			[]slog.Attr{slog.Int("b", 2), slog.Int("a", 1)},
		},
		{
			"single group",
			new(slogx.GroupOrAttrs).WithGroup("g1"),
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Group("g1", slog.Int("a", 1))},
		},
		{
			"nested groups",
			new(slogx.GroupOrAttrs).WithGroup("g1").WithGroup("g2"),
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Group("g1", slog.Group("g2", slog.Int("a", 1)))},
		},
		{
			"group with attrs before",
			new(slogx.GroupOrAttrs).
				WithAttrs([]slog.Attr{slog.Int("b", 2)}).
				WithGroup("g1"),
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Int("b", 2), slog.Group("g1", slog.Int("a", 1))},
		},
		{
			"group with attrs after",
			new(slogx.GroupOrAttrs).
				WithGroup("g1").
				WithAttrs([]slog.Attr{slog.Int("b", 2)}),
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{slog.Group("g1", slog.Int("b", 2), slog.Int("a", 1))},
		},
		{
			"complex hierarchy",
			new(slogx.GroupOrAttrs).
				WithAttrs([]slog.Attr{slog.Int("b", 2)}).
				WithGroup("g1").
				WithAttrs([]slog.Attr{slog.Int("c", 3)}).
				WithGroup("g2"),
			makeRecord("msg", slog.Int("a", 1)),
			[]slog.Attr{
				slog.Int("b", 2),
				slog.Group("g1",
					slog.Int("c", 3),
					slog.Group("g2", slog.Int("a", 1)),
				),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			result := tc.goa.Record(tc.r)
			got := collectAttrs(result)
			t.DeepEqual(got, tc.want)
		})
	}
}

// TestGroupOrAttrs_Record_ManyAttrs covers all combinations around the number of attrs
// which slog.Record keeps inline (5), for a goa without groups.
func TestGroupOrAttrs_Record_ManyAttrs(t *testing.T) {
	t.Parallel()

	sum := func(xs []int) (total int) {
		for _, x := range xs {
			total += x
		}
		return total
	}

	for _, chain := range [][]int{{3}, {1, 1, 1}, {7}, {2, 2, 2}, {1, 5, 1, 4}, slices.Repeat([]int{1}, 40)} {
		for _, recAttrs := range []int{0, 1, 2, 3, 5, 6, 9} {
			name := fmt.Sprintf("%d calls/%d attrs/r%d", len(chain), sum(chain), recAttrs)
			t.Run(name, func(tt *testing.T) {
				tt.Parallel()
				t := check.Must(tt)

				var goa *slogx.GroupOrAttrs
				var want []slog.Attr
				for i, n := range chain {
					attrs := make([]slog.Attr, n)
					for j := range attrs {
						attrs[j] = slog.Int(fmt.Sprintf("g%d_%d", i, j), i*100+j)
					}
					goa = goa.WithAttrs(slices.Clone(attrs))
					want = append(want, attrs...)
				}
				recordAttrs := make([]slog.Attr, 0, recAttrs)
				for i := range recAttrs {
					recordAttrs = append(recordAttrs, slog.Int(fmt.Sprintf("r%d", i), i))
				}
				want = append(want, recordAttrs...)

				r := makeRecord("msg", recordAttrs...)
				result := goa.Record(r)
				t.DeepEqual(collectAttrs(result), want)
				t.Equal(result.NumAttrs(), len(want))
				// The original is not modified.
				t.True(slices.EqualFunc(collectAttrs(r), recordAttrs, slog.Attr.Equal))
			})
		}
	}
}

// TestGroupOrAttrs_Record_Allocs is not parallel:
// AllocsPerRun counts allocations of the whole process.
func TestGroupOrAttrs_Record_Allocs(tt *testing.T) { //nolint:paralleltest // See above.
	if race.Enabled {
		tt.Skip("skipping test in race mode")
	}
	t := check.Must(tt)

	// While all attrs fit into slog.Record's inline storage, Record must not allocate.
	// This also guards the assumption about that size (nAttrsInline in log/slog).
	attrs := func(n int) []slog.Attr {
		as := make([]slog.Attr, n)
		for i := range as {
			as[i] = slog.Int("k", i)
		}
		return as
	}
	for _, tc := range []struct {
		goa    []int
		rec    int
		allocs float64
	}{
		{[]int{2}, 0, 0},
		{[]int{2}, 1, 0},
		{[]int{2, 2}, 1, 0},
		{[]int{1, 1, 1, 1}, 1, 0},
		{[]int{3}, 2, 0},
		{[]int{5}, 0, 0},
		{[]int{1}, 4, 0},
		// Above the inline storage: only the overflow slice of the record while it fits the stack buffer.
		{[]int{2}, 5, 1},
		{[]int{1, 1, 1, 1, 1, 1}, 1, 1},
		{[]int{8, 4}, 4, 1},
	} {
		var goa *slogx.GroupOrAttrs
		for _, n := range tc.goa {
			goa = goa.WithAttrs(attrs(n))
		}
		r := makeRecord("msg", attrs(tc.rec)...)
		allocs := testing.AllocsPerRun(100, func() { sinkRecord = goa.Record(r) })
		t.Equal(allocs, tc.allocs, "goa %v rec %d", tc.goa, tc.rec)
	}
}

// TestGroupOrAttrs_Record_EmptyGroups checks that empty groups, which are counted by Total
// but dropped by slog.Record, don't break any of the paths of Record without groups.
func TestGroupOrAttrs_Record_EmptyGroups(t *testing.T) {
	t.Parallel()

	for _, calls := range []int{1, 2, 3, 6, 12, 20} {
		t.Run(fmt.Sprintf("%d calls", calls), func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)

			var goa *slogx.GroupOrAttrs
			want := make([]slog.Attr, 0, calls+1)
			for i := range calls {
				a := slog.Int(fmt.Sprintf("g%d", i), i)
				goa = goa.WithAttrs([]slog.Attr{slog.Group("e"), a, slog.Group("e")})
				want = append(want, a)
			}
			recordAttrs := []slog.Attr{slog.Group("e"), slog.Int("r", 1), slog.Group("e")}
			want = append(want, slog.Int("r", 1))

			t.DeepEqual(collectAttrs(goa.Record(makeRecord("msg", recordAttrs...))), want)
		})
	}
}

func TestGroupOrAttrs_All(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		goa       *slogx.GroupOrAttrs
		wantKeys  []string
		wantAttrs []slog.Attr
	}{
		{
			"nil goa",
			nil,
			nil,
			nil,
		},
		{
			"zero goa",
			&slogx.GroupOrAttrs{},
			nil,
			nil,
		},
		{
			"single attr",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			[]string{""},
			[]slog.Attr{slog.Int("a", 1)},
		},
		{
			"multiple attrs",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1), slog.String("b", "2")}),
			[]string{"", ""},
			[]slog.Attr{slog.Int("a", 1), slog.String("b", "2")},
		},
		{
			"single group",
			new(slogx.GroupOrAttrs).WithGroup("g1"),
			[]string{"g1"},
			[]slog.Attr{{}},
		},
		{
			"multiple groups",
			new(slogx.GroupOrAttrs).WithGroup("g1").WithGroup("g2"),
			[]string{"g1", "g2"},
			[]slog.Attr{{}, {}},
		},
		{
			"mixed",
			new(slogx.GroupOrAttrs).
				WithAttrs([]slog.Attr{slog.Int("a", 1)}).
				WithGroup("g1").
				WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Int("c", 3)}).
				WithGroup("g2"),
			[]string{"", "g1", "", "", "g2"},
			[]slog.Attr{slog.Int("a", 1), {}, slog.Int("b", 2), slog.Int("c", 3), {}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			var gotKeys []string
			var gotAttrs []slog.Attr
			for key, attr := range tc.goa.All() {
				gotKeys = append(gotKeys, key)
				gotAttrs = append(gotAttrs, attr)
			}
			t.DeepEqual(gotKeys, tc.wantKeys)
			t.DeepEqual(gotAttrs, tc.wantAttrs)
		})
	}
}

func TestGroupOrAttrs_All_EarlyStop(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	goa := new(slogx.GroupOrAttrs).
		WithAttrs([]slog.Attr{slog.Int("a", 1)}).
		WithGroup("g1").
		WithAttrs([]slog.Attr{slog.Int("b", 2)})

	var count int
	for range goa.All() {
		count++
		if count == 2 {
			break
		}
	}
	t.Equal(count, 2)
}

func TestGroupOrAttrs_Immutability(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	goa1 := new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)})
	goa2 := goa1.WithGroup("g1")
	goa3 := goa1.WithAttrs([]slog.Attr{slog.Int("b", 2)})

	t.Equal(goa1.Total(), 1)
	t.Equal(goa2.Total(), 2)
	t.Equal(goa3.Total(), 2)

	// Check that original is not modified
	keys1 := make([]string, 0, 1)
	for key := range goa1.All() {
		keys1 = append(keys1, key)
	}
	t.DeepEqual(keys1, []string{""})
}

func TestGroupOrAttrs_WithAttrs_Unchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goa   *slogx.GroupOrAttrs
		attrs []slog.Attr
	}{
		{
			"nil goa with nil attrs",
			nil,
			nil,
		},
		{
			"nil goa with empty attrs",
			nil,
			[]slog.Attr{},
		},
		{
			"nil goa with empty groups",
			nil,
			[]slog.Attr{slog.Group("empty")},
		},
		{
			"existing goa with empty attrs",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			[]slog.Attr{},
		},
		{
			"existing goa with empty groups",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			[]slog.Attr{slog.Group("empty"), slog.Group("also_empty")},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			result := tc.goa.WithAttrs(tc.attrs)
			// Should return same instance or equivalent
			t.Equal(result.Total(), tc.goa.Total())
		})
	}
}

func TestGroupOrAttrs_WithGroup_Unchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goa  *slogx.GroupOrAttrs
	}{
		{
			"nil goa",
			nil,
		},
		{
			"zero goa",
			&slogx.GroupOrAttrs{},
		},
		{
			"existing goa",
			new(slogx.GroupOrAttrs).WithGroup("g1"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			result := tc.goa.WithGroup("")
			// Should return same instance
			t.Equal(result, tc.goa)
		})
	}
}

func TestGroupOrAttrs_Record_PreservesRecordAttrs(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	goa := new(slogx.GroupOrAttrs).
		WithAttrs([]slog.Attr{slog.Int("b", 2)}).
		WithGroup("g1")

	r := makeRecord("test msg",
		slog.Int("a", 1),
		slog.String("c", "3"))

	got := goa.Record(r)

	t.Equal(got.Time, r.Time)
	t.Equal(got.Message, r.Message)
	t.Equal(got.Level, r.Level)
	t.Equal(got.PC, r.PC)
	t.Equal(got.NumAttrs(), 2)
}

func TestGroupOrAttrs_Reverse(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	goa := new(slogx.GroupOrAttrs).
		WithAttrs([]slog.Attr{slog.Int("a", 1)}).
		WithGroup("g1").
		WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Int("c", 3)}).
		WithGroup("g2")

	// Collect items from All iterator
	allItems := make([]struct {
		group string
		attr  slog.Attr
	}, 0, goa.Total())
	for group, attr := range goa.All() {
		allItems = append(allItems, struct {
			group string
			attr  slog.Attr
		}{group, attr})
	}

	// Items should be in original order
	t.Must(t.Len(allItems, 5))
	t.Equal(allItems[0].attr.Key, "a")
	t.Equal(allItems[1].group, "g1")
	t.Equal(allItems[2].attr.Key, "b")
	t.Equal(allItems[3].attr.Key, "c")
	t.Equal(allItems[4].group, "g2")
}

func TestGroupOrAttrs_ChainedOperations(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	// Test complex chaining scenario
	goa := new(slogx.GroupOrAttrs).
		WithAttrs([]slog.Attr{slog.Int("a", 1)}).
		WithGroup("g1").
		WithAttrs([]slog.Attr{slog.Int("b", 2)}).
		WithGroup("g2").
		WithAttrs([]slog.Attr{slog.Int("c", 3)}).
		WithGroup("g3")

	t.Equal(goa.Total(), 6)

	rec := makeRecord("msg", slog.Int("d", 4))
	result := goa.Record(rec)

	// Result should be: a=1, g1:[b=2, g2:[c=3, g3:[d=4]]]
	attrs := collectAttrs(result)
	t.Must(t.Len(attrs, 2))
	t.Equal(attrs[0].Key, "a")
	t.Equal(attrs[0].Value.Int64(), int64(1))
	t.Equal(attrs[1].Key, "g1")

	// Navigate through nested groups: g1 contains b=2 and g2
	g1Attrs := attrs[1].Value.Group()
	t.Must(t.Len(g1Attrs, 2))
	t.Equal(g1Attrs[0].Key, "b")
	t.Equal(g1Attrs[0].Value.Int64(), int64(2))
	t.Equal(g1Attrs[1].Key, "g2")

	// g2 contains c=3 and g3
	g2Attrs := g1Attrs[1].Value.Group()
	t.Must(t.Len(g2Attrs, 2))
	t.Equal(g2Attrs[0].Key, "c")
	t.Equal(g2Attrs[0].Value.Int64(), int64(3))
	t.Equal(g2Attrs[1].Key, "g3")

	// g3 contains only d=4
	g3Attrs := g2Attrs[1].Value.Group()
	t.Equal(len(g3Attrs), 1)
	t.Equal(g3Attrs[0].Key, "d")
	t.Equal(g3Attrs[0].Value.Int64(), int64(4))
}

func TestGroupOrAttrs_Handler_NoGroupsOrAttrs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goa  *slogx.GroupOrAttrs
	}{
		{"nil goa", nil},
		{"zero goa", &slogx.GroupOrAttrs{}},
		{"empty attrs", new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Group("empty")})},
		{"empty group name", new(slogx.GroupOrAttrs).WithGroup("")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)
			// Mock without expectations fails the test on any call.
			next := NewMockHandler(gomock.NewController(tt))
			t.Equal(tc.goa.Handler(next), slog.Handler(next))
		})
	}
}

func TestGroupOrAttrs_Handler_CallOrder(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	ctrl := gomock.NewController(tt)
	next := NewMockHandler(ctrl)
	h1 := NewMockHandler(ctrl)
	h2 := NewMockHandler(ctrl)
	h3 := NewMockHandler(ctrl)
	h4 := NewMockHandler(ctrl)
	h5 := NewMockHandler(ctrl)
	gomock.InOrder(
		next.EXPECT().WithAttrs([]slog.Attr{slog.Int("a", 1)}).Return(h1),
		h1.EXPECT().WithGroup("g1").Return(h2),
		h2.EXPECT().WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Int("c", 3)}).Return(h3),
		h3.EXPECT().WithGroup("g2").Return(h4),
		h4.EXPECT().WithAttrs([]slog.Attr{slog.Int("d", 4)}).Return(h5),
	)

	goa := new(slogx.GroupOrAttrs).
		WithAttrs([]slog.Attr{slog.Int("a", 1)}).
		WithGroup("g1").
		WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Int("c", 3)}).
		WithGroup("g2").
		WithAttrs([]slog.Attr{slog.Int("d", 4)})

	t.Equal(goa.Handler(next), slog.Handler(h5))
}

func TestGroupOrAttrs_Handler_ClonesAttrs(tt *testing.T) {
	tt.Parallel()
	t := check.Must(tt)

	ctrl := gomock.NewController(tt)
	next := NewMockHandler(ctrl)
	derived := NewMockHandler(ctrl)
	next.EXPECT().WithAttrs(gomock.Any()).DoAndReturn(func(attrs []slog.Attr) slog.Handler {
		// A handler is allowed to modify the slice it received.
		attrs[0] = slog.Int("modified", 0)
		return derived
	}).Times(2)

	goa := new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)})

	t.Equal(goa.Handler(next), slog.Handler(derived))
	t.Equal(goa.Handler(next), slog.Handler(derived))
	for key, attr := range goa.All() {
		t.Equal(key, "")
		t.DeepEqual(attr, slog.Int("a", 1))
	}
}

func TestGroupOrAttrs_Handler_Output(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goa  *slogx.GroupOrAttrs
		want func(slog.Handler) slog.Handler
	}{
		{
			"attrs",
			new(slogx.GroupOrAttrs).WithAttrs([]slog.Attr{slog.Int("a", 1)}),
			func(h slog.Handler) slog.Handler {
				return h.WithAttrs([]slog.Attr{slog.Int("a", 1)})
			},
		},
		{
			"groups",
			new(slogx.GroupOrAttrs).WithGroup("g1").WithGroup("g2"),
			func(h slog.Handler) slog.Handler {
				return h.WithGroup("g1").WithGroup("g2")
			},
		},
		{
			"mixed",
			new(slogx.GroupOrAttrs).
				WithAttrs([]slog.Attr{slog.Int("a", 1)}).
				WithGroup("g1").
				WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Int("c", 3)}).
				WithGroup("g2"),
			func(h slog.Handler) slog.Handler {
				return h.
					WithAttrs([]slog.Attr{slog.Int("a", 1)}).
					WithGroup("g1").
					WithAttrs([]slog.Attr{slog.Int("b", 2), slog.Int("c", 3)}).
					WithGroup("g2")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			t := check.Must(tt)

			var got, want bytes.Buffer
			opts := &slog.HandlerOptions{ReplaceAttr: removeTime}
			r := makeRecord("msg", slog.Int("r", 0))

			t.Nil(tc.goa.Handler(slog.NewJSONHandler(&got, opts)).Handle(tt.Context(), r))
			t.Nil(tc.want(slog.NewJSONHandler(&want, opts)).Handle(tt.Context(), r))
			t.Equal(got.String(), want.String())
		})
	}
}

func makeRecord(msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(time.Now(), slog.LevelInfo, msg, 0xDEADBEEF)
	r.AddAttrs(attrs...)
	return r
}

func collectAttrs(r slog.Record) []slog.Attr {
	var attrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	return attrs
}
