package slogx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"runtime"
	"slices"
	"strings"
)

const KeyPackage = "package"

// ColumnarHandler is a modified version of slog.TextHandler that provides
// additional options for convenient package name logging, methods to
// add pefix/suffix attrs and set print format for attr values.
type ColumnarHandler struct {
	handler     slog.Handler
	opts        ColumnarHandlerOption
	opList      []data
	listPrefix  []slog.Attr
	listSufix   []slog.Attr
	attrsFormat map[string]string
}

// ColumnarHandlerOption is an option that allows to add package name to attrs
// and set mapping for package names. It also supports slog.HandlerOptions.
type ColumnarHandlerOption struct {
	AddPackage bool
	ModPackage map[string]string
	slog.HandlerOptions
}

type data struct {
	group string
	attrs []slog.Attr
}

// NewColumnarHandler creates new ColumnarHandler with given options.
func NewColumnarHandler(w io.Writer, opts *ColumnarHandlerOption) *ColumnarHandler {
	const sizeHint = 16
	if opts == nil {
		opts = &ColumnarHandlerOption{
			HandlerOptions: slog.HandlerOptions{},
		}
	}
	return &ColumnarHandler{
		handler:     slog.NewTextHandler(w, &opts.HandlerOptions),
		opts:        *opts,
		opList:      make([]data, 0, sizeHint),
		listPrefix:  make([]slog.Attr, 0, sizeHint),
		listSufix:   make([]slog.Attr, 0, sizeHint),
		attrsFormat: make(map[string]string, sizeHint),
	}
}

// Enabled works as (slog.Handler).Enabled. It reports
// whether the ColumnarHandler handles records at the given level.
func (h *ColumnarHandler) Enabled(_ context.Context, l slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.HandlerOptions.Level != nil {
		minLevel = h.opts.HandlerOptions.Level.Level()
	}
	return l >= minLevel
}

// Handle works as (slog.Handler).Handler. It also add prefix/suffix attrs
// and format attr values.
func (h *ColumnarHandler) Handle(ctx context.Context, r slog.Record) error {
	handler := h.handler
	if h.opts.AddPackage {
		handler = h.handler.WithAttrs([]slog.Attr{slog.String(KeyPackage, h.getPackageName(r.PC))})
	}

	handler = h.addAttrsAndGroups(handler)

	return handler.Handle(ctx, r)
}

// WithAttrs works as (slog.Handler).WithAttrs. It returns a new Handler
// whose attributes consists of h's attributes followed by attrs.
func (h *ColumnarHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	columnarHandler := h.clone()
	columnarHandler.opList = append(columnarHandler.opList, data{
		attrs: attrs,
	})
	return columnarHandler
}

// WithGroup works as (slog.Handler).WithGroup.
func (h *ColumnarHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	columnarHandler := h.clone()
	columnarHandler.opList = append(columnarHandler.opList, data{
		group: name,
	})
	return columnarHandler
}

// AppendAttrsPrefix creates a new ColumnarHandler with appended attrs to current
// prefix attrs.
func (h *ColumnarHandler) AppendAttrsPrefix(attrs []slog.Attr) *ColumnarHandler {
	columnarHandler := h.clone()
	columnarHandler.listPrefix = append(columnarHandler.listPrefix, attrs...)
	return columnarHandler
}

// SetAttrsPrefix creates a new ColumnarHandler with replaced current prefix attrs.
func (h *ColumnarHandler) SetAttrsPrefix(attrs []slog.Attr) *ColumnarHandler {
	columnarHandler := h.clone()
	columnarHandler.listPrefix = attrs
	return columnarHandler
}

// PrependAttrsSufix creates a new ColumnarHandler with prepended attrs to current
// suffix attrs.
func (h *ColumnarHandler) PrependAttrsSufix(attrs []slog.Attr) *ColumnarHandler {
	columnarHandler := h.clone()
	columnarHandler.listSufix = append(append([]slog.Attr(nil), attrs...), columnarHandler.listSufix...)
	return columnarHandler
}

// SetAttrsSufix creates a new ColumnarHandler with replaced current suffix attrs.
func (h *ColumnarHandler) SetAttrsSufix(attrs []slog.Attr) *ColumnarHandler {
	columnarHandler := h.clone()
	columnarHandler.listSufix = attrs
	return columnarHandler
}

// SetAttrsFormat creates a new ColumnarHandler with set format for attrs values
// as fmt.Sprintf(format,val).
func (h *ColumnarHandler) SetAttrsFormat(format map[string]string) *ColumnarHandler {
	columnarHandler := h.clone()
	for k, v := range format {
		columnarHandler.attrsFormat[k] = v
	}
	return columnarHandler
}

func (h *ColumnarHandler) clone() *ColumnarHandler {
	return &ColumnarHandler{
		handler:     h.handler,
		opts:        h.opts,
		opList:      slices.Clip(h.opList),
		listPrefix:  slices.Clip(h.listPrefix),
		listSufix:   slices.Clip(h.listSufix),
		attrsFormat: maps.Clone(h.attrsFormat),
	}
}

func (h *ColumnarHandler) addAttrsAndGroups(handler slog.Handler) slog.Handler {
	handler = handler.WithAttrs(h.formatValues(h.listPrefix))
	for _, op := range h.opList {
		if len(op.group) > 0 {
			handler = handler.WithGroup(op.group)
		} else {
			handler = handler.WithAttrs(h.formatValues(op.attrs))
		}
	}
	handler = handler.WithAttrs(h.formatValues(h.listSufix))

	return handler
}

func (h *ColumnarHandler) formatValues(attrs []slog.Attr) []slog.Attr {
	var formatedAttrs []slog.Attr
	for _, attr := range attrs {
		if format, ok := h.attrsFormat[attr.Key]; !ok {
			formatedAttrs = append(formatedAttrs, attr)
		} else {
			formatedAttrs = append(formatedAttrs, slog.Attr{
				Key:   attr.Key,
				Value: slog.StringValue(fmt.Sprintf(format, attr.Value.String())),
			})
		}
	}
	return formatedAttrs
}

func (h *ColumnarHandler) getPackageName(pc uintptr) string {
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	if f.Function == "" {
		return ""
	}
	for key, val := range h.opts.ModPackage {
		switch {
		case strings.HasPrefix(f.Function, key):
			return val
		case strings.HasSuffix(key, "/...") && strings.HasPrefix(f.Function, key[:len(key)-3]):
			return val
		}
	}
	dir := strings.Split(f.Function, "/")
	pkg := strings.Split(dir[len(dir)-1], ".")
	return pkg[0]
}
