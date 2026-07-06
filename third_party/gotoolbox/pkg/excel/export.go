package excel

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

// ExportRegistry 导出注册中心
type ExportRegistry struct {
	defaultExporters map[reflect.Kind]ExporterFunc
	fieldExporters   map[string]ExporterFunc // Key: "StructName.FieldName"
	typeExporters    map[reflect.Type]ExporterFunc
	mu               sync.RWMutex
}
type ExporterFunc func(interface{}) (string, error)

func NewExportRegistry() *ExportRegistry {
	registry := &ExportRegistry{
		defaultExporters: make(map[reflect.Kind]ExporterFunc),
		fieldExporters:   make(map[string]ExporterFunc),
		typeExporters:    make(map[reflect.Type]ExporterFunc),
	}
	registry.initDefaultExporters()
	return registry
}

func (r *ExportRegistry) initDefaultExporters() {
	// 注册默认导出器
	r.RegisterDefault(reflect.String, func(value interface{}) (string, error) {
		return value.(string), nil
	})

	r.RegisterDefault(reflect.Int, func(value interface{}) (string, error) {
		return strconv.Itoa(value.(int)), nil
	})

	r.RegisterDefault(reflect.Int64, func(value interface{}) (string, error) {
		return strconv.FormatInt(value.(int64), 10), nil
	})

	r.RegisterDefault(reflect.Uint, func(value interface{}) (string, error) {
		return strconv.FormatUint(uint64(value.(uint)), 10), nil
	})

	r.RegisterDefault(reflect.Uint64, func(value interface{}) (string, error) {
		return strconv.FormatUint(value.(uint64), 10), nil
	})

	r.RegisterDefault(reflect.Bool, func(value interface{}) (string, error) {
		if value.(bool) {
			return "是", nil
		}
		return "否", nil
	})

	r.RegisterDefault(reflect.Slice, func(value interface{}) (string, error) {
		sliceVal := reflect.ValueOf(value)
		if sliceVal.Kind() != reflect.Slice {
			return "", errors.Errorf("expected slice, got %s", sliceVal.Kind())
		}

		var builder strings.Builder
		for i := 0; i < sliceVal.Len(); i++ {
			elem := sliceVal.Index(i).Interface()
			elemStr, err := r.ExportValue(elem)
			if err != nil {
				return "", err
			}

			if i > 0 {
				builder.WriteString(",")
			}
			builder.WriteString(elemStr)
		}
		return builder.String(), nil
	})

	// 时间类型导出器
	r.RegisterTypeExporter(reflect.TypeOf(time.Time{}), func(value interface{}) (string, error) {
		t := value.(time.Time)
		return t.Format("2006-01-02"), nil
	})
}

// 注册默认导出器（按类型）
func (r *ExportRegistry) RegisterDefault(kind reflect.Kind, exporter ExporterFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultExporters[kind] = exporter
}

// 注册类型导出器
func (r *ExportRegistry) RegisterTypeExporter(typ reflect.Type, exporter ExporterFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typeExporters[typ] = exporter
}

// 注册字段导出器
func (r *ExportRegistry) RegisterFieldExporter(structType reflect.Type, fieldName string, exporter ExporterFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s.%s", structType.Name(), fieldName)
	r.fieldExporters[key] = exporter
}

// 导出值
func (r *ExportRegistry) ExportValue(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}

	typ := reflect.TypeOf(value)
	exporter := r.GetExporter(typ, "", typ)
	return exporter(value)
}

// 获取导出器
func (r *ExportRegistry) GetExporter(structType reflect.Type, fieldName string, fieldType reflect.Type) ExporterFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// 1. 优先检查字段专属导出器
	if structType != nil && fieldName != "" {
		key := fmt.Sprintf("%s.%s", structType.Name(), fieldName)
		if exporter, ok := r.fieldExporters[key]; ok {
			return exporter
		}
	}
	// 2. 检查类型导出器
	if exporter, ok := r.typeExporters[fieldType]; ok {
		return exporter
	}
	// 3. 使用默认类型导出器
	if exporter, ok := r.defaultExporters[fieldType.Kind()]; ok {
		return exporter
	}
	// 4. 最终回没有匹配的导出器，返回 nil
	return nil
}

// 通用导出器
func genericExporter(value interface{}) (string, error) {
	// 尝试使用JSON格式化
	jsonBytes, err := json.Marshal(value)
	if err == nil {
		return string(jsonBytes), nil
	}

	// 尝试字符串转换
	return fmt.Sprintf("%v", value), nil
}

// GetField 获取结构体字段的字符串表示
func GetField(s interface{}, fieldName string, registry *ExportRegistry) (string, error) {
	if s == nil {
		return "", nil
	}
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		return "", errors.Errorf("field %s not found", fieldName)
	}

	fieldType := field.Type()
	structType := val.Type()
	exporter := registry.GetExporter(structType, fieldName, fieldType)
	return exporter(field.Interface())
}

// ExportStruct 导出整个结构体为字符串映射
func ExportStruct(s interface{}, registry *ExportRegistry) (map[string]string, error) {
	if s == nil {
		return nil, nil
	}
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	typ := val.Type()
	result := make(map[string]string)

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// 跳过非导出字段
		if !field.IsExported() {
			continue
		}

		// 获取字段名
		fieldName := field.Name

		// 获取导出器
		exporter := registry.GetExporter(typ, fieldName, field.Type)

		// 导出值
		strValue, err := exporter(fieldValue.Interface())
		if err != nil {
			return nil, errors.Errorf("failed to export field %s: %v", fieldName, err)
		}

		result[fieldName] = strValue
	}

	return result, nil
}
