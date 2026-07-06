package mathutil

import (
	"fmt"
	"strconv"

	"github.com/wangweihong/gotoolbox/pkg/generic"
)

func ParseInt(size string) (int, error) {
	if size == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(size, 10, 32)

	return int(value), err
}

func ParseInt64(size string) (int64, error) {
	if size == "" {
		return 0, nil
	}

	return strconv.ParseInt(size, 10, 64)
}

// ParseNumExplicitType 解析字符串为对应类型数据 ,必须在调用时明确类型变量
func ParseNumExplicitType[T generic.Int](d string) (T, error) {
	var zero T
	return ParseNum(zero, d)
}

func MustParseNumExplicitType[T generic.Int](d string) T {
	r, _ := ParseNumExplicitType[T](d)
	return r
}

// ParseNum 由传入参数决定解析的类型
func ParseNum[T generic.Int](a T, d string) (T, error) {
	var zero T

	if d == "" {
		return zero, nil
	}

	// Select the bit size based on the type of T.
	switch any(zero).(type) {
	case int32, int:
		value, err := strconv.ParseInt(d, 10, 32)
		if err != nil {
			return zero, err
		}
		return T(value), nil
	case int64:
		value, err := strconv.ParseInt(d, 10, 64)
		if err != nil {
			return zero, err
		}
		return T(value), nil
	case uint32, uint:
		value, err := strconv.ParseUint(d, 10, 32)
		if err != nil {
			return zero, err
		}
		return T(value), nil
	case uint64:
		value, err := strconv.ParseUint(d, 10, 64)
		if err != nil {
			return zero, err
		}
		return T(value), nil
	}
	return zero, fmt.Errorf("unsupported type: %T", zero)
}

func MustParseNum[T generic.Int](a T, d string) T {
	r, _ := ParseNum(a, d)
	return r
}
