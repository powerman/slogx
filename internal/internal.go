package internal

import (
	"log/slog"
)

type (
	Attr           = slog.Attr
	Handler        = slog.Handler
	HandlerOptions = slog.HandlerOptions
	Kind           = slog.Kind
	Level          = slog.Level
	LevelVar       = slog.LevelVar
	Leveler        = slog.Leveler
	Record         = slog.Record
	Source         = slog.Source
	Value          = slog.Value
)

const (
	TimeKey    = slog.TimeKey
	LevelKey   = slog.LevelKey
	MessageKey = slog.MessageKey
	SourceKey  = slog.SourceKey
)

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

const (
	KindAny       = slog.KindAny
	KindBool      = slog.KindBool
	KindDuration  = slog.KindDuration
	KindFloat64   = slog.KindFloat64
	KindInt64     = slog.KindInt64
	KindString    = slog.KindString
	KindTime      = slog.KindTime
	KindUint64    = slog.KindUint64
	KindGroup     = slog.KindGroup
	KindLogValuer = slog.KindLogValuer
)

var (
	Any            = slog.Any
	AnyValue       = slog.AnyValue
	Bool           = slog.Bool
	Duration       = slog.Duration
	Group          = slog.Group
	GroupValue     = slog.GroupValue
	Int            = slog.Int
	IntValue       = slog.IntValue
	New            = slog.New
	NewJSONHandler = slog.NewJSONHandler
	NewRecord      = slog.NewRecord
	String         = slog.String
	StringValue    = slog.StringValue
	Time           = slog.Time
)
