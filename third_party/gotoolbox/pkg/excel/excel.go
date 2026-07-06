package excel

import (
	"context"
	"os"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/fieldutil"

	//"github.com/wangweihong/gotoolbox/pkg/fieldutil"
	"github.com/wangweihong/gotoolbox/pkg/log"
)

func ImportFromFile[T any](ctx context.Context, pr *importParserRegistry, sheet string, fileName string, failFields ...string) ([]T, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer f.Close()

	xlsx, err := excelize.OpenReader(f)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return Import[T](ctx, pr, xlsx, sheet, failFields...)
}

// pr: 解析器注册表。用于注册字段的的解析。举个例子slice类型的字段,有些需要通过","分隔,有些通过"|"分隔,每种结构体的要求均可能不同。
//
//	因此提供解析器供调用者自行根据需求自定义解析器
//
// failFields: 当某些关键字段解析失败, 直接失败而非设置零值
func Import[T any](ctx context.Context, pr *importParserRegistry, xlsx *excelize.File, sheet string, failFields ...string) ([]T, error) {
	// 获取指定工作表中所有行
	rows := xlsx.GetRows(sheet)
	if len(rows) == 0 {
		return nil, errors.Errorf("no rows in sheel %v", sheet)
	}

	var zero T
	fieldTagMap := GenerateFieldTagRowMap(zero, ExcelTagName)
	titleRow := FindFieldColAndTitleRow(fieldTagMap, rows)
	for k, v := range fieldTagMap {
		if v.Col == -1 {
			log.Infof("field tag %v/%v without matching row", k, v.Tag)
		}
	}

	datas := make([]T, 0, len(rows))
	for i, row := range rows {
		// 跳过标题行
		if i == titleRow {
			continue
		}

		var zero T
		if err := SetDataFieldValue(fieldTagMap, row, &zero, pr, failFields...); err != nil {
			log.Debugf("set row[%d] data value err:%v", i, err)
			continue
		}
		datas = append(datas, zero)
	}

	return datas, nil
}

func Export[T any](ctx context.Context, pr *ExportRegistry, f *excelize.File, sheet string, rawData []T, HideField ...string) error {
	if f == nil {
		return errors.Errorf("missing xlsx")
	}
	index := f.NewSheet(sheet)

	// 从结构体中提取excel tag的值作为标题
	var zero T
	excelTags := fieldutil.ParseStructFieldTags(zero, ExcelTagName, HideField...)
	headers := excelTags.Tags()

	if err := SetExcelHeader(f, sheet, headers); err != nil {
		return errors.WithStack(err)
	}

	for rowIdx, d := range rawData {
		if err := SetExcelDataFromObject(f, pr, sheet, headers, rowIdx, d, HideField...); err != nil {
			return errors.WithStack(err)
		}
	}

	f.SetActiveSheet(index)
	return nil
}

func ExportToFile[T any](ctx context.Context, pr *ExportRegistry, fileName string, rawData []T, sheet string, HideField ...string) error {
	excelFile := excelize.NewFile()

	if err := Export[T](ctx, pr, excelFile, sheet, rawData, HideField...); err != nil {
		return errors.WithStack(err)
	}

	if err := excelFile.SaveAs(fileName); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
