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

// LayoutHandlerOptions contains options for [NewLayoutHandler].
// These options extend [slog.HandlerOptions] to define output layout
// by controlling attributes order and formatting.
//
// PrefixKeys and SuffixKeys makes it possible to reorder attributes
// (including built-in attributes except [slog.MessageKey]) to appear
// right before message or at the end of the output respectively,
// in the fixed order defined by these slices.
//
// Format makes it possible to:
//
//   - Remove attribute from output (including built-in attributes).
//     This is more convenient than using ReplaceAttr for this purpose.
//   - Hide sensitive attribute value (e.g. secret).
//     This can be used as an additional layer of protection besides [slog.LogValuer]
//     and ReplaceAttr. Unlike removing the attribute, it makes possible to notice
//     the attempt to log the sensitive value without exposing the actual value.
//   - Ensure vertical alignment for attributes output before message
//     by adding padding to the left or right of the value
//     and truncating value from the end or the beginning.
//   - Output bare values without "key=" and/or attribute separator.
//     This allows more compact output for attributes which meaning is obvious
//     from their value or position (e.g. time, level, HTTP method, host:port, etc).
//   - Compact output for built-in attribute level by using short level names.
//   - Disable quoting to avoid cluttering the output with extra escaping backslashes
//     when the value is already in a safe format (e.g. JSON, Go syntax, etc).
//     This should be used with care to avoid misleading output.
//   - Disable quoting to output multiline values (e.g. stack traces, JSON, Go syntax, etc).
//     This should be used with care to avoid misleading output.
//   - Add custom prefix/suffix to attribute value (ANSI colors, brackets, etc).
//
// Here is an example configuration which ensures vertical alignment for message:
//
//	Format: map[string]string{
//		slog.LevelKey: " level=%3.3s", // Use alternative short level names.
//	},
//	SuffixKeys: []string{
//		slog.SourceKey, // Can be truncated and padded instead of moving to the end.
//	},
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
	// Use empty string format to remove the attr from output.
	// Use format without %v or %s verb to hide the actual value.
	//
	// The format is mostly a subset (just one extension) of the fmt package formats:
	//
	//   - Single '%v' or '%s' verb with optional flags, minimum and maximum width.
	//     - '%v' is value with default slog.TextHandler formatting (with quoting as needed).
	//     - '%s' is value with slog.TextHandler formatting without quoting.
	//   - Flag '-' for left alignment (default is right alignment).
	//   - Minimum width for padding value with spaces.
	//   - Positive maximum width for truncating value from the end if longer.
	//   - Negative maximum width for truncating value from the beginning if longer.
	//     (This is the only extension beyond fmt formats: accepting '-' after '.'.)
	//   - '%%' for a '%'
	//   - Other characters are output verbatim.
	//
	// Examples:
	//
	//   "%-5v"          - only value without attr separator, left aligned, minimum width 5
	//   " %10v"         - only value, right aligned, minimum width 10
	//   " %.10v"        - only value, maximum width 10 (output is truncated if longer)
	//   " %.-10v"       - same as above, but value is truncated from the beginning
	//   " key=%-10.8v"  - left aligned, min width 10, max width 8 (right padded 2+ spaces)
	//   " group.key=%v" - when used for key "group.key" will result in default output
	//                     (but always with a space prefix even if it's the first attribute)
	//   " pass=REDACTED"- when used for key "pass" will hide the actual value
	//   ""              - attribute is removed from output
	//   "\n%s"	     - unquoted multiline value starting on a new line
	//
	// Special cases:
	// - For slog.LevelKey minimum=3 and maximum=3 will result in short level names:
	//   "DBG", "INF", "WRN", "ERR", "D±n", "I+n", "W+n", "E+n".
	//
	// If two keys are output next to each other (e.g. "host" and "port") then it is
	// useful to include a custom separator (e.g. ':') in the format of the second key.
	// For example: {"host": " [%s", "port": ":%s]"} will output " [example.com:80]".
	//
	// NewLayoutHandler will panic is format is invalid
	// (unknown flag/verb after '%', more than one verb).
	Format map[string]string

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

// LayoutHandler is a handler created by [NewLayoutHandler]
// that writes [slog.Record] to an [io.Writer] in a text format
// designed for compact and easy to read output.
//
// It is a drop-in replacement for [slog.TextHandler] and implemented using modified
// slog.TextHandler code, so it has exactly same behaviour and similar performance
// when not using any of the extra options.
//
// To get improved output you should define order and formatting for some of the attributes
// you use in your application (see [LayoutHandlerOptions] for details and examples).
type LayoutHandler struct {
	next slog.Handler
}

// NewLayoutHandler creates a new [LayoutHandler] that writes to w, using the given options.
//
// Panics if opts.Format contains an invalid format.
func NewLayoutHandler(w io.Writer, opts *LayoutHandlerOptions) slog.Handler {
	if opts == nil {
		opts = &LayoutHandlerOptions{}
	}
	o := &internal.LayoutHandlerOptions{
		AddSource:        opts.AddSource,
		Level:            opts.Level,
		ReplaceAttr:      opts.ReplaceAttr,
		Format:           parseAttrFormatMap(opts.Format),
		PrefixKeys:       opts.PrefixKeys,
		SuffixKeys:       opts.SuffixKeys,
		RecordTimeFormat: opts.RecordTimeFormat,
		TimeFormat:       opts.TimeFormat,
	}
	return &LayoutHandler{
		next: internal.NewLayoutHandler(w, o),
	}
}

// Enabled implements [slog.Handler] interface.
func (h *LayoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// WithAttrs implements [slog.Handler] interface.
func (h *LayoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LayoutHandler{next: h.next.WithAttrs(attrs)}
}

// WithGroup implements [slog.Handler] interface.
func (h *LayoutHandler) WithGroup(name string) slog.Handler {
	return &LayoutHandler{next: h.next.WithGroup(name)}
}

// Handle implements [slog.Handler] interface.
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
