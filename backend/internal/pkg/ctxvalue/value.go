package ctxvalue

import (
	"context"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

func GetValue[T any](ctx context.Context, key interface{}) (T, error) {
	val := ctx.Value(key)
	if val == nil {
		var zero T
		return zero, errors.Errorf("key %v not found in context", key)
	}
	result, ok := val.(T)
	if !ok {
		var zero T
		return zero, errors.Errorf(
			"type assertion failed for key %v: expected %T, got %T",
			key, zero, val,
		)
	}
	return result, nil
}
