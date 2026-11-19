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

// Preallocated capacities, chosen to cover typical usage without resizing.
const (
	layoutAttrBufPrealloc = 32 // Preformatted prefix/suffix attr value.
	groupsSlicePrealloc   = 10 // Nesting depth of groups seen by ReplaceAttr.
)

// Larger pooled buffers are dropped to reduce peak allocation, same limit as in buffer.Free.
const maxPooledBufSize = 16 << 10

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

//nolint:gochecknoglobals // Used as a constant: Go has no struct constants.
var noFormat = AttrFormat{MaxWidth: -1}

// LayoutHandlerOptions contains options for [NewLayoutHandler].
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
	// If multiple attributes have the same key, including built-in ones,
	// only the last one is output (unlike slog.TextHandler, which outputs all of them):
	// a record attribute wins over one added by With, which wins over a built-in one.
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
	// If multiple attributes have the same key, including built-in ones,
	// only the last one is output (unlike slog.TextHandler, which outputs all of them):
	// a record attribute wins over one added by With, which wins over a built-in one.
	// If slog.MessageKey is present in SuffixKeys, it is ignored.
	// If same key is present multiple times in SuffixKeys, all but the first are ignored.
	// If same key is present in both PrefixKeys and SuffixKeys, it is output as a prefix.
	//
	// Keys not present in PrefixKeys and SuffixKeys are output as usual,
	// between the message and the suffix keys, in order they were added.
	SuffixKeys []string
}

// slotLeadSep reports whether a non-empty layout attr with key has a leading attrSep.
func (opts *LayoutHandlerOptions) slotLeadSep(key string) bool {
	_, ok := opts.Format[key]
	return !ok
}

// fast reports whether key needs neither ReplaceAttr nor Format.
func (opts *LayoutHandlerOptions) fast(key string) bool {
	_, ok := opts.Format[key]
	return opts.ReplaceAttr == nil && !ok
}

// layoutAttrs holds preformatted prefix and suffix attrs, in order of PrefixKeys and SuffixKeys.
// A non-empty attr has a leading attrSep unless its key has Format, as appendAttr writes it,
// so attrs are concatenated as is and slotLeadSep derives the leading attrSep from the key.
type layoutAttrs [][]byte

func makeLayoutAttrs(opts *LayoutHandlerOptions) layoutAttrs {
	return make([][]byte, len(opts.PrefixKeys)+len(opts.SuffixKeys))
}

// first returns the index of the first non-empty attr, or -1 if all are empty.
func (la layoutAttrs) first() int {
	for i, a := range la {
		if len(a) > 0 {
			return i
		}
	}
	return -1
}

// layoutSlot returns the index of key in layoutAttrs, or -1 if key is not a layout key.
func layoutSlot(key string, opts *LayoutHandlerOptions) int {
	i := slices.Index(opts.PrefixKeys, key)
	if i < 0 {
		i = slices.Index(opts.SuffixKeys, key)
		if i < 0 {
			return -1
		}
		i += len(opts.PrefixKeys)
	}
	return i
}

// LayoutHandler is a handler created by [NewLayoutHandler].
type LayoutHandler struct {
	opts                *LayoutHandlerOptions
	timeSlot            int         // index of TimeKey in layoutAttrs, or -1
	levelSlot           int         // index of LevelKey in layoutAttrs, or -1
	fastTime            bool        // TimeKey needs neither ReplaceAttr nor Format
	fastLevel           bool        // LevelKey needs neither ReplaceAttr nor Format
	fastMsg             bool        // MessageKey needs neither ReplaceAttr nor Format
	preformattedLeadSep bool        // preformattedAttrs has a leading attrSep
	layoutAttrs         layoutAttrs // preformatted prefix and suffix attrs
	preformattedAttrs   []byte
	groups              []string // all groups started from WithGroup
	prefix              []byte   // key prefix
	mu                  *sync.Mutex
	w                   io.Writer
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
	seen := make(map[string]bool, len(opts.PrefixKeys)+len(opts.SuffixKeys)+1 /* for MessageKey */)
	seen[MessageKey] = true // Ignore MessageKey in both.
	opts.PrefixKeys = dedupKeys(opts.PrefixKeys, seen)
	opts.SuffixKeys = dedupKeys(opts.SuffixKeys, seen)

	return &LayoutHandler{
		opts:        opts,
		timeSlot:    layoutSlot(TimeKey, opts),
		levelSlot:   layoutSlot(LevelKey, opts),
		fastTime:    opts.fast(TimeKey),
		fastLevel:   opts.fast(LevelKey),
		fastMsg:     opts.fast(MessageKey),
		layoutAttrs: makeLayoutAttrs(opts),
		mu:          &sync.Mutex{},
		w:           w,
	}
}

