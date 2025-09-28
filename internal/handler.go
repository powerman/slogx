// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.
//
// Modified by Alex Efros to remove JSON support and add Layout support.

package internal

import (
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/powerman/slogx/internal/buffer"
)

// Separator for attrs.
const attrSep = ' '

type commonHandler struct {
	opts              HandlerOptions
	preformattedAttrs []byte
	// groupPrefix holds the prefix for groups that were already pre-formatted.
	// A group will appear here when a call to WithGroup is followed by
	// a call to WithAttrs.
	groupPrefix string
	groups      []string // all groups started from WithGroup
	nOpenGroups int      // the number of groups opened in preformattedAttrs
	mu          *sync.Mutex
	w           io.Writer
}

func (h *commonHandler) clone() *commonHandler {
	// We can't use assignment because we can't copy the mutex.
	return &commonHandler{
		opts:              h.opts,
		preformattedAttrs: slices.Clip(h.preformattedAttrs),
		groupPrefix:       h.groupPrefix,
		groups:            slices.Clip(h.groups),
		nOpenGroups:       h.nOpenGroups,
		w:                 h.w,
		mu:                h.mu, // mutex shared among all clones of this handler
	}
}

// enabled reports whether l is greater than or equal to the
// minimum level.
func (h *commonHandler) enabled(l Level) bool {
	minLevel := LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return l >= minLevel
}

func (h *commonHandler) withAttrs(as []Attr) *commonHandler {
	// We are going to ignore empty groups, so if the entire slice consists of
	// them, there is nothing to do.
	if countEmptyGroups(as) == len(as) {
		return h
	}
	h2 := h.clone()
	// Pre-format the attributes as an optimization.
	state := h2.newHandleState((*buffer.Buffer)(&h2.preformattedAttrs), false)
	defer state.free()
	state.prefix.WriteString(h.groupPrefix)
	if pfa := h2.preformattedAttrs; len(pfa) > 0 {
		state.emitSep = true
	}
	// Remember the position in the buffer, in case all attrs are empty.
	pos := state.buf.Len()
	state.openGroups()
	if !state.appendAttrs(as) {
		state.buf.SetLen(pos)
	} else {
		// Remember the new prefix for later keys.
		h2.groupPrefix = state.prefix.String()
		// Remember how many opened groups are in preformattedAttrs,
		// so we don't open them again when we handle a Record.
		h2.nOpenGroups = len(h2.groups)
	}
	return h2
}

