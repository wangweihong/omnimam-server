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

// 解析器函数, 将字段值转换成对应的类型
type ParserFunc func(fieldValue string, fieldType reflect.Type) (interface{}, error)

// 用于注册字段相应的解析
type importParserRegistry struct {
	//默认解析器
	defaultParsers map[reflect.Kind]ParserFunc
	//结构体字段解析器
	fieldParsers map[string]ParserFunc // Key: "StructName.FieldName"
	//类型解析器
	typeParsers map[reflect.Type]ParserFunc
	mu          sync.RWMutex
}

func NewimportParserRegistry() *importParserRegistry {
	return &importParserRegistry{
		defaultParsers: make(map[reflect.Kind]ParserFunc),
		fieldParsers:   make(map[string]ParserFunc),
		typeParsers:    make(map[reflect.Type]ParserFunc),
	}
}

func (r *importParserRegistry) initDefaultParsers() {
	// 注册默认解析器
	r.RegisterDefault(reflect.String, func(value string, _ reflect.Type) (interface{}, error) {
		return value, nil
	})

	r.RegisterDefault(reflect.Int, func(value string, _ reflect.Type) (interface{}, error) {
		if value == "" {
			return 0, nil
		}
		return strconv.Atoi(value)
	})

	r.RegisterDefault(reflect.Int64, func(value string, _ reflect.Type) (interface{}, error) {
		if value == "" {
			return int64(0), nil
		}
		return strconv.ParseInt(value, 10, 64)
	})

	r.RegisterDefault(reflect.Uint, func(value string, _ reflect.Type) (interface{}, error) {
		if value == "" {
			return uint(0), nil
		}
		i, err := strconv.ParseUint(value, 10, 64)
		return uint(i), err
	})

	r.RegisterDefault(reflect.Uint64, func(value string, _ reflect.Type) (interface{}, error) {
		if value == "" {
			return uint64(0), nil
		}
		return strconv.ParseUint(value, 10, 64)
	})

	r.RegisterDefault(reflect.Bool, parseBool)

	r.RegisterDefault(reflect.Slice, func(value string, fieldType reflect.Type) (interface{}, error) {
		if strings.TrimSpace(value) == "" || value == "<nil>" {
			return reflect.Zero(fieldType).Interface(), nil
		}

		// 获取切片元素类型
		elemType := fieldType.Elem()
		values := strings.Split(value, ",")
		slice := reflect.MakeSlice(fieldType, len(values), len(values))

		// 递归解析每个元素
		for i, v := range values {
			elemValue, err := r.ParseValue(v, elemType)
			if err != nil {
				return nil, err
			}
			slice.Index(i).Set(reflect.ValueOf(elemValue))
		}

		return slice.Interface(), nil
	})

	// 时间类型解析器
	r.RegisterTypeParser(reflect.TypeOf(time.Time{}), ParseTime)
}

// 注册默认反射类型解析器
func (r *importParserRegistry) RegisterDefault(kind reflect.Kind, parser ParserFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultParsers[kind] = parser
}

// 按字段注册 (结构体名.字段名)
func (r *importParserRegistry) RegisterFieldParser(structName, fieldName string, parser ParserFunc) {
	key := fmt.Sprintf("%s.%s", structName, fieldName)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fieldParsers[key] = parser
}

func (r *importParserRegistry) RegisterTypeParser(typ reflect.Type, parser ParserFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typeParsers[typ] = parser
}

// 通用值解析方法
func (r *importParserRegistry) ParseValue(value string, typ reflect.Type) (interface{}, error) {
	parser := r.GetParser(nil, "", typ)
	return parser(value, typ)
}

// 获取解析器
func (r *importParserRegistry) GetParser(structType reflect.Type, fieldName string, fieldType reflect.Type) ParserFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// 1. 优先检查字段专属解析器
	if structType != nil && fieldName != "" {
		key := fmt.Sprintf("%s.%s", structType.Name(), fieldName)
		if parser, ok := r.fieldParsers[key]; ok {
			return parser
		}
	}
	// 2. 检查类型解析器
	if parser, ok := r.typeParsers[fieldType]; ok {
		return parser
	}
	// 3. 使用默认类型解析器
	if parser, ok := r.defaultParsers[fieldType.Kind()]; ok {
		return parser
	}
	// 4. 最终回退到通用解析
	return genericParser
}

// 通用解析器
func genericParser(value string, fieldType reflect.Type) (interface{}, error) {
	if fieldType.Kind() == reflect.String {
		return value, nil
	}
	result := reflect.New(fieldType).Interface()
	if err := json.Unmarshal([]byte(value), result); err == nil {
		return reflect.ValueOf(result).Elem().Interface(), nil
	}

	return nil, errors.Errorf("no parser found for type %s", fieldType)
}

func parseBool(value string, _ reflect.Type) (interface{}, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "t", "1", "是", "yes", "y":
		return true, nil
	case "false", "f", "0", "否", "no", "n", "":
		return false, nil
	default:
		return nil, fmt.Errorf("invalid bool value: %s", value)
	}
}

func ParseTime(str string, _ reflect.Type) (interface{}, error) {
	str = strings.TrimSpace(str)
	excelSerial, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return time.Time{}, errors.WithStack(err)
	}

	// Excel 日期从 1900 年 1 月 1 日开始计算
	excelStartDate := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

	// 计算日期偏移（天数）和时间偏移（纳秒）
	days := int(excelSerial)
	// 计算时间偏移，乘以一天的纳秒数（24小时 * 60分钟 * 60秒 * 1e9纳秒）
	nanos := int64((excelSerial - float64(days)) * 24 * 60 * 60 * 1e9)

	// 计算 Excel 日期起始日期开始的日期时间
	excelDateTime := excelStartDate.AddDate(0, 0, days).Add(time.Duration(nanos))
	return excelDateTime, nil
}
