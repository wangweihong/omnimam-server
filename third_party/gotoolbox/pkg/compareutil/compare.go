package compareutil

import (
	"fmt"
	"reflect"
	"strconv"
)

func StructReflectValueCompare(v1, v2 reflect.Value, fieldName string) int {
	if v1.Type().Kind() != reflect.Struct || v2.Type().Kind() != reflect.Struct {
		return 0
	}

	fi, fj := v1.FieldByName(fieldName), v2.FieldByName(fieldName)

	if !fi.IsValid() || !fj.IsValid() {
		return 0
	}

	if fi.Type().Comparable() && fj.Type().Comparable() {
		if fi.Interface() == fj.Interface() {
			return 0
		}
	}

	return ReflectCompare(fi, fj)
}

func ReflectCompare(fi, fj reflect.Value) int {
	if !fi.Type().Comparable() || !fj.Type().Comparable() {
		return 0
	}

	if fi.Interface() == fj.Interface() {
		return 0
	}

	switch fi.Kind() {
	case reflect.String:
		if fi.String() < fj.String() {
			return -1
		}

		if fi.String() > fj.String() {
			return 1
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fi.Int() < fj.Int() {
			return -1
		}

		if fi.Int() > fj.Int() {
			return 1
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if fi.Uint() < fj.Uint() {
			return -1
		}

		if fi.Uint() > fj.Uint() {
			return 1
		}
	case reflect.Float32, reflect.Float64:
		if fi.Float() < fj.Float() {
			return -1
		}

		if fi.Float() > fj.Float() {
			return 1
		}
	default:
		si := fmt.Sprintf("%v", fi.Interface())
		sj := fmt.Sprintf("%v", fj.Interface())

		if si < sj {
			return -1
		}

		if si > sj {
			return 1
		}
	}
	return 0
}

func CompareInterface(a, b any, ascending bool) int {
	aFloat, aErr := toFloat64(a)
	bFloat, bErr := toFloat64(b)
	// 转换成浮点数比较
	if aErr == nil && bErr == nil {
		if ascending {
			return int(aFloat - bFloat)
		}
		return int(bFloat - aFloat)
	}
	// 转换成字符串比较
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	if ascending {
		if aStr < bStr {
			return -1
		} else if aStr > bStr {
			return 1
		}
		return 0
	}

	if aStr > bStr {
		return -1
	} else if aStr < bStr {
		return 1
	}
	return 0
}

func toFloat64(val any) (float64, error) {
	switch v := val.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %v to float64", val)
	}
}

func IsStructReflectValueCanCompare(v1, v2 reflect.Value, fieldName string) (reflect.Value, reflect.Value, bool) {
	if v1.Type().Kind() != reflect.Struct || v2.Type().Kind() != reflect.Struct {
		return reflect.Value{}, reflect.Value{}, false
	}

	fi := v1.FieldByName(fieldName)
	fj := v2.FieldByName(fieldName)

	if !fi.IsValid() || !fj.IsValid() {
		return fi, fj, false
	}

	if !fi.Type().Comparable() || !fj.Type().Comparable() {
		return fi, fj, false
	}

	return fi, fj, true
}

// 确保指针指向的数值限制在指定范围内
//
//	var s Score = 120
//	setLimit(&s, 0, 100)  // s = 100
//
//	var s = -1
//	setLimit(&s, 0, 100) // s = 0
func SetLimit[T interface{ ~int | ~int64 }](limit *T, min, max T) {
	if *limit <= min {
		*limit = min
	}
	if *limit > max {
		*limit = max
	}
}
