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
	"unicode/utf8"

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

	// PrefixKeys specifies keys that, if present, output just before the message key,
	// in order given by the slice.
	// Key should be the full key, including group prefixes separated by '.'.
	//
	// If multiple attributes have the same key only the last one is output.
	// If slog.MessageKey is present in PrefixKeys, it is ignored.
	// If same key is present multiple times in PrefixKeys, all but the first are ignored.
	// If same key is present in both PrefixKeys and SuffixKeys, it is output as a prefix.
	//
	// Keys not present in PrefixKeys and SuffixKeys are output as usual,
	// between the message and the suffix keys, in order they were added.
	PrefixKeys []string

	// SuffixKeys specifies keys that, if present, output after all other attributes,
	// in order given by the slice.
	// Key should be the full key, including group prefixes separated by '.'.
	//
	// If multiple attributes have the same key only the last one is output.
	// If slog.MessageKey is present in SuffixKeys, it is ignored.
	// If same key is present multiple times in SuffixKeys, all but the first are ignored.
	// If same key is present in both PrefixKeys and SuffixKeys, it is output as a prefix.
	//
	// Keys not present in PrefixKeys and SuffixKeys are output as usual,
	// between the message and the suffix keys, in order they were added.
	SuffixKeys []string
}

type layoutAttrs struct {
	keys []string // opts.{Prefix,Suffix}Keys
	attr [][]byte // index from keys -> subslice of buf, len(attr)==len(keys)
	buf  []byte   // preformatted attrs (without attrSep)
}

func makeLayoutAttrs(keys []string) layoutAttrs {
	return layoutAttrs{
		keys: keys,
		attr: make([][]byte, len(keys)),
	}
}

func (la layoutAttrs) clone() layoutAttrs {
	return layoutAttrs{
		keys: la.keys,
		attr: slices.Clone(la.attr),
		buf:  slices.Clip(la.buf),
	}
}

func (la layoutAttrs) pos() int {
	return len(la.buf)
}

func (la *layoutAttrs) buffer() *buffer.Buffer {
	return (*buffer.Buffer)(&la.buf)
}

func (la layoutAttrs) index(key string) int {
	return slices.Index(la.keys, key)
}

func (la layoutAttrs) set(index int, start int) {
	la.attr[index] = la.buf[start:]
}

type LayoutHandler struct {
	opts              LayoutHandlerOptions
	prefixAttrs       layoutAttrs // preformatted prefix attrs
	suffixAttrs       layoutAttrs // preformatted suffix attrs
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

	// Remove duplicate keys in PrefixKeys and SuffixKeys,
	// keeping the first occurrence of each key.
	// If a key is present in both, it is kept in PrefixKeys.
	prefixKeys := make([]string, 0, len(opts.PrefixKeys))
	suffixKeys := make([]string, 0, len(opts.SuffixKeys))
	seen := make(map[string]bool, len(opts.PrefixKeys)+len(opts.SuffixKeys)+1 /* for MessageKey */)
	seen[MessageKey] = true // Ignore MessageKey in both.
	for _, k := range opts.PrefixKeys {
		if !seen[k] {
			seen[k] = true
			prefixKeys = append(prefixKeys, k)
		}
	}
	for _, k := range opts.SuffixKeys {
		if !seen[k] {
			seen[k] = true
			suffixKeys = append(suffixKeys, k)
		}
	}
	opts.PrefixKeys = prefixKeys
	opts.SuffixKeys = suffixKeys

	return &LayoutHandler{
		opts:        *opts,
		prefixAttrs: makeLayoutAttrs(opts.PrefixKeys),
		suffixAttrs: makeLayoutAttrs(opts.SuffixKeys),
		mu:          &sync.Mutex{},
		w:           w,
	}
}

