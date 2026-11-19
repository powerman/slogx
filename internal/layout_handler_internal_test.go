package internal

import (
	"io"
	"strings"
	"testing"

	"github.com/powerman/check"
)

// A slot buffer grown by a large attr must not be retained by the pool,
// while small slot buffers are kept for reuse.
//
//nolint:paralleltest // Reads slotBufs after free puts them into the pool, other tests must not reuse them.
func TestHandleState_FreeDropsLargeSlotBufs(tt *testing.T) {
	t := check.Must(tt)

	h := NewLayoutHandler(io.Discard, &LayoutHandlerOptions{SuffixKeys: []string{"small", "large"}})
	s := h.newPrivateHandleState()
	s.appendAttr(String("small", "v"))
	s.appendAttr(String("large", strings.Repeat("x", maxPooledBufSize)))
	bufs := s.slotBufs
	s.free()

	t.Equal(string(bufs[0]), " small=v")
	t.Zero(cap(bufs[1]))
}
