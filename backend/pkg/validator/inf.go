package validator

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/validation"
)

func ValidateList(vs ...validation.Validator) error {
	var errs []error
	for _, v := range vs {
		if err := v.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.NewAggregate(errs...)
	}
	return nil
}

// ValidateAll 先基于结构体字段进行binding tag的go-validator检测, 如果字段实现了Validator, 再次进行验证
func ValidateAll(s interface{}) error {
	cval := NewCustomValidator(validation.LangEN)
	cval.Engine()
	visited := &sync.Map{}
	// 先进行Tag检测，注意这里不会递归，除非设置"dive"
	if err := cval.Validate(s); err != nil {
		return err
	}

	return validateCustom(s, visited)
}

// 遍历结构体所有的字段, 凡是实现了validator.Validate,均用来调用检测
func validateCustom(s interface{}, visited *sync.Map) error {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	return validateRecursive(v, visited)
}

func validateRecursive(v reflect.Value, visited *sync.Map) error {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		ptr := v.Pointer()
		// 避免指向同个对象, 陷入死循环
		if _, loaded := visited.LoadOrStore(ptr, struct{}{}); loaded {
			return nil // 已访问过，跳过
		}

		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		// 跳过不可导出字段
		if !field.CanInterface() {
			continue
		}
		// 优先检查并执行自定义验证
		if validator, ok := getCustomValidator(field); ok {
			if err := validator.Validate(); err != nil {
				return fmt.Errorf("%s: %w", fieldType.Name, err)
			}
			continue // 跳过递归
		}

		switch field.Kind() {
		case reflect.Struct:
			if err := validateRecursive(field, visited); err != nil {
				return err
			}
		case reflect.Ptr:
			if field.Type().Elem().Kind() == reflect.Struct {
				if err := validateRecursive(field, visited); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func getCustomValidator(field reflect.Value) (validation.Validator, bool) {
	if validator, ok := field.Interface().(validation.Validator); ok {
		return validator, true
	}
	if field.CanAddr() {
		if validator, ok := field.Addr().Interface().(validation.Validator); ok {
			return validator, true
		}
	}
	return nil, false
}
