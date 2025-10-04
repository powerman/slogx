package slogx

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/powerman/slogx/internal"
)

// LayoutHandlerOptions contains options for NewLayoutHandler.
type LayoutHandlerOptions struct {
	// AddSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	AddSource bool

	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes LevelInfo.
	// The handler calls Level.Level for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler

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
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr

	// Format specifies per-attribute formatting options.
	//
	// If an attribute's key is present in the map, the corresponding
	// formatting options are applied when outputting the attribute,
	// otherwise the attribute is output in the default TextHandler format.
	//
	// Key should be the full key, including group prefixes separated by '.'.
	//
	// All attributes included in Format are output without attribute separator (' '),
	// key and '='.
	// Include these parts in format as prefix if needed.
	//
	// Use empty string value for a key to remove the attr from output.
	//
	// The format is mostly a subset (just one extension) of the fmt package formats:
	// - single '%v' or '%s' with optional flags, minimum and maximum width for value
	//   - '%v' for default slog.TextHandler formatting (with quoting as needed)
	//   - '%s' for slog.TextHandler formatting without quoting
	// - flag '-' for left alignment (default is right alignment)
	// - negative maximum width for truncating value from the beginning instead of the end
	//   (this is the only extension beyond fmt formats: accepting '-' after '.')
	// - '%%' for a '%'
	// - other characters are output verbatim
	//
	// %s is an attr's value quoted and formatted in same way as used by TextHandler.
	//
	// Examples:
	//   "%-5v"          - only value without attr separator, left aligned, minimum width 5
	//   " %10v"         - only value, right aligned, minimum width 10
	//   " %.10v"        - only value, maximum width 10 (output is truncated if longer)
	//   " %.-10v"       - same as above, but value is truncated from the beginning
	//   " key=%-10.8v"  - left aligned, min width 10, max width 8 (right padded 2+ spaces)
	//   " group.key=%v" - when used for key "group.key" will result in default output
	//                     (but always with a space prefix even if it's the first attribute)
	//   " pass=REDACTED"- when used for key "pass" will hide the actual value
	//   ""              - attribute is removed from output
	//
	// Special cases:
	// - For slog.TimeKey minimum and maximum width means substring offset and length:
	//  "%11.12v" will output "15:04:05.999", "%.10v" will output "2006-01-02".
	// - For slog.LevelKey minimum=3 and maximum=3 will result in short level names:
	//   "DBG", "INF", "WRN", "ERR", "D±n", "I+n", "W+n", "E+n".
	//
	// If two keys are output next to each other (e.g. "host" and "port") then it is
	// useful to include a custom separator (e.g. ':') in the format of the second key.
	// For example: {"host": " [%s", "port": ":%s]"} will output " [example.com:80]".
	//
	// NewLayoutHandler will panic is format is invalid (unknown flag/verb after '%').
	Format map[string]string

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

// LayoutHandler is a handler that writes Records to an io.Writer in a text format
// designed for easy human reading.
type LayoutHandler struct {
	next slog.Handler
}

// NewLayoutHandler creates a new LayoutHandler that writes to w, using the given options.
//
// These options extend slog.HandlerOptions with extra options to define attrs layout by
// controlling their order of attributes and their formatting.
// If these extra options are not provided then the handler behaves exactly like slog.TextHandler.
//
// opts.PrefixKeys and opts.SuffixKeys make it possible to reorder attributes
// (including built-in attributes except slog.MessageKey) to appear
// before slog.MessageKey or at the end of the output respectively,
// in the fixed order defined by these slices.
//
// opts.Format makes it possible to:
// - Remove attributes from output (including built-in attributes).
// - Hide sensitive attribute values (e.g. passwords).
// - Ensure vertical alignment for PrefixKeys.
// - Truncate long attribute values.
// - Output bare values without "key=" and attribute separator.
// - Add custom prefix/suffix to attribute value.
//
// Here is an example of minimal configuration which ensures vertical alignment for message:
//
//	Format: map[string]string{
//		slog.LevelKey: " level=%3.3s", // short level with fixed width 3
//	},
//	SuffixKeys: []string{slog.SourceKey}, // source width is unknown, put it at the end
//
// NewLayoutHandler panics if opts.Format contains an invalid format.
func NewLayoutHandler(w io.Writer, opts *LayoutHandlerOptions) slog.Handler {
	if opts == nil {
		opts = &LayoutHandlerOptions{}
	}
	o := &internal.LayoutHandlerOptions{
		AddSource:   opts.AddSource,
		Level:       opts.Level,
		ReplaceAttr: opts.ReplaceAttr,
		Format:      parseAttrFormatMap(opts.Format),
		PrefixKeys:  opts.PrefixKeys,
		SuffixKeys:  opts.SuffixKeys,
	}
	return &LayoutHandler{
		next: internal.NewLayoutHandler(w, o),
	}
}

// Enabled implements slog.Handler interface.
func (h *LayoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// WithAttrs implements slog.Handler interface.
func (h *LayoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LayoutHandler{next: h.next.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler interface.
func (h *LayoutHandler) WithGroup(name string) slog.Handler {
	return &LayoutHandler{next: h.next.WithGroup(name)}
}

// Handle implements slog.Handler interface.
func (h *LayoutHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.next.Handle(ctx, r)
}

func parseAttrFormatMap(m map[string]string) map[string]internal.AttrFormat {
	if len(m) == 0 {
		return nil
	}
	af := make(map[string]internal.AttrFormat, len(m))
	for k, v := range m {
		af[k] = parseAttrFormat(v)
	}
	return af
}

var reAttrFormat = regexp.MustCompile(`^((?:[^%]+|%%)*)(%(-?)(\d*)([.](-?)(\d*))?([vs]))?((?:[^%]+|%%)*)$`)

func parseAttrFormat(s string) internal.AttrFormat {
	ms := reAttrFormat.FindStringSubmatch(s)
	if ms == nil {
		panic("slogx: invalid attr format: " + s)
	}
	var (
		prefix         = ms[1]
		hasVerb        = ms[2] != ""
		alignLeft      = ms[3] == "-"
		minWidth       = ms[4]
		hasMaxWidth    = ms[5] != ""
		truncFromStart = ms[6] == "-"
		maxWidth       = ms[7]
		verb           = ms[8]
		suffix         = ms[9]
	)

	af := internal.AttrFormat{
		Prefix:         strings.ReplaceAll(prefix, "%%", "%"),
		Suffix:         strings.ReplaceAll(suffix, "%%", "%"),
		MinWidth:       0,
		MaxWidth:       -1,
		AlignRight:     !alignLeft,
		TruncFromStart: truncFromStart,
		SkipQuote:      verb == "s",
	}

	var err error
	if minWidth != "" {
		af.MinWidth, err = strconv.Atoi(minWidth)
		if err != nil {
			panic("slogx: invalid attr format (min width): " + s)
		}
	}
	if hasMaxWidth {
		af.MaxWidth = 0 // MaxWidth present without value means 0.
	}
	if maxWidth != "" {
		af.MaxWidth, err = strconv.Atoi(maxWidth)
		if err != nil {
			panic("slogx: invalid attr format (max width): " + s)
		}
	}
	if !hasVerb {
		af.MaxWidth = 0 // No %v or %s verb means no value output.
	}

	return af
}
