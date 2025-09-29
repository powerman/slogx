// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.
//
// Modified by Alex Efros to remove JSON support and add Layout support.

package internal

import (
	"context"
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

// AttrFormat specifies how to format an attribute.
//
// Value {MaxWidth: -1} results in outputting just the value, without attrSep, key and '='.
// Zero value results in outputting nothing, same as removing the attr using ReplaceAttr.
type AttrFormat struct {
	Prefix     string // Printed instead of attr key.
	Suffix     string // Printed after the attr value.
	MinWidth   int    // Minimum width of the attr value.
	MaxWidth   int    // Maximum width of the attr value. -1 means no limit. 0 means no value.
	AlignRight bool
}

type LayoutHandlerOptions struct {
	// AddSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	AddSource bool

	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes LevelInfo.
	// The handler calls Level.Level for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	Level Leveler

	// ReplaceAttr is called to rewrite each non-group attribute before it is logged.
	// The attribute's value has been resolved (see [Value.Resolve]).
	// If ReplaceAttr returns a zero Attr, the attribute is discarded.
	//
	// The built-in attributes with keys "time", "level", "source", and "msg"
	// are passed to this function, except that time is omitted
	// if zero, and source is omitted if AddSource is false.
	//
	// The first argument is a list of currently open groups that contain the
	// Attr. It must not be retained or modified. ReplaceAttr is never called
	// for Group attributes, only their contents. For example, the attribute
	// list
	//
	//     Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
	//
	// results in consecutive calls to ReplaceAttr with the following arguments:
	//
	//     nil, Int("a", 1)
	//     []string{"g"}, Int("b", 2)
	//     nil, Int("c", 3)
	//
	// ReplaceAttr can be used to change the default keys of the built-in
	// attributes, convert types (for example, to replace a `time.Time` with the
	// integer seconds since the Unix epoch), sanitize personal information, or
	// remove attributes from the output.
	ReplaceAttr func(groups []string, a Attr) Attr

	// Format specifies per-attribute formatting options.
	//
	// If an attribute's key is present in the map, the corresponding
	// formatting options are applied when outputting the attribute,
	// otherwise the attribute is output in the default TextHandler format.
	//
	// Key should be the full key, including group prefixes separated by '.'.
	//
	// All attributes included in Format are output without attrSep (' '), key and '='.
	// Include these parts in AttrFormat.Prefix as needed.
	//
	// Use zero AttrFormat value for a key to remove the attr from output.
	Format map[string]AttrFormat
}

type LayoutHandler struct {
	opts              LayoutHandlerOptions
	preformattedAttrs []byte
	groups            []string // all groups started from WithGroup
	prefix            []byte   // key prefix
	mu                *sync.Mutex
	w                 io.Writer
}

// NewLayoutHandler creates a [LayoutHandler] that writes to w,
// using the given options.
// If opts is nil, the default options are used.
func NewLayoutHandler(w io.Writer, opts *LayoutHandlerOptions) *LayoutHandler {
	if opts == nil {
		opts = &LayoutHandlerOptions{}
	}
	return &LayoutHandler{
		opts: *opts,
		mu:   &sync.Mutex{},
		w:    w,
	}
}

func (h *LayoutHandler) clone() *LayoutHandler {
	// We can't use assignment because we can't copy the mutex.
	return &LayoutHandler{
		opts:              h.opts,
		preformattedAttrs: slices.Clip(h.preformattedAttrs),
		groups:            slices.Clip(h.groups),
		prefix:            slices.Clip(h.prefix),
		mu:                h.mu, // mutex shared among all clones of this handler
		w:                 h.w,
	}
}

// Enabled reports whether l is greater than or equal to the
// minimum level.
func (h *LayoutHandler) Enabled(_ context.Context, l Level) bool {
	minLevel := LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return l >= minLevel
}

func (h *LayoutHandler) WithAttrs(as []Attr) Handler {
	// We are going to ignore empty groups, so if the entire slice consists of
	// them, there is nothing to do.
	if countEmptyGroups(as) == len(as) {
		return h
	}
	h2 := h.clone()
	// Pre-format the attributes as an optimization.
	state := h2.newHandleState((*buffer.Buffer)(&h2.preformattedAttrs), false)
	defer state.free()
	state.prefix.Write(h.prefix)
	if pfa := h2.preformattedAttrs; len(pfa) > 0 {
		state.emitSep = true
	}
	state.appendAttrs(as)
	return h2
}

func (h *LayoutHandler) WithGroup(name string) Handler {
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	h2.prefix = append(h2.prefix, name...)
	h2.prefix = append(h2.prefix, keyComponentSep)
	return h2
}

// Handle is the internal implementation of Handler.Handle
// used by TextHandler and LayoutHandler.
func (h *LayoutHandler) Handle(_ context.Context, r Record) error {
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
		if _, ok := h.opts.Format[key]; rep == nil && !ok {
			state.appendKey(key)
			state.appendTime(val)
		} else {
			state.appendAttr(Time(key, val))
		}
	}
	// level
	key := LevelKey
	val := r.Level
	if _, ok := h.opts.Format[key]; rep == nil && !ok {
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
	if _, ok := h.opts.Format[key]; rep == nil && !ok {
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
		s.prefix.Write(s.h.prefix)
		r.Attrs(func(a Attr) bool {
			s.appendAttr(a)
			return true
		})
	}
}

// handleState holds state for a single call to commonHandler.handle.
type handleState struct {
	h       *LayoutHandler
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

func (h *LayoutHandler) newHandleState(buf *buffer.Buffer, freeBuf bool) handleState {
	s := handleState{
		h:       h,
		buf:     buf,
		freeBuf: freeBuf,
		emitSep: false,
		prefix:  buffer.New(),
	}
	if h.opts.ReplaceAttr != nil {
		s.groups = groupPool.Get().(*[]string)
		*s.groups = append(*s.groups, h.groups...)
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
func (s *handleState) appendAttrs(as []Attr) {
	for _, a := range as {
		s.appendAttr(a)
	}
}

// appendAttr appends the Attr's key and value.
// It handles replacement and checking for an empty key.
// It reports whether something was appended.
func (s *handleState) appendAttr(a Attr) {
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
		return
	}
	// Special case: Source.
	if v := a.Value; v.Kind() == KindAny {
		if src, ok := v.Any().(*Source); ok {
			if isEmptySource(src) {
				return
			}
			a.Value = StringValue(fmt.Sprintf("%s:%d", src.File, src.Line))
		}
	}
	if a.Value.Kind() == KindGroup {
		attrs := a.Value.Group()
		// Output only non-empty groups.
		if len(attrs) > 0 {
			// Inline a group with an empty key.
			if a.Key != "" {
				s.openGroup(a.Key)
			}
			s.appendAttrs(attrs)
			if a.Key != "" {
				s.closeGroup(a.Key)
			}
		}
	} else {
		key := s.key(a.Key)
		if format, ok := s.h.opts.Format[key]; ok {
			s.appendFormat(format, a.Value)
		} else {
			s.appendKey(key)
			s.appendValue(a.Value)
		}
	}
}

func (s *handleState) key(key string) string {
	if s.prefix != nil && len(*s.prefix) > 0 {
		return string(*s.prefix) + key
	}
	return key
}

func (s *handleState) appendFormat(format AttrFormat, v Value) {
	if format.Prefix != "" {
		s.buf.WriteString(format.Prefix)
	}

	if format.MaxWidth != 0 {
		pos := s.buf.Len()
		s.appendValue(v)
		n := s.buf.Len() - pos
		if w := format.MaxWidth; w > 0 && n > w {
			s.buf.SetLen(pos + w)
			n = w
		}
		if w := format.MinWidth; w > n {
			pad := w - n
			s.buf.SetLen(pos + w)
			padStart := pos + n
			if format.AlignRight {
				padStart = pos
				copy((*s.buf)[pos+pad:], (*s.buf)[pos:pos+n])
			}
			for i := range pad {
				(*s.buf)[padStart+i] = ' '
			}
		}
	}

	if format.Suffix != "" {
		s.buf.WriteString(format.Suffix)
	}

	s.emitSep = true
}

func (s *handleState) appendError(err error) {
	s.appendString(fmt.Sprintf("!ERROR:%v", err))
}

func (s *handleState) appendKey(key string) {
	if s.emitSep {
		s.buf.WriteByte(attrSep)
	}
	s.appendString(key)
	s.buf.WriteByte('=')
	s.emitSep = true
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
