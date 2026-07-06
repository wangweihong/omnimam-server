package excel

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/sets"
)

var (
	ExcelTagName = "excel"
)

// 用于记录Excel表某个标题对应的行号
type ExcelTag struct {
	// 标签名
	Tag string
	// 在Excel表的列号
	Col int
}

// generateFieldTagRowMap 生成结构体字段和excel tag的列映射关系
// key为字段名
func GenerateFieldTagRowMap(s interface{}, tagName string) map[string]ExcelTag {
	fieldMap := make(map[string]ExcelTag)
	if tagName != "" {
		val := reflect.ValueOf(s)
		typ := reflect.TypeOf(s)
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)
			tag := field.Tag.Get(tagName)
			if tag != "" {
				// 设置为-1, 避免和row[0]混淆
				fieldMap[field.Name] = ExcelTag{Tag: tag, Col: -1}
			}
		}
	}
	return fieldMap
}

// 找到结构体字段在excel表中对应的列号以及标题栏
func FindFieldColAndTitleRow(fieldTagMap map[string]ExcelTag, rows [][]string) /*titleRow*/ int {
	//根据标题行每列的值来查找结构体对应字段在哪一行
	var titleRow = -1 // 记录标题行的行号, 后续忽略掉
	for i, row := range rows {
		for j, cell := range row {
			for k, field := range fieldTagMap {
				if cell == field.Tag {
					field.Col = j
					fieldTagMap[k] = field
					titleRow = i
				}
			}
		}

		if titleRow != -1 {
			break
		}
	}
	return titleRow
}

func SetDataFieldValue(fieldTagMap map[string]ExcelTag, row []string, data interface{}, registry *importParserRegistry, failFields ...string) error {
	for fieldName, field := range fieldTagMap {
		// 忽略未找到的字段
		if field.Col == -1 {
			continue
		}
		if err := SetField(data, fieldName, row[field.Col], registry, failFields...); err != nil {
			return err
		}
	}
	return nil
}

func SetField(s interface{}, fieldName, value string, registry *importParserRegistry, failFields ...string) error {
	if s == nil {
		return nil
	}
	val := reflect.ValueOf(s).Elem()
	failFieldsSet := sets.NewString(failFields...)
	field := val.FieldByName(fieldName)
	if !field.IsValid() || !field.CanSet() {
		return fmt.Errorf("field %s is invalid or cannot be set", fieldName)
	}
	fieldType := field.Type()
	structType := val.Type()

	parser := registry.GetParser(structType, fieldName, fieldType)

	parsedValue, err := parser(value, fieldType)
	if err != nil {
		if failFieldsSet.Has(fieldName) {
			return errors.Errorf("field %v parse value '%v' failed: %v", fieldName, value, err)
		}
		// 对于非关键字段，尝试使用零值
		if field.Kind() != reflect.Ptr {
			field.Set(reflect.Zero(fieldType))
		}
		return nil
	}
	// 设置值
	parsedVal := reflect.ValueOf(parsedValue)
	if parsedVal.Type().AssignableTo(fieldType) {
		field.Set(parsedVal)
	} else if parsedVal.Type().ConvertibleTo(fieldType) {
		field.Set(parsedVal.Convert(fieldType))
	} else {
		return errors.Errorf("parsed value type %s is not assignable to field type %s",
			parsedVal.Type(), fieldType)
	}
	return nil
}

// 设置Excel表标题
func SetExcelHeader(f *excelize.File, category string, headers []string) error {
	// 设置标题头颜色
	// 白色字体,加粗,背景为红色
	style, err := f.NewStyle(`{"font":{"bold":true,"color":"#FFFFFF"},"fill":{"type":"pattern","color":["#FF0000"],"pattern":1}}`)
	if err != nil {
		return err
	}

	for colIdx, colName := range headers {
		//相当于A1, B1, C1
		cell := excelize.ToAlphaString(colIdx) + "1"
		f.SetCellValue(category, cell, colName)
		f.SetCellStyle(category, cell, cell, style)
	}
	return nil
}

// SetExcelDataFromObject 根据标题头的顺序，依次将data对象的字段的数据插入到excel表中
func SetExcelDataFromObject(f *excelize.File, registry *ExportRegistry, category string, headers []string, rowIdx int, data interface{}, hideFields ...string) error {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	t := val.Type()

	for colIdx := 0; colIdx < len(headers); colIdx++ {

		//以A2为偏移量开始插入数据
		cell := excelize.ToAlphaString(colIdx) + strconv.Itoa(rowIdx+2)
		fieldVal := val.Field(colIdx)
		if !fieldVal.IsValid() || !fieldVal.CanInterface() {
			continue
		}
		field := t.Field(colIdx)
		fieldName := field.Name

		// f.SetCellValue本身就具有处理val type的能力
		// 因此如果字段/类型没有注册到Exporter Registry, 则默认使用f.SetCellValue的能力
		exporter := registry.GetExporter(t, fieldName, field.Type)
		if exporter != nil {
			valueStr, err := exporter(fieldVal.Interface())
			if err != nil {
				// 对于导出失败的情况，记录错误但继续处理其他字段
				log.Debugf("error exporting field %s: %v\n", fieldName, err)
				valueStr = "" // 使用空字符串作为回退
			}

			f.SetCellValue(category, cell, valueStr)
		} else {
			val := fieldVal.Interface()
			f.SetCellValue(category, cell, val)
		}
	}
	return nil
}
