package sortutil

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/compareutil"

	"github.com/wangweihong/gotoolbox/pkg/structutil"

	"github.com/wangweihong/gotoolbox/pkg/fieldutil"

	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
)

func FieldTagSort(si any, tag string, asc bool, defaultComparer, condition func(i, j int) bool) error {
	if !sliceutil.IsSliceOfStructs(si) {
		return fmt.Errorf("no slice or elem no struct")
	}

	sl := sliceutil.ToInterfaceSlice(si)
	if len(sl) == 0 {
		return nil
	}

	tagMap := fieldutil.GetFieldTagMapping(structutil.InitializeStruct(sl[0]))
	field, ok := tagMap[tag]
	if ok {
		fmt.Println("find tag:", tag)
		sort.SliceStable(sl, func(i, j int) bool {
			ret := 0

			vi, vj := reflect.ValueOf(sl[i]), reflect.ValueOf(sl[j])
			fi, fj, canCompare := compareutil.IsStructReflectValueCanCompare(vi, vj, field)
			if canCompare {
				ret = compareutil.ReflectCompare(fi, fj)
			}
			fmt.Printf("canCompare:%v, ret:%v\n", canCompare, ret)

			var result bool
			switch ret {
			case 1:
				result = true
			case -1:
				result = false
			default:
				result = defaultComparer(i, j)
			}

			if asc {
				return !result
			} else {
				return result
			}
		})
	} else {
		sort.SliceStable(sl, defaultComparer)
	}
	return nil
}

// 对指定的struct slice进行排序，通过"field.JsonTag.FieldName"或者"field/JsonTag/FieldName"来指定排序的字段,
// target允许传递多个过滤字段, 通过空格切分
// 如果排序字段是map, 则通过'mapFieldOrTag.key'或者"mapFieldOrTag/key"
func StructSliceSort[T any](slice []T, target string, sortAsc bool) {
	if target == "" {
		return
	}

	//过滤字段允许传多个
	targets := strings.Split(target, " ")
	// 去重
	targets = sliceutil.Unique(targets)
	for _, v := range targets {
		sort.SliceStable(slice, GetSortComparator(slice, v, sortAsc))
	}
}

func GetSortComparator[T any](slice []T, target string, sortAsc bool) func(int, int) bool {
	def := func(i int, j int) bool { return false }

	if len(slice) <= 1 {
		return def
	}

	var fields []string
	var ss []string
	if strings.LastIndex(target, "/") > 0 {
		ss = strings.Split(target, "/")
	} else {
		ss = strings.Split(target, ".")
	}
	for _, v := range ss {
		field := strings.Trim(v, " ")
		if field != "" {
			fields = append(fields, field)
		}
	}

	ret := func(i int, j int) bool {
		elemi := toBaseType(reflect.ValueOf(slice[i]), fields)
		elemj := toBaseType(reflect.ValueOf(slice[j]), fields)
		if elemi.Kind() != elemj.Kind() {
			return false
		}
		if sortAsc {
			switch elemi.Kind() {
			case reflect.String:
				return elemi.String() < elemj.String()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return elemi.Int() < elemj.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return elemi.Uint() < elemj.Uint()
			case reflect.Float32, reflect.Float64:
				return elemi.Float() < elemj.Float()
			}
		} else {
			switch elemi.Kind() {
			case reflect.String:
				return elemi.String() > elemj.String()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return elemi.Int() > elemj.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return elemi.Uint() > elemj.Uint()
			case reflect.Float32, reflect.Float64:
				return elemi.Float() > elemj.Float()
			}
		}
		return false
	}

	return ret
}

func GetSortComparator2(slice any, target string, sortAsc bool) func(int, int) bool {
	def := func(i int, j int) bool { return false }

	if slice == nil {
		return nil
	}
	valueOf := reflect.ValueOf(slice)
	if valueOf.Kind() != reflect.Slice {
		return def
	}

	if valueOf.Len() <= 1 {
		return def
	}

	var fields []string
	var ss []string
	if strings.LastIndex(target, "/") > 0 {
		ss = strings.Split(target, "/")
	} else {
		ss = strings.Split(target, ".")
	}
	for _, v := range ss {
		field := strings.Trim(v, " ")
		if field != "" {
			fields = append(fields, field)
		}
	}

	ret := func(i int, j int) bool {
		elemi := toBaseType(valueOf.Index(i), fields)
		elemj := toBaseType(valueOf.Index(j), fields)
		if elemi.Kind() != elemj.Kind() {
			return false
		}
		if sortAsc {
			switch elemi.Kind() {
			case reflect.String:
				return elemi.String() < elemj.String()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return elemi.Int() < elemj.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return elemi.Uint() < elemj.Uint()
			case reflect.Float32, reflect.Float64:
				return elemi.Float() < elemj.Float()
			}
		} else {
			switch elemi.Kind() {
			case reflect.String:
				return elemi.String() > elemj.String()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				return elemi.Int() > elemj.Int()
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				return elemi.Uint() > elemj.Uint()
			case reflect.Float32, reflect.Float64:
				return elemi.Float() > elemj.Float()
			}
		}
		return false
	}

	return ret
}

// 沿着给定的字段路径逐级深入访问嵌套数据结构，最终返回路径终点的值
func toBaseType(value reflect.Value, fields []string) reflect.Value {
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return reflect.Value{}
	}
	if value.Kind() == reflect.Ptr {
		value = reflect.Indirect(value)
	}
	if len(fields) == 0 {
		return value
	}

	switch value.Kind() {
	case reflect.Struct:
		value = getFieldByJSONTagOrName(value, fields[0])
	case reflect.Map:
		value = value.MapIndex(reflect.ValueOf(fields[0]))
	}

	if value.Kind() != reflect.Struct && value.Kind() != reflect.Ptr && value.Kind() != reflect.Map {
		return value
	}

	return toBaseType(value, fields[1:])
}

func getFieldByJSONTagOrName(v reflect.Value, fieldName string) reflect.Value {
	t := v.Type()
	// 第一次遍历：查找匹配的JSON tag
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if tagVal, ok := sf.Tag.Lookup("json"); ok {
			jsonName := strings.Split(tagVal, ",")[0] // 忽略tag选项
			if jsonName == "-" {
				continue // 跳过明确忽略的字段
			}
			if jsonName == fieldName {
				return v.Field(i)
			}
		}
	}

	// 第二次遍历：查找匹配的字段名
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.Name == fieldName {
			return v.Field(i)
		}
	}

	return reflect.Value{} // 未找到字段
}
