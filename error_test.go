package slogx_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/powerman/check"

	"github.com/powerman/slogx"
)

// errorNoAttrs returns slogx.errorNoAttrs{err: err}.
func errorNoAttrs(err error) error {
	a := slogx.ErrorAttrs()(nil, slog.Any("err", err))
	if a.Value.Kind() != slog.KindGroup {
		err = slogx.NewError(err, "_fake", nil)
		a = slogx.ErrorAttrs()(nil, slog.Any("err", err))
	}
	group := a.Value.Group()
	a = group[len(group)-1]
	return a.Value.Any().(error)
}

func TestErrorAttrs(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	const badKey = "!BADKEY"

	var (
		e               = "new error"
		key             = "Key"
		group           = []string{"group"}
		err             = errors.New(e) //nolint:err113 // False positive.
		newError        = slogx.NewError(err, "key1", "value1", "key2", "value2")
		newErrorAttrs   = slogx.NewErrorAttrs(newError, slog.Int("key3", 3), slog.Int("key4", 4))
		wrapedError     = fmt.Errorf("error: %w", err)
		newErrorNoAttrs = errorNoAttrs(wrapedError)
		errorAttrsFunc  = slogx.ErrorAttrs()

		newErrorBadKey       = slogx.NewError(err, "key1")
		newErrorBadKeyAttr   = slog.Any("key", newErrorBadKey)
		attrGroupValueBadKey = slog.Any("", slog.GroupValue(slog.String(badKey, "key1"), slog.Any("key", errorNoAttrs(newErrorBadKey))))

		newErrorAttr       = slogx.NewError(err, slog.Int("key", 3))
		newErrorAttrAttr   = slog.Any("", newErrorAttr)
		attrGroupValueAttr = slog.Any("", slog.GroupValue(slog.Int("key", 3), slog.String("", newErrorAttr.Error())))

		newErrorInt             = slogx.NewError(err, 8)
		newErrorIntAttr         = slog.Any("", newErrorInt)
		attrGroupValueBadKeyInt = slog.Any("", slog.GroupValue(slog.Any(badKey, 8), slog.String("", newErrorInt.Error())))

		strAttr = slog.String("key", "value")
		anyAttr = slog.Any("key", complex(2.2, 2.7))
		errAttr = slog.Any("key", err)

		attrGroupValue    = slog.Any("", slog.GroupValue(slog.Int("key3", 3), slog.Int("key4", 4), slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.String(key, "new error")))
		attrGroupValueKey = slog.Any(key, slog.GroupValue(slog.Int("key3", 3), slog.Int("key4", 4), slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.String(key, "new error")))

		wrapedError1 = fmt.Errorf("error1: %w", err)
		wrapedError2 = fmt.Errorf("error2: %w", newError)
		wrapedError3 = fmt.Errorf("error3: %w", newErrorAttrs)
		value1       = slog.AnyValue(wrapedError1)
		groupValue2  = slog.GroupValue(slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Any(key, errorNoAttrs(wrapedError2)))
		groupValue3  = slog.GroupValue(slog.Int("key3", 3), slog.Int("key4", 4), slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Any(key, errorNoAttrs(wrapedError3)))
	)

	t.DeepEqual(slogx.NewError(nil), nil)
	t.DeepEqual(slogx.NewErrorAttrs(nil), nil)

	t.DeepEqual(slogx.NewError(err).Error(), e)
	t.DeepEqual(errors.Unwrap(errors.Unwrap(newErrorNoAttrs)), wrapedError)

	t.Equal(errorAttrsFunc(nil, newErrorBadKeyAttr).String(), attrGroupValueBadKey.String())
	t.DeepEqual(errorAttrsFunc(nil, newErrorAttrAttr).String(), attrGroupValueAttr.String())
	t.DeepEqual(errorAttrsFunc(nil, newErrorIntAttr).String(), attrGroupValueBadKeyInt.String())

	t.DeepEqual(errorAttrsFunc(nil, strAttr), strAttr)
	t.DeepEqual(errorAttrsFunc(nil, anyAttr), anyAttr)
	t.DeepEqual(errorAttrsFunc(nil, errAttr), errAttr)

	t.Equal(errorAttrsFunc(nil, slog.Any(key, newErrorAttrs)).String(), attrGroupValue.String())
	t.Equal(errorAttrsFunc(group, slog.Any(key, newErrorAttrs)).String(), attrGroupValueKey.String())

	t.DeepEqual(errorAttrsFunc(nil, slog.Any(key, wrapedError1)), slog.Attr{Key: key, Value: value1})
	t.DeepEqual(errorAttrsFunc(nil, slog.Any(key, wrapedError2)), slog.Attr{Key: "", Value: groupValue2})
	t.DeepEqual(errorAttrsFunc(group, slog.Any(key, wrapedError3)), slog.Attr{Key: key, Value: groupValue3})
}

func TestErrorAttrsOptions(tt *testing.T) {
	t := check.T(tt)
	t.Parallel()

	var (
		buf bytes.Buffer

		fGroupAttrs          = slogx.ErrorAttrs(slogx.GroupTopErrorAttrs())
		fInlineAttrs         = slogx.ErrorAttrs(slogx.InlineSubErrorAttrs())
		fGroupAndInlineAttrs = slogx.ErrorAttrs(slogx.GroupTopErrorAttrs(), slogx.InlineSubErrorAttrs())

		err            = errors.New("new error") //nolint:err113 // False positive.
		newError       = slogx.NewError(err, "key1", "value1", "key2", "value2")
		newErrorAttrs  = slogx.NewErrorAttrs(newError, slog.Int("key3", 3), slog.Int("key4", 4))
		errorAttrsAttr = slog.Any("key", newErrorAttrs)
		errWithoutAttr = slog.Any("key", err)

		attrWithKey = slog.Any(
			"key",
			slog.GroupValue(slog.Int("key3", 3), slog.Int("key4", 4), slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Any("key", slogx.NewErrorNoAttrs(err))),
		)
		attrWithoutKey = slog.Any(
			"",
			slog.GroupValue(slog.Int("key3", 3), slog.Int("key4", 4), slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Any("key", slogx.NewErrorNoAttrs(err))),
		)
	)

	t.DeepEqual((fGroupAndInlineAttrs([]string{"g"}, errWithoutAttr)), errWithoutAttr)

	tests := []struct {
		f      func(groups []string, a slog.Attr) slog.Attr
		groups []string
		want   slog.Attr
	}{
		{fGroupAttrs, []string{}, attrWithKey},
		{fGroupAttrs, []string{"g"}, attrWithKey},
		{fInlineAttrs, []string{}, attrWithoutKey},
		{fInlineAttrs, []string{"g"}, attrWithoutKey},
		{fGroupAndInlineAttrs, []string{}, attrWithKey},
		{fGroupAndInlineAttrs, []string{"g"}, attrWithoutKey},
	}

	for _, tc := range tests {
		t.Run("", func(tt *testing.T) {
			t := check.T(tt).MustAll()

			buf.Reset()
			t.Equal((tc.f(tc.groups, errorAttrsAttr)).String(), tc.want.String())
		})
	}
}