func (h *commonHandler) withGroup(name string) *commonHandler {
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

// handle is the internal implementation of Handler.Handle
// used by TextHandler and LayoutHandler.
func (h *commonHandler) handle(r Record) error {
	state := h.newHandleState(buffer.New(), true)
	defer state.free()
	// Built-in attributes. They are not in a group.
	stateGroups := state.groups
	state.groups = nil // So ReplaceAttrs sees no groups instead of the pre groups.
	rep := h.opts.ReplaceAttr
	// time
	if !r.Time.IsZero() {
		key := TimeKey
		val := r.Time.Round(0) // strip monotonic to match Attr behavior
		if rep == nil {
			state.appendKey(key)
			state.appendTime(val)
		} else {
			state.appendAttr(Time(key, val))
		}
	}
	// level
	key := LevelKey
	val := r.Level
	if rep == nil {
		state.appendKey(key)
		state.appendString(val.String())
	} else {
		state.appendAttr(Any(key, val))
	}
	// source
	if h.opts.AddSource {
		src := r.Source()
		if src == nil {
			src = &Source{}
		}
		state.appendAttr(Any(SourceKey, src))
	}
	key = MessageKey
	msg := r.Message
	if rep == nil {
		state.appendKey(key)
		state.appendString(msg)
	} else {
		state.appendAttr(String(key, msg))
	}
	state.groups = stateGroups // Restore groups passed to ReplaceAttrs.
	state.appendNonBuiltIns(r)
	state.buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(*state.buf)
	return err
}

func (s *handleState) appendNonBuiltIns(r Record) {
	// preformatted Attrs
	if pfa := s.h.preformattedAttrs; len(pfa) > 0 {
		if s.emitSep {
			s.buf.WriteByte(attrSep)
		}
		s.buf.Write(pfa)
		s.emitSep = true
	}
	// Attrs in Record -- unlike the built-in ones, they are in groups started
	// from WithGroup.
	// If the record has no Attrs, don't output any groups.
	if r.NumAttrs() > 0 {
		s.prefix.WriteString(s.h.groupPrefix)
		// The group may turn out to be empty even though it has attrs (for
		// example, ReplaceAttr may delete all the attrs).
		// So remember where we are in the buffer, to restore the position
		// later if necessary.
		pos := s.buf.Len()
		s.openGroups()
		empty := true
		r.Attrs(func(a Attr) bool {
			if s.appendAttr(a) {
				empty = false
			}
			return true
		})
		if empty {
			s.buf.SetLen(pos)
		}
	}
}

// handleState holds state for a single call to commonHandler.handle.
type handleState struct {
	h       *commonHandler
	buf     *buffer.Buffer
	freeBuf bool           // should buf be freed?
	emitSep bool           // whether to emit a separator before next key
	prefix  *buffer.Buffer // key prefix
	groups  *[]string      // pool-allocated slice of active groups, for ReplaceAttr
}

var groupPool = sync.Pool{New: func() any {
	s := make([]string, 0, 10)
	return &s
}}

func (h *commonHandler) newHandleState(buf *buffer.Buffer, freeBuf bool) handleState {
	s := handleState{
		h:       h,
		buf:     buf,
		freeBuf: freeBuf,
		emitSep: false,
		prefix:  buffer.New(),
	}
	if h.opts.ReplaceAttr != nil {
		s.groups = groupPool.Get().(*[]string)
		*s.groups = append(*s.groups, h.groups[:h.nOpenGroups]...)
	}
	return s
}

func (s *handleState) free() {
	if s.freeBuf {
		s.buf.Free()
	}
	if gs := s.groups; gs != nil {
		*gs = (*gs)[:0]
		groupPool.Put(gs)
	}
	s.prefix.Free()
}

func (s *handleState) openGroups() {
	for _, n := range s.h.groups[s.h.nOpenGroups:] {
		s.openGroup(n)
	}
}

// Separator for group names and keys.
const keyComponentSep = '.'

// openGroup starts a new group of attributes
// with the given name.
func (s *handleState) openGroup(name string) {
	s.prefix.WriteString(name)
	s.prefix.WriteByte(keyComponentSep)
	// Collect group names for ReplaceAttr.
	if s.groups != nil {
		*s.groups = append(*s.groups, name)
	}
}

// closeGroup ends the group with the given name.
func (s *handleState) closeGroup(name string) {
	(*s.prefix) = (*s.prefix)[:len(*s.prefix)-len(name)-1 /* for keyComponentSep */]
	if s.groups != nil {
		*s.groups = (*s.groups)[:len(*s.groups)-1]
	}
}

// appendAttrs appends the slice of Attrs.
// It reports whether something was appended.
func (s *handleState) appendAttrs(as []Attr) bool {
	nonEmpty := false
	for _, a := range as {
		if s.appendAttr(a) {
			nonEmpty = true
		}
	}
	return nonEmpty
}

// appendAttr appends the Attr's key and value.
// It handles replacement and checking for an empty key.
// It reports whether something was appended.
func (s *handleState) appendAttr(a Attr) bool {
	a.Value = a.Value.Resolve()
	if rep := s.h.opts.ReplaceAttr; rep != nil && a.Value.Kind() != KindGroup {
		var gs []string
		if s.groups != nil {
			gs = *s.groups
		}
		// a.Value is resolved before calling ReplaceAttr, so the user doesn't have to.
		a = rep(gs, a)
		// The ReplaceAttr function may return an unresolved Attr.
		a.Value = a.Value.Resolve()
	}
	// Elide empty Attrs.
	if a.Equal(Attr{}) {
		return false
	}
	// Special case: Source.
	if v := a.Value; v.Kind() == KindAny {
		if src, ok := v.Any().(*Source); ok {
			if isEmptySource(src) {
				return false
			}
			a.Value = StringValue(fmt.Sprintf("%s:%d", src.File, src.Line))
		}
	}
	if a.Value.Kind() == KindGroup {
		attrs := a.Value.Group()
		// Output only non-empty groups.
		if len(attrs) > 0 {
			// The group may turn out to be empty even though it has attrs (for
			// example, ReplaceAttr may delete all the attrs).
			// So remember where we are in the buffer, to restore the position
			// later if necessary.
			pos := s.buf.Len()
			// Inline a group with an empty key.
			if a.Key != "" {
				s.openGroup(a.Key)
			}
			if !s.appendAttrs(attrs) {
				s.buf.SetLen(pos)
				return false
			}
			if a.Key != "" {
				s.closeGroup(a.Key)
			}
		}
	} else {
		s.appendKey(a.Key)
		s.appendValue(a.Value)
	}
	return true
}

func (s *handleState) appendError(err error) {
	s.appendString(fmt.Sprintf("!ERROR:%v", err))
}

func (s *handleState) appendKey(key string) {
	if s.emitSep {
		s.buf.WriteByte(attrSep)
	}
	if s.prefix != nil && len(*s.prefix) > 0 {
		s.appendTwoStrings(string(*s.prefix), key)
	} else {
		s.appendString(key)
	}
	s.buf.WriteByte('=')
	s.emitSep = true
}

// appendTwoStrings implements appendString(prefix + key), but faster.
func (s *handleState) appendTwoStrings(x, y string) {
	buf := *s.buf
	switch {
	case !needsQuoting(x) && !needsQuoting(y):
		buf.WriteString(x)
		buf.WriteString(y)
	default:
		buf = strconv.AppendQuote(buf, x+y)
	}
	*s.buf = buf
}

func (s *handleState) appendString(str string) {
	if needsQuoting(str) {
		*s.buf = strconv.AppendQuote(*s.buf, str)
	} else {
		s.buf.WriteString(str)
	}
}

func (s *handleState) appendValue(v Value) {
	defer func() {
		if r := recover(); r != nil {
			// If it panics with a nil pointer, the most likely cases are
			// an encoding.TextMarshaler or error fails to guard against nil,
			// in which case "<nil>" seems to be the feasible choice.
			//
			// Adapted from the code in fmt/print.go.
			if v := reflect.ValueOf(v.Any()); v.Kind() == reflect.Pointer && v.IsNil() {
				s.appendString("<nil>")
				return
			}

			// Otherwise just print the original panic message.
			s.appendString(fmt.Sprintf("!PANIC: %v", r))
		}
	}()

	err := appendTextValue(s, v)
	if err != nil {
		s.appendError(err)
	}
}

func (s *handleState) appendTime(t time.Time) {
	*s.buf = appendRFC3339Millis(*s.buf, t)
}

func appendRFC3339Millis(b []byte, t time.Time) []byte {
	// Format according to time.RFC3339Nano since it is highly optimized,
	// but truncate it to use millisecond resolution.
	// Unfortunately, that format trims trailing 0s, so add 1/10 millisecond
	// to guarantee that there are exactly 4 digits after the period.
	const prefixLen = len("2006-01-02T15:04:05.000")
	n := len(b)
	t = t.Truncate(time.Millisecond).Add(time.Millisecond / 10)
	b = t.AppendFormat(b, time.RFC3339Nano)
	b = append(b[:n+prefixLen], b[n+prefixLen+1:]...) // drop the 4th digit
	return b
}
