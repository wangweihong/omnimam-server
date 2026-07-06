package structutil

import "reflect"

// InitializeStruct 创建一个同类型的结构, 用于提取结构标签,字段名等功能
func InitializeStruct(ptr any) any {
	val := reflect.ValueOf(ptr)
	if val.Kind() == reflect.Ptr {
		structType := val.Type().Elem()
		if structType.Kind() != reflect.Struct {
			return nil
		}
		newStruct := reflect.New(structType)
		return newStruct.Elem().Interface()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}
	newStruct := reflect.New(reflect.TypeOf(ptr))
	return newStruct.Elem().Interface()
}