// dedupKeys returns keys without the ones in seen, keeping the first occurrence of each key.
// It adds each kept key to seen, so a later call drops keys kept by an earlier one.
func dedupKeys(keys []string, seen map[string]bool) []string {
	uniq := make([]string, 0, len(keys))
	for _, k := range keys {
		if !seen[k] {
			seen[k] = true
			uniq = append(uniq, k)
		}
	}
	return uniq
}

// clone returns a copy of h which shares the mutex with h.
// Slices are cloned or clipped, so modifying or appending to them does not modify h.
func (h *LayoutHandler) clone() *LayoutHandler {
	h2 := *h
	h2.layoutAttrs = slices.Clone(h.layoutAttrs)
	h2.preformattedAttrs = slices.Clip(h.preformattedAttrs)
	h2.groups = slices.Clip(h.groups)
	h2.prefix = slices.Clip(h.prefix)
	return &h2
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

// WithAttrs implements [Handler].
func (h *LayoutHandler) WithAttrs(as []Attr) Handler {
	// We are going to ignore empty groups, so if the entire slice consists of
	// them, there is nothing to do.
	if CountEmptyGroups(as) == len(as) {
		return h
	}
	h2 := h.clone()
	// Pre-format the attributes as an optimization.
	state := h2.newHandleState(h2.layoutAttrs, (*buffer.Buffer)(&h2.preformattedAttrs), false)
	defer state.free()
	state.leadSep = h2.preformattedLeadSep
	state.prefix.Write(h2.prefix)
	state.appendAttrs(as)
	h2.preformattedLeadSep = state.leadSep
	return h2
}

// WithGroup implements [Handler].
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

// Handle implements [Handler].
//
//nolint:funlen // Inherent to assembling time/level/source/message/prefix/suffix in one pass.
func (h *LayoutHandler) Handle(_ context.Context, r Record) error {
	// Built-in attrs may be written to layoutAttrs too, so Handle must not share it.
	state := h.newPrivateHandleState()
	defer state.free()
	// Built-in attributes. They are not in a group.
	stateGroups := state.groups
	state.groups = nil // So ReplaceAttrs sees no groups instead of the pre groups.
	// time
	if !r.Time.IsZero() {
		key := TimeKey
		val := r.Time.Round(0) // strip monotonic to match Attr behavior
		if h.fastTime {
			orig := state.beginLayout(h.timeSlot)
			state.appendKey(key)
			state.appendTime(key, val)
			state.endLayout(h.timeSlot, orig)
		} else {
			state.appendAttr(Time(key, val))
		}
	}
	// level
	key := LevelKey
	val := r.Level
	if h.fastLevel {
		orig := state.beginLayout(h.levelSlot)
		state.appendKey(key)
		state.appendString(val.String(), noFormat)
		state.endLayout(h.levelSlot, orig)
	} else {
		state.appendAttr(Any(key, val))
	}
	// source
	if h.opts.AddSource {
		src := recordSource(r)
		if src == nil {
			src = &Source{}
		}
		state.appendAttr(Any(SourceKey, src))
	}
	// Prefix attrs are inserted before the message.
	messagePos := state.buf.Len()
	// message
	key = MessageKey
	msg := r.Message
	if h.fastMsg {
		state.appendKey(key)
		state.appendString(msg, noFormat)
	} else {
		state.appendAttr(String(key, msg))
	}

	state.groups = stateGroups // Restore groups passed to ReplaceAttrs.
	state.appendNonBuiltIns(r)

	nPrefix := len(h.opts.PrefixKeys)
	iPrefix := state.layoutAttrs[:nPrefix].first()
	leadSep := state.lineLeadSep(messagePos, iPrefix)
	if iPrefix >= 0 {
		insertLayoutAttrs(state.buf, messagePos, state.layoutAttrs[iPrefix:nPrefix])
	}
	writeLayoutAttrs(state.buf, state.layoutAttrs[nPrefix:])
	state.buf.WriteByte('\n')
	line := *state.buf
	if leadSep {
		line = line[1:]
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(line)
	return err
}

// handleState holds state for a single call to LayoutHandler.Handle or WithAttrs.
type handleState struct {
	h           *LayoutHandler
	layoutAttrs layoutAttrs
	// layoutAttrsBuf is a pooled backing array for layoutAttrs, kept across reuses of handleState.
	layoutAttrsBuf layoutAttrs
	// slotBufs are pooled buffers for layoutAttrs values written by a private handleState,
	// kept across reuses of handleState.
	slotBufs [][]byte
	// prefixBuf is a pooled backing buffer for prefix, kept across reuses of handleState.
	prefixBuf []byte
	buf       *buffer.Buffer
	prefix    *buffer.Buffer // key prefix
	groups    *[]string      // pool-allocated slice of active groups, for ReplaceAttr
	private   bool           // are layoutAttrs private (see newPrivateHandleState)?
	freeBuf   bool           // should buf be freed?
	leadSep   bool           // buf has a leading attrSep
}

type origBuf struct {
	buf     *buffer.Buffer
	leadSep bool
}

// beginLayout redirects output to layoutAttrs[i], replacing its previous value.
// Negative i means no redirection.
// The returned value must be passed to endLayout.
func (s *handleState) beginLayout(i int) origBuf {
	orig := origBuf{buf: s.buf, leadSep: s.leadSep}
	switch {
	case i < 0:
	case s.private:
		s.slotBufs[i] = s.slotBufs[i][:0]
		s.buf = (*buffer.Buffer)(&s.slotBufs[i])
	default:
		// The old value may be shared with other handlers, so it must not be reused.
		s.layoutAttrs[i] = make([]byte, 0, layoutAttrBufPrealloc)
		s.buf = (*buffer.Buffer)(&s.layoutAttrs[i])
	}
	return orig
}

// endLayout restores output redirected by beginLayout,
// keeping the original leadSep.
func (s *handleState) endLayout(i int, orig origBuf) {
	if i < 0 {
		return
	}
	if s.private {
		s.layoutAttrs[i] = s.slotBufs[i]
	}
	s.buf = orig.buf
	s.leadSep = orig.leadSep
}

func writeLayoutAttrs(buf *buffer.Buffer, attrs layoutAttrs) {
	for _, a := range attrs {
		buf.Write(a)
	}
}

// insertLayoutAttrs inserts attrs into buf at pos.
func insertLayoutAttrs(buf *buffer.Buffer, pos int, attrs layoutAttrs) {
	n := 0
	for _, a := range attrs {
		n += len(a)
	}
	b := *buf
	b = slices.Grow(b, n)[:len(b)+n]
	copy(b[pos+n:], b[pos:])
	for _, a := range attrs {
		pos += copy(b[pos:], a)
	}
	*buf = b
}

// lineLeadSep reports whether the line assembled by Handle has a leading attrSep,
// which must be dropped.
// iPrefix is the index of the first non-empty prefix attr, or -1.
// It must be called before suffix attrs are appended to buf.
func (s *handleState) lineLeadSep(messagePos, iPrefix int) bool {
	opts := s.h.opts
	switch {
	case messagePos == 0 && iPrefix >= 0:
		return opts.slotLeadSep(opts.PrefixKeys[iPrefix])
	case s.buf.Len() > 0:
		return s.leadSep
	}
	i := s.layoutAttrs[len(opts.PrefixKeys):].first()
	return i >= 0 && opts.slotLeadSep(opts.SuffixKeys[i])
}

func (s *handleState) appendNonBuiltIns(r Record) {
	// preformatted Attrs
	if pfa := s.h.preformattedAttrs; len(pfa) > 0 {
		if len(*s.buf) == 0 {
			s.leadSep = s.h.preformattedLeadSep
		}
		s.buf.Write(pfa)
	}
	// Preformatted attrs with layout keys follow (and thus replace) built-in attrs.
	for i, a := range s.h.layoutAttrs {
		if a != nil {
			s.layoutAttrs[i] = a
		}
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

var groupPool = sync.Pool{New: func() any { //nolint:gochecknoglobals // sync.Pool must be package-level.
	s := make([]string, 0, groupsSlicePrealloc)
	return &s
}}

var handleStatePool = sync.Pool{New: func() any { //nolint:gochecknoglobals // sync.Pool must be package-level.
	return &handleState{}
}}

// newHandleState returns a pooled handleState which modifies layoutAttrs in place.
func (h *LayoutHandler) newHandleState(layoutAttrs layoutAttrs, buf *buffer.Buffer, freeBuf bool) *handleState {
	s := handleStatePool.Get().(*handleState)
	s.h = h
	s.layoutAttrs = layoutAttrs
	s.buf = buf
	s.freeBuf = freeBuf
	s.leadSep = false
	s.prefix = (*buffer.Buffer)(&s.prefixBuf)
	if h.opts.ReplaceAttr != nil {
		s.groups = groupPool.Get().(*[]string)
		*s.groups = append(*s.groups, h.groups...)
	}
	return s
}

// newPrivateHandleState is like newHandleState,
// but the state works on private empty layoutAttrs,
// which are valid only until free.
// Preformatted layoutAttrs of h are added by appendNonBuiltIns.
func (h *LayoutHandler) newPrivateHandleState() *handleState {
	n := len(h.layoutAttrs)
	s := h.newHandleState(nil, buffer.New(), true)
	s.layoutAttrsBuf = slices.Grow(s.layoutAttrsBuf[:0], n)[:n]
	s.layoutAttrs = s.layoutAttrsBuf
	s.private = true
	if n > len(s.slotBufs) {
		s.slotBufs = append(s.slotBufs, make([][]byte, n-len(s.slotBufs))...)
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
	for i, b := range s.slotBufs {
		if cap(b) > maxPooledBufSize {
			s.slotBufs[i] = nil
		}
	}
	if cap(s.prefixBuf) > maxPooledBufSize {
		s.prefixBuf = nil
	}
	clear(s.layoutAttrsBuf) // avoid retaining attrs of this call
	*s = handleState{       // avoid retaining other references
		layoutAttrsBuf: s.layoutAttrsBuf[:0],
		slotBufs:       s.slotBufs,
		prefixBuf:      s.prefixBuf[:0],
	}
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
//
//nolint:gocognit // Handles group inlining, ReplaceAttr, Source and layout redirection in one pass.
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
			if sourceIsEmpty(src) {
				return
			}
			a.Value = StringValue(fmt.Sprintf("%s:%d", src.File, src.Line))
		}
	}
	//nolint:nestif // Inlining a group with an empty key requires this branching.
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
		slot := layoutSlot(key, s.h.opts)
		orig := s.beginLayout(slot)

		if format, ok := s.h.opts.Format[key]; ok {
			s.appendFormat(format, key, a.Value)
		} else {
			s.appendKey(key)
			s.appendValue(key, a.Value, noFormat)
		}

		s.endLayout(slot, orig)
	}
}

func (s *handleState) key(key string) string {
	if len(*s.prefix) > 0 {
		return string(*s.prefix) + key
	}
	return key
}

func (s *handleState) appendFormat(format AttrFormat, key string, v Value) {
	s.buf.WriteString(format.Prefix)

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

	s.buf.WriteString(format.Suffix)
}

//nolint:funlen,gocognit // Truncation with quoting/ellipsis/alignment inherently branches a lot.
func (s *handleState) appendFormatValue(key string, v Value, format AttrFormat) {
	pos := s.buf.Len()
	s.appendValue(key, v, format)
	// Count runes in the appended value up to max amount needed for next checks.
	n := 0
	// Detect quoted values to close the quote after truncation.
	quoted := pos < s.buf.Len() && (*s.buf)[pos] == '"'
	// Position after value's last rune that fits into MaxWidth when we cut from the end.
	// The last rune is MaxWidth-1 for unquoted values and MaxWidth-2 for quoted.
	cutPos := pos
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
		switch {
		case w == 1:
			s.buf.SetLen(pos)
			s.buf.WriteString(`…`)
		case quoted && w == 2:
			s.buf.SetLen(pos)
			s.buf.WriteString(`……`)
		case format.TruncFromStart:
			// Keep the last runes which fit into MaxWidth after the marker,
			// including the closing quote of quoted values.
			marker, keep := "…", w-1
			if quoted {
				marker = `"…`
				keep--
			}
			startPos := s.buf.Len()
			for range keep {
				_, size := utf8.DecodeLastRune((*s.buf)[pos:startPos])
				startPos -= size
			}
			*s.buf = slices.Replace(*s.buf, pos, startPos, []byte(marker)...)
		case quoted:
			s.buf.SetLen(cutPos)
			s.buf.WriteString(`…"`)
		default:
			s.buf.SetLen(cutPos)
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

// appendKey always writes attrSep before the key.
// If it starts the output line, Handle drops it.
// Such a leading attrSep is tracked by leadSep flags instead of checking for ' ',
// because a Format Prefix may start with ' ' too.
func (s *handleState) appendKey(key string) {
	if len(*s.buf) == 0 {
		s.leadSep = true
	}
	s.buf.WriteByte(attrSep)
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
		*s.buf = t.AppendFormat(*s.buf, s.h.opts.RecordTimeFormat)
	case key != TimeKey && s.h.opts.TimeFormat != "":
		*s.buf = t.AppendFormat(*s.buf, s.h.opts.TimeFormat)
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
	t = t.Truncate(time.Millisecond).Add(time.Millisecond / 10) //nolint:mnd // 1/10 millisecond, see comment above.
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