func (h *LayoutHandler) clone() *LayoutHandler {
	// We can't use assignment because we can't copy the mutex.
	return &LayoutHandler{
		opts:              h.opts,
		prefixAttrs:       h.prefixAttrs.clone(),
		suffixAttrs:       h.suffixAttrs.clone(),
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
	state := h2.newHandleState(
		h2.prefixAttrs, h2.suffixAttrs,
		(*buffer.Buffer)(&h2.preformattedAttrs), false)
	defer state.free()
	state.prefix.Write(h.prefix)
	if pfa := h2.preformattedAttrs; len(pfa) > 0 {
		state.emitSep = true
	}
	state.appendAttrs(as)
	h2.prefixAttrs = state.prefixAttrs
	h2.suffixAttrs = state.suffixAttrs
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
	state := h.newHandleState(h.prefixAttrs.clone(), h.suffixAttrs.clone(), buffer.New(), true)
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
	messagePos := state.buf.Len()
	_, messageHasFormat := h.opts.Format[key]
	messageEmitSep := state.emitSep && !messageHasFormat
	if rep == nil && !messageHasFormat {
		state.appendKey(key)
		state.appendString(msg)
	} else {
		state.appendAttr(String(key, msg))
	}
	messageEmitSep = messageEmitSep && state.buf.Len() > messagePos

	state.groups = stateGroups // Restore groups passed to ReplaceAttrs.
	state.appendNonBuiltIns(r)

	buf := state.buf
	prefixAttrs := &state.prefixAttrs
	suffixAttrs := &state.suffixAttrs
	if prefixAttrs.pos() > 0 {
		buf = buffer.New()
		defer buf.Free()
		// Insert prefix attrs before the message.
		buf.Write((*state.buf)[:messagePos])
		for i, a := range prefixAttrs.attr {
			if len(a) > 0 {
				k := prefixAttrs.keys[i]
				if _, ok := h.opts.Format[k]; buf.Len() > 0 && !ok {
					buf.WriteByte(attrSep)
				}
				buf.Write(a)
			}
		}
		// Write the message and the rest of non-suffix attrs.
		switch {
		case buf.Len() == 0 && messageEmitSep:
			messagePos++ // skip leading separator
		case buf.Len() > 0 && !messageEmitSep:
			buf.WriteByte(attrSep)
		}
		buf.Write((*state.buf)[messagePos:])
	}
	if suffixAttrs.pos() > 0 {
		// Append suffix attrs after all other attrs.
		for i, a := range suffixAttrs.attr {
			if len(a) > 0 {
				k := suffixAttrs.keys[i]
				if _, ok := h.opts.Format[k]; buf.Len() > 0 && !ok {
					buf.WriteByte(attrSep)
				}
				buf.Write(a)
			}
		}
	}
	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(*buf)
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

// handleState holds state for a single call to LayoutHandler.Handle.
type handleState struct {
	h           *LayoutHandler
	prefixAttrs layoutAttrs
	suffixAttrs layoutAttrs
	buf         *buffer.Buffer
	freeBuf     bool           // should buf be freed?
	emitSep     bool           // whether to emit a separator before next key
	prefix      *buffer.Buffer // key prefix
	groups      *[]string      // pool-allocated slice of active groups, for ReplaceAttr
}

var groupPool = sync.Pool{New: func() any {
	s := make([]string, 0, 10)
	return &s
}}

var handleStatePool = sync.Pool{New: func() any {
	return &handleState{}
}}

func (h *LayoutHandler) newHandleState(
	prefixAttrs layoutAttrs, suffixAttrs layoutAttrs,
	buf *buffer.Buffer, freeBuf bool,
) *handleState {
	s := handleStatePool.Get().(*handleState)
	s.h = h
	s.prefixAttrs = prefixAttrs
	s.suffixAttrs = suffixAttrs
	s.buf = buf
	s.freeBuf = freeBuf
	s.emitSep = false
	s.prefix = buffer.New()
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
	handleStatePool.Put(s)
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

		// Redirect output to prefixAttrs.buf or suffixAttrs.buf if needed.
		// Keep the original emitSep state when output is redirected.
		var layout *layoutAttrs
		var layoutIndex int
		if layoutIndex = s.prefixAttrs.index(key); layoutIndex >= 0 {
			layout = &s.prefixAttrs
		} else if layoutIndex = s.suffixAttrs.index(key); layoutIndex >= 0 {
			layout = &s.suffixAttrs
		}

		origBuf := s.buf
		origEmitSep := s.emitSep
		var layoutPos int
		if layout != nil {
			layoutPos = layout.pos()
			s.buf = layout.buffer()
			s.emitSep = false
		}

		if format, ok := s.h.opts.Format[key]; ok {
			s.appendFormat(format, a.Value)
		} else {
			s.appendKey(key)
			s.appendValue(a.Value)
		}

		if layout != nil {
			layout.set(layoutIndex, layoutPos)
			s.buf = origBuf
			s.emitSep = origEmitSep
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
		// Count runes in the appended value up to max amount needed for next checks.
		n := 0
		// Detect quoted values to close the quote after truncation.
		quoted := pos < s.buf.Len() && (*s.buf)[pos] == '"'
		// Position after value's last rune that fits into MaxWidth (when enforced).
		// The last rune is MaxWidth-1 for unquoted values and MaxWidth-2 for quoted.
		cutPos := pos // Valid for MaxWidth=1 and quoted MaxWidth=2.
		if nMax := max(format.MinWidth, format.MaxWidth); nMax > 0 {
			for i := pos; i < s.buf.Len() && n <= nMax; {
				_, size := utf8.DecodeRune((*s.buf)[i:])
				i += size
				n++
				// Update cutPos for unquoted MaxWidth>=2 and quoted MaxWidth>2.
				switch {
				case format.MaxWidth >= 2 && !quoted && n == format.MaxWidth-1:
					cutPos = i
				case format.MaxWidth > 2 && quoted && n == format.MaxWidth-2:
					cutPos = i
				}
			}
		}
		if w := format.MaxWidth; w > 0 && n > w {
			s.buf.SetLen(cutPos)
			switch {
			case w == 2 && quoted:
				s.buf.WriteString(`……`)
			case quoted:
				s.buf.WriteString(`…"`)
			default:
				s.buf.WriteString(`…`)
			}
			n = w
		}
		if w := format.MinWidth; w > n {
			pad := w - n
			padStart := s.buf.Len()
			s.buf.SetLen(padStart + pad)
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
