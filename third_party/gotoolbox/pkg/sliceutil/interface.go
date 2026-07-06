package sliceutil

import "reflect"

func IsSliceOfStructs(input any) bool {
	val := reflect.ValueOf(input)

	if val.Kind() != reflect.Slice {
		return false
	}
	elemType := val.Type().Elem()

	return elemType.Kind() == reflect.Struct
}

func ToInterfaceSlice(slice any) []any {
	v := reflect.ValueOf(slice)
	if v.Kind() != reflect.Slice {
		return nil
	}

	result := make([]any, v.Len())
	for i := 0; i < v.Len(); i++ {
		result[i] = v.Index(i).Interface()
	}
	return result
}
