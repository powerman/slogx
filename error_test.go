package slogx_test

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

func TestErrorAttrs(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	const badKey = "!BADKEY"

	var (
		e              = "new error"
		key            = "Key"
		group          = []string{"group"}
		err            = errors.New(e) //nolint:err113 // False positive.
		newError       = slogx.NewError(err, "key1", "value1", "key2", "value2")
		newErrorAttrs  = slogx.NewErrorAttrs(newError, slog.Int("key3", 3), slog.Int("key4", 4))
		errorAttrsFunc = slogx.ErrorAttrs()

		newErrorBadKey     = slogx.NewError(err, "key1")
		newErrorBadKeyAttr = slog.Any("", newErrorBadKey)
		groupValueBadKey   = slog.GroupValue(slog.String(badKey, "key1"), slog.String("", newErrorBadKey.Error()))

		newErrorAttr     = slogx.NewError(err, slog.Int("key", 3))
		newErrorAttrAttr = slog.Any("", newErrorAttr)
		groupValueAttr   = slog.GroupValue(slog.Int("key", 3), slog.String("", newErrorAttr.Error()))

		newErrorInt         = slogx.NewError(err, 8)
		newErrorIntAttr     = slog.Any("", newErrorInt)
		groupValueBadKeyInt = slog.GroupValue(slog.Any(badKey, 8), slog.String("", newErrorInt.Error()))

		strAttr    = slog.String("key", "value")
		anyAttr    = slog.Any("key", complex(2.2, 2.7))
		errAttr    = slog.Any("key", err)
		groupValue = slog.GroupValue(slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Int("key3", 3), slog.Int("key4", 4), slog.String(key, "new error"))

		wrapedError1 = fmt.Errorf("error1: %w", err)
		wrapedError2 = fmt.Errorf("error2: %w", newError)
		wrapedError3 = fmt.Errorf("error3: %w", newErrorAttrs)
		value1       = slog.AnyValue(wrapedError1)
		groupValue2  = slog.GroupValue(slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.String(key, "error2: new error"))
		groupValue3  = slog.GroupValue(slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Int("key3", 3), slog.Int("key4", 4), slog.String(key, "error3: new error"))
	)

	t.DeepEqual(slogx.NewError(nil), nil)
	t.DeepEqual(slogx.NewErrorAttrs(nil), nil)

	t.DeepEqual(slogx.NewError(err).Error(), e)
	t.DeepEqual(errorAttrsFunc(nil, newErrorBadKeyAttr), slog.Attr{Key: "", Value: groupValueBadKey})
	t.DeepEqual(errorAttrsFunc(nil, newErrorAttrAttr), slog.Attr{Key: "", Value: groupValueAttr})
	t.DeepEqual(errorAttrsFunc(nil, newErrorIntAttr), slog.Attr{Key: "", Value: groupValueBadKeyInt})

	t.DeepEqual(errorAttrsFunc(nil, strAttr), strAttr)
	t.DeepEqual(errorAttrsFunc(nil, anyAttr), anyAttr)
	t.DeepEqual(errorAttrsFunc(nil, errAttr), errAttr)

	t.DeepEqual(errorAttrsFunc(nil, slog.Any(key, newErrorAttrs)), slog.Attr{Key: "", Value: groupValue})
	t.DeepEqual(errorAttrsFunc(group, slog.Any(key, newErrorAttrs)), slog.Attr{Key: key, Value: groupValue})

	t.DeepEqual(errorAttrsFunc(nil, slog.Any(key, wrapedError1)), slog.Attr{Key: key, Value: value1})
	t.DeepEqual(errorAttrsFunc(nil, slog.Any(key, wrapedError2)), slog.Attr{Key: "", Value: groupValue2})
	t.DeepEqual(errorAttrsFunc(group, slog.Any(key, wrapedError3)), slog.Attr{Key: key, Value: groupValue3})
}
