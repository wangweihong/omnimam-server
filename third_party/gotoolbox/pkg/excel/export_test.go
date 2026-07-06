package excel_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	. "github.com/smartystreets/goconvey/convey"
	excel "github.com/wangweihong/gotoolbox/pkg/excel"
	"github.com/wangweihong/gotoolbox/pkg/timeutil"
)

type Status int

const (
	Pending Status = iota
	Approved
	Rejected
)

func TestExportStruct(t *testing.T) {
	Convey("TestExportStruct", t, func() {
		exportRegistry := excel.NewExportRegistry()

		// 注册枚举类型导出器
		exportRegistry.RegisterTypeExporter(
			reflect.TypeOf(Status(0)),
			func(value interface{}) (string, error) {
				status := value.(Status)
				switch status {
				case Pending:
					return "待处理", nil
				case Approved:
					return "已批准", nil
				case Rejected:
					return "已拒绝", nil
				default:
					return "未知状态", nil
				}
			},
		)

		// 注册货币类型导出器
		exportRegistry.RegisterTypeExporter(
			reflect.TypeOf(decimal.Decimal{}),
			func(value interface{}) (string, error) {
				d := value.(decimal.Decimal)
				return d.StringFixed(2), nil
			},
		)
		// 注册复杂结构体导出
		type Order struct {
			ID     string
			Amount decimal.Decimal
			Status Status
			Time   time.Time
		}
		exportRegistry.RegisterFieldExporter(
			reflect.TypeOf(Order{}),
			"Time",
			func(value interface{}) (string, error) {
				t := value.(time.Time)
				return t.Format("2006-01-02 15:04:05"), nil
			},
		)

		order := Order{
			ID:     "ORD-001",
			Amount: decimal.NewFromFloat(123.456),
			Status: Approved,
			Time:   timeutil.MustParseTime("\"2022-01-30T12:34:56Z\""),
		}

		exported, err := excel.ExportStruct(&order, exportRegistry)
		So(err, ShouldBeNil)

		So(exported["Status"], ShouldEqual, "已批准")
		So(exported["Amount"], ShouldEqual, "123.46")
		So(exported["Time"], ShouldEqual, "2022-01-30 12:34:56")
	})
}
