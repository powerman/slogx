// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-go file.
//
// Based on handler.go:
// - Type commonHandler renamed to LayoutHandler.
// - Removed JSON support.
// - Added Layout support.

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
//
// Special cases:
//   - LevelKey with MinWidth=3 and MaxWidth=3 outputs short level string (e.g. "WRN").
type AttrFormat struct {
	Prefix         string // Printed instead of attr key.
	Suffix         string // Printed after the attr value.
	MinWidth       int    // Minimum width of the attr value.
	MaxWidth       int    // Maximum width of the attr value. -1 means no limit. 0 means no value.
	AlignRight     bool   // MinWidth padding added to the left.
	TruncFromStart bool   // MaxWidth truncate from the beginning.
	SkipQuote      bool   // Do not quote the value, even if needed.
}

var noFormat = AttrFormat{MaxWidth: -1}

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

	// RecordTimeFormat specifies the time format for the built-in slog.TimeKey attribute
	// instead of default (RFC3339 with millisecond precision).
	RecordTimeFormat string

	// TimeFormat specifies the time format for user-defined time.Time attributes
	// instead of default (RFC3339 with millisecond precision).
	TimeFormat string

	// Format specifies per-attribute formatting options.
	//
	// If an attribute's key is present in the map,
	// the corresponding formatting options are applied when outputting the attribute,
	// otherwise the attribute is output in the default slog.TextHandler format.
	//
	// Key should be the full key, including group prefixes separated by '.'.
	//
	// All attributes included in Format are output without attribute separator (' '),
	// key and '='. Include these parts in format as prefix if needed.
	//
	// Use zero AttrFormat value to remove the attr from output.
	Format map[string]AttrFormat

	// PrefixKeys specifies keys that, if present, output just before the message key,
	// in order given by the slice.
	//
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
	//
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

type layoutAttrs [][]byte // index from prefix/suffix keys -> preformatted attr

func makeLayoutAttrs(opts *LayoutHandlerOptions) layoutAttrs {
	return make([][]byte, len(opts.PrefixKeys)+len(opts.SuffixKeys))
}

func (la layoutAttrs) clone() layoutAttrs {
	return slices.Clone(la)
}

func (la layoutAttrs) hasPrefix(opts *LayoutHandlerOptions) bool {
	for i := range opts.PrefixKeys {
		if len(la[i]) > 0 {
			return true
		}
	}
	return false
}

func (la layoutAttrs) buffer(key string, opts *LayoutHandlerOptions) *buffer.Buffer {
	i := slices.Index(opts.PrefixKeys, key)
	if i < 0 {
		i = slices.Index(opts.SuffixKeys, key)
		if i < 0 {
			return nil
		}
		i += len(opts.PrefixKeys)
	}
	la[i] = make([]byte, 0, 32) // replace old value, preallocate some space
	return (*buffer.Buffer)(&la[i])
}

type startSepState int

const (
	sepNone     startSepState = iota // no attrs yet
	sepSkipped                       // first attr has no Format (output without required attrSep)
	sepIncluded                      // first attr has Format (does not need attrSep)
)

type LayoutHandler struct {
	opts                   *LayoutHandlerOptions
	layoutAttrs            layoutAttrs // preformatted prefix and suffix attrs
	preformattedAttrs      []byte
	preformattedAttrsStart startSepState
	groups                 []string // all groups started from WithGroup
	prefix                 []byte   // key prefix
	mu                     *sync.Mutex
	w                      io.Writer
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
		opts:        opts,
		layoutAttrs: makeLayoutAttrs(opts),
		mu:          &sync.Mutex{},
		w:           w,
	}
}

