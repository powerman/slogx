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

	var (
		e              = "new error"
		key            = "Key"
		group          = []string{"group"}
		err            = errors.New(e) //nolint:goerr113 // False positive. ???
		strAttr        = slog.String("key", "value")
		anyAttr        = slog.Any("key", complex(2.2, 2.7))
		errAttr        = slog.Any("key", err)
		groupValue     = "=\\[key1=value1 key2=value2 key3=3 key4=4 Key=new error\\]"
		groupValueKey  = "Key=\\[key1=value1 key2=value2 key3=3 key4=4 Key=new error\\]"
		newError       = slogx.NewError(err, "key1", "value1", "key2", "value2")
		newErrorAttrs  = slogx.NewErrorAttrs(newError, slog.Int("key3", 3), slog.Int("key4", 4))
		wrapedError1   = fmt.Errorf("error1: %w", err)
		wrapedError2   = fmt.Errorf("error2: %w", newError)
		wrapedError3   = fmt.Errorf("error3: %w", newErrorAttrs)
		errNoAttr1     = slog.Any("key", slogx.NewErrorNoAttrs(wrapedError1))
		errNoAttr2     = slog.Any("key", slogx.NewErrorNoAttrs(wrapedError3))
		value1         = slog.AnyValue(wrapedError1)
		groupValue2    = slog.GroupValue(slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Any(key, slogx.NewErrorNoAttrs(wrapedError2)))
		groupValue3    = slog.GroupValue(slog.Any("key1", "value1"), slog.Any("key2", "value2"), slog.Int("key3", 3), slog.Int("key4", 4), slog.Any(key, slogx.NewErrorNoAttrs(wrapedError3)))
		errorAttrsFunc = slogx.ErrorAttrs()
	)

	t.DeepEqual(slogx.NewError(nil), nil)
	t.DeepEqual(slogx.NewErrorAttrs(nil), nil)
	t.DeepEqual(slogx.NewErrorNoAttrs(nil), nil)

	t.DeepEqual(slogx.NewError(err).Error(), e)
	t.DeepEqual(slogx.NewErrorNoAttrs(err).Error(), e)

	t.DeepEqual(errorAttrsFunc(nil, strAttr), strAttr)
	t.DeepEqual(errorAttrsFunc(nil, anyAttr), anyAttr)
	t.DeepEqual(errorAttrsFunc(nil, errAttr), errAttr)

	t.DeepEqual(errorAttrsFunc(nil, errNoAttr1), errNoAttr1)
	t.DeepEqual(errorAttrsFunc(nil, errNoAttr2), errNoAttr2)

	t.Match(errorAttrsFunc(nil, slog.Any(key, newErrorAttrs)), groupValue)
	t.Match(errorAttrsFunc(group, slog.Any(key, newErrorAttrs)), groupValueKey)

	t.DeepEqual(errorAttrsFunc(nil, slog.Any(key, wrapedError1)), slog.Attr{Key: key, Value: value1})
	t.DeepEqual(errorAttrsFunc(nil, slog.Any(key, wrapedError2)), slog.Attr{Key: "", Value: groupValue2})
	t.DeepEqual(errorAttrsFunc(group, slog.Any(key, wrapedError3)), slog.Attr{Key: key, Value: groupValue3})
}
