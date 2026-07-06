package reflectutil

import (
	"reflect"
)

// 创建指定反射值的指针
func CreatePointerToValue(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr {
		return v
	}

	ptr := reflect.New(v.Type())
	ptr.Elem().Set(v)
	return ptr
}