func (h *LayoutHandler) clone() *LayoutHandler {
	// We can't use assignment because we can't copy the mutex.
	return &LayoutHandler{
		opts:                   h.opts,
		layoutAttrs:            h.layoutAttrs.clone(),
		preformattedAttrs:      slices.Clip(h.preformattedAttrs),
		preformattedAttrsStart: h.preformattedAttrsStart,
		groups:                 slices.Clip(h.groups),
		prefix:                 slices.Clip(h.prefix),
		mu:                     h.mu, // mutex shared among all clones of this handler
		w:                      h.w,
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
	state := h2.newHandleState(h2.layoutAttrs, (*buffer.Buffer)(&h2.preformattedAttrs), false)
	defer state.free()
	state.bufStart = h2.preformattedAttrsStart
	state.prefix.Write(h2.prefix)
	state.appendAttrs(as)
	h2.layoutAttrs = state.layoutAttrs
	h2.preformattedAttrsStart = state.bufStart
	return h2
}

func (h *LayoutHandler) WithGroup(name string) Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	h2.prefix = append(h2.prefix, name...)
	h2.prefix = append(h2.prefix, keyComponentSep)
	return h2
}

// Handle is the internal implementation of Handler.Handle
// used by TextHandler and LayoutHandler.
func (h *LayoutHandler) Handle(_ context.Context, r Record) error {
	var state *handleState
	if r.NumAttrs() == 0 {
		state = h.newHandleState(h.layoutAttrs, buffer.New(), true)
	} else {
		state = h.newHandleState(h.layoutAttrs.clone(), buffer.New(), true)
	}
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
			state.appendTime(key, val)
		} else {
			state.appendAttr(Time(key, val))
		}
	}
	// level
	key := LevelKey
	val := r.Level
	if _, ok := h.opts.Format[key]; rep == nil && !ok {
		state.appendKey(key)
		state.appendString(val.String(), noFormat)
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
	// To inject prefix attrs before the message, we need to know where the message
	// starts and is next attr appended at this point has skipped attrSep
	// (may happens if there were no attrs yet and next attr has no Format).
	state.bufStart = sepNone
	messagePos := state.buf.Len()
	// message
	key = MessageKey
	msg := r.Message
	if _, ok := h.opts.Format[key]; rep == nil && !ok {
		state.appendKey(key)
		state.appendString(msg, noFormat)
	} else {
		state.appendAttr(String(key, msg))
	}

	state.groups = stateGroups // Restore groups passed to ReplaceAttrs.
	state.appendNonBuiltIns(r)

	buf := state.buf
	if state.layoutAttrs.hasPrefix(h.opts) {
		buf = buffer.New()
		defer buf.Free()
		// Insert prefix attrs before the message.
		buf.Write((*state.buf)[:messagePos])
		for i, k := range h.opts.PrefixKeys {
			a := state.layoutAttrs[i]
			if len(a) > 0 {
				if _, ok := h.opts.Format[k]; buf.Len() > 0 && !ok {
					buf.WriteByte(attrSep)
				}
				buf.Write(a)
			}
		}
		// Write the message and the rest of non-suffix attrs.
		if buf.Len() > 0 && state.bufStart == sepSkipped {
			buf.WriteByte(attrSep)
		}
		buf.Write((*state.buf)[messagePos:])
	}
	// Append suffix attrs after all other attrs.
	offset := len(h.opts.PrefixKeys)
	for i, k := range h.opts.SuffixKeys {
		a := state.layoutAttrs[offset+i]
		if len(a) > 0 {
			if _, ok := h.opts.Format[k]; buf.Len() > 0 && !ok {
				buf.WriteByte(attrSep)
			}
			buf.Write(a)
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
		if len(*s.buf) > 0 && s.h.preformattedAttrsStart == sepSkipped {
			s.buf.WriteByte(attrSep)
			if s.bufStart == sepNone {
				s.bufStart = sepIncluded
			}
		} else if s.bufStart == sepNone {
			s.bufStart = s.h.preformattedAttrsStart
		}
		s.buf.Write(pfa)

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
	layoutAttrs layoutAttrs
	buf         *buffer.Buffer
	freeBuf     bool // should buf be freed?
	bufStart    startSepState
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

func (h *LayoutHandler) newHandleState(layoutAttrs layoutAttrs, buf *buffer.Buffer, freeBuf bool) *handleState {
	s := handleStatePool.Get().(*handleState)
	s.h = h
	s.layoutAttrs = layoutAttrs
	s.buf = buf
	s.freeBuf = freeBuf
	s.bufStart = sepNone
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
	*s = handleState{} // avoid retaining references
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

		// Redirect output to layoutAttrs if needed.
		// Keep the original bufStart state when output is redirected.
		layoutBuf := s.layoutAttrs.buffer(key, s.h.opts)
		origBuf := s.buf
		origBufStart := s.bufStart
		if layoutBuf != nil {
			s.buf = layoutBuf
		}

		if format, ok := s.h.opts.Format[key]; ok {
			s.appendFormat(format, key, a.Value)
		} else {
			s.appendKey(key)
			s.appendValue(key, a.Value, noFormat)
		}

		if layoutBuf != nil {
			s.buf = origBuf
			s.bufStart = origBufStart
		}
	}
}

func (s *handleState) key(key string) string {
	if s.prefix != nil && len(*s.prefix) > 0 {
		return string(*s.prefix) + key
	}
	return key
}

func (s *handleState) appendFormat(format AttrFormat, key string, v Value) {
	if format.Prefix != "" {
		s.buf.WriteString(format.Prefix)
	}

	switch {
	// Special case: short level for "%3.3s" format of LevelKey.
	case format.MinWidth == 3 && format.MaxWidth == 3 && key == LevelKey:
		if l, ok := v.Any().(Level); ok {
			s.buf.WriteString(shortLevel(l))
		} else {
			s.appendFormatValue(key, v, format)
		}

	case format.MaxWidth != 0:
		s.appendFormatValue(key, v, format)

	case format.MinWidth > 0:
		for range format.MinWidth {
			s.buf.WriteByte(' ')
		}
	}

	if format.Suffix != "" {
		s.buf.WriteString(format.Suffix)
	}

	if s.bufStart == sepNone {
		if format.Prefix != "" || format.MinWidth > 0 || format.MaxWidth != 0 || format.Suffix != "" {
			s.bufStart = sepIncluded
		}
	}
}

func (s *handleState) appendFormatValue(key string, v Value, format AttrFormat) {
	pos := s.buf.Len()
	s.appendValue(key, v, format)
	// Count runes in the appended value up to max amount needed for next checks.
	n := 0
	// Detect quoted values to close the quote after truncation.
	quoted := pos < s.buf.Len() && (*s.buf)[pos] == '"'
	// If Alternate is false, we cut from the end.
	// Position after value's last rune that fits into MaxWidth (when enforced).
	// The last rune is MaxWidth-1 for unquoted values and MaxWidth-2 for quoted.
	cutPos := pos // Valid for MaxWidth=1 and quoted MaxWidth=2.
	// If Alternate is true, we cut from the beginning.
	// Position of the first rune that does fit into MaxWidth.
	// The first rune is MaxWidth-1 from the end for unquoted values and
	// MaxWidth-2 from the end for quoted values.
	startPos := pos
	if nMax := max(format.MinWidth, format.MaxWidth); nMax > 0 {
		var sizes []int // Ring buffer of rune sizes for Alternate.
		if format.TruncFromStart && format.MaxWidth > 0 {
			sizes = make([]int, format.MaxWidth)
		}
		for i := pos; i < s.buf.Len() && (n <= nMax || format.TruncFromStart); {
			_, size := utf8.DecodeRune((*s.buf)[i:])
			i += size
			n++
			if len(sizes) > 0 {
				if n > format.MaxWidth {
					startPos += sizes[n%len(sizes)]
				}
				sizes[n%len(sizes)] = size
			}
			// Update cutPos for unquoted MaxWidth>=2 and quoted MaxWidth>2.
			switch {
			case format.MaxWidth >= 2 && !quoted && n == format.MaxWidth-1:
				cutPos = i
			case format.MaxWidth > 2 && quoted && n == format.MaxWidth-2:
				cutPos = i
			}
		}
		if len(sizes) > 0 && n > format.MaxWidth {
			startPos += sizes[(n+1)%len(sizes)] // Skip 1 for … marker.
			if quoted && len(sizes) > 1 {
				startPos += sizes[(n+2)%len(sizes)] // Skip 1 for opening quote.
			}
		}
	}
	if w := format.MaxWidth; w > 0 && n > w {
		if format.TruncFromStart {
			switch {
			case w == 1:
				s.buf.SetLen(pos)
				s.buf.WriteString(`…`)
			case quoted && w == 2:
				s.buf.SetLen(pos)
				s.buf.WriteString(`……`)
			default:
				var buf []byte
				overwrite := 3 // 3 byte for …
				if quoted {
					overwrite++ // 1 byte for opening quote
				}
				if startPos-pos >= overwrite {
					buf = (*s.buf)[startPos:]
				} else {
					buf = append([]byte(nil), (*s.buf)[startPos:]...)
				}
				s.buf.SetLen(pos)
				if quoted {
					s.buf.WriteString(`"…`)
				} else {
					s.buf.WriteString(`…`)
				}
				s.buf.Write(buf)
			}
		} else {
			s.buf.SetLen(cutPos)
			switch {
			case !quoted:
				s.buf.WriteString(`…`)
			case w == 1:
				s.buf.WriteString(`…`)
			case w == 2:
				s.buf.WriteString(`……`)
			default:
				s.buf.WriteString(`…"`)
			}
		}
		n = w
	}
	if w := format.MinWidth; w > n {
		pad := w - n
		padStart := s.buf.Len()
		s.buf.SetLen(padStart + pad)
		if format.AlignRight {
			padStart = pos
			copy((*s.buf)[pos+pad:], (*s.buf)[pos:])
		}
		for i := range pad {
			(*s.buf)[padStart+i] = ' '
		}
	}
}

func (s *handleState) appendError(err error) {
	s.appendString(fmt.Sprintf("!ERROR:%v", err), noFormat)
}

func (s *handleState) appendKey(key string) {
	if len(*s.buf) > 0 {
		s.buf.WriteByte(attrSep)
		if s.bufStart == sepNone {
			s.bufStart = sepIncluded
		}
	} else if s.bufStart == sepNone {
		s.bufStart = sepSkipped
	}
	s.appendString(key, noFormat)
	s.buf.WriteByte('=')
}

func (s *handleState) appendString(str string, format AttrFormat) {
	if !format.SkipQuote && needsQuoting(str) {
		*s.buf = strconv.AppendQuote(*s.buf, str)
	} else {
		s.buf.WriteString(str)
	}
}

func (s *handleState) appendValue(key string, v Value, format AttrFormat) {
	defer func() {
		if r := recover(); r != nil {
			// If it panics with a nil pointer, the most likely cases are
			// an encoding.TextMarshaler or error fails to guard against nil,
			// in which case "<nil>" seems to be the feasible choice.
			//
			// Adapted from the code in fmt/print.go.
			if v := reflect.ValueOf(v.Any()); v.Kind() == reflect.Pointer && v.IsNil() {
				s.appendString("<nil>", noFormat)
				return
			}

			// Otherwise just print the original panic message.
			s.appendString(fmt.Sprintf("!PANIC: %v", r), noFormat)
		}
	}()

	err := appendTextValue(s, key, v, format)
	if err != nil {
		s.appendError(err)
	}
}

func (s *handleState) appendTime(key string, t time.Time) {
	switch {
	case key == TimeKey && s.h.opts.RecordTimeFormat != "":
		s.buf.WriteString(t.Format(s.h.opts.RecordTimeFormat))
	case key != TimeKey && s.h.opts.TimeFormat != "":
		s.buf.WriteString(t.Format(s.h.opts.TimeFormat))
	default:
		*s.buf = appendRFC3339Millis(*s.buf, t)
	}
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

func shortLevel(l Level) string {
	switch {
	case l == LevelDebug:
		return "DBG"
	case l < LevelInfo:
		return fmt.Sprintf("D%+d", l-LevelDebug)
	case l == LevelInfo:
		return "INF"
	case l < LevelWarn:
		return fmt.Sprintf("I%+d", l-LevelInfo)
	case l == LevelWarn:
		return "WRN"
	case l < LevelError:
		return fmt.Sprintf("W%+d", l-LevelWarn)
	case l == LevelError:
		return "ERR"
	default:
		return fmt.Sprintf("E%+d", l-LevelError)
	}
}
