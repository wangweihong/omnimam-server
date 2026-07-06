package fieldutil

import (
	"reflect"
)

const (
	JsonTag = "json"
)

type Fields []reflect.StructField

// ParseStructFields 从结构体或者结构体指针(指针非空值)中获取其字段信息
func ParseStructFields(s any) Fields {
	if s == nil {
		return nil
	}

	typ := reflect.TypeOf(s)
	vyp := reflect.ValueOf(s)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		vyp = vyp.Elem()
		if !vyp.IsValid() {
			return nil
		}
	}

	if typ.Kind() != reflect.Struct {
		return nil
	}

	if typ.NumField() != 0 {
		fts := make([]reflect.StructField, 0, typ.NumField())
		// 这里不处理匿名字段，应由字段本身去根据类型去处理
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			//	vfield := vyp.Field(i)
			fts = append(fts, field)
		}
		return fts
	}

	return nil
}

func (fs Fields) ToMap() map[string]reflect.StructField {
	ftm := make(map[string]reflect.StructField, len(fs))
	for _, v := range fs {
		ftm[v.Name] = v
	}
	return ftm
}

type FieldValue struct {
	Index int
	T     reflect.StructField
	V     reflect.Value
}

type FieldValues []FieldValue

// ParseStructFieldValues 从结构体或者结构体指针(指针非空值)中获取其字段信息和值
func ParseStructFieldValues(s any) FieldValues {
	if s == nil {
		return nil
	}

	typ := reflect.TypeOf(s)
	vyp := reflect.ValueOf(s)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		vyp = vyp.Elem()
		if !vyp.IsValid() {
			return nil
		}
	}

	if typ.Kind() != reflect.Struct {
		return nil
	}

	if typ.NumField() != 0 {
		fts := make([]FieldValue, 0, typ.NumField())
		// 这里不处理匿名字段，应由字段本身去根据类型去处理
		for i := 0; i < typ.NumField(); i++ {
			ft := typ.Field(i)
			fv := vyp.Field(i)

			fts = append(fts, FieldValue{
				Index: i,
				T:     ft,
				V:     fv,
			})
		}
		return fts
	}

	return nil
}

func (fs FieldValues) ToMap() map[string]FieldValue {
	ftm := make(map[string]FieldValue, len(fs))
	for _, v := range fs {
		ftm[v.T.Name] = v
	}
	return ftm
}

func (fs FieldValues) ExportedList() FieldValues {
	ftm := make([]FieldValue, len(fs))
	for _, v := range fs {
		if v.T.IsExported() {
			ftm = append(ftm, v)
		}
	}
	return ftm
}

func (fs FieldValues) FieldByName(fieldName string, opts ...FieldOption) *FieldValue {
	var fo fieldOption
	for _, o := range opts {
		o(&fo)
	}

	for _, v := range fs {
		if fo.export && !v.T.IsExported() {
			continue
		}

		if v.T.Name == fieldName {
			return &v
		}

		// 解析匿名结构体
		if fo.iterate {
			if v.T.Anonymous {
				elem := v.V
				if v.T.Type.Kind() == reflect.Ptr {
					elem = v.V.Elem()
				}

				if elem.Kind() != reflect.Struct {
					continue
				}

				ffs := ParseStructFieldValues(elem.Interface())
				if ffs != nil {
					ffv := ffs.FieldByName(fieldName)
					if ffv != nil {
						// 嵌套结构的索引不能使用
						ffv.Index = -1
						return ffv
					}
				}
			}
		}
	}

	return nil
}

type fieldOption struct {
	// 递归匿名结构体
	iterate bool
	// 只返回导出字段
	export bool
}

type FieldOption func(*fieldOption)

func WithIterate() FieldOption {
	return func(c *fieldOption) {
		c.iterate = true
	}
}

func WithExport() FieldOption {
	return func(c *fieldOption) {
		c.export = true
	}
}
