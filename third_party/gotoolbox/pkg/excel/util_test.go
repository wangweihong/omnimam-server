package excel_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	excel "github.com/wangweihong/gotoolbox/pkg/excel"
)

type User struct {
	Name      string
	Age       int
	Active    bool
	Roles     []string
	Score     []string
	CreatedAt time.Time
}

func TestSetField(t *testing.T) {
	Convey("TestSetField", t, func() {
		registry := excel.NewimportParserRegistry()
		// 注册自定义解析器
		registry.RegisterFieldParser("User", "Roles", func(value string, _ reflect.Type) (interface{}, error) {
			// 特殊的分隔符处理
			if value == "" {
				return []string{}, nil
			}
			return strings.Split(value, "|"), nil
		})
		registry.RegisterFieldParser("User", "Score", func(value string, _ reflect.Type) (interface{}, error) {
			// 特殊的分隔符处理
			if value == "" {
				return []string{}, nil
			}
			return strings.Split(value, "-"), nil
		})
		user := &User{}

		// 设置字段值
		err := excel.SetField(user, "Name", "张三", registry)
		So(err, ShouldBeNil)
		err = excel.SetField(user, "Age", "30", registry)
		So(err, ShouldBeNil)
		err = excel.SetField(user, "Active", "是", registry)
		So(err, ShouldBeNil)
		err = excel.SetField(user, "Roles", "admin|user|guest", registry)
		So(err, ShouldBeNil)
		err = excel.SetField(user, "CreatedAt", "2023-05-15", registry)
		So(err, ShouldBeNil)
		So(user.Roles, ShouldResemble, []string{"admin", "user", "guest"})

		err = excel.SetField(user, "Score", "2023-05-15", registry)
		So(err, ShouldBeNil)
		So(user.Score, ShouldResemble, []string{"2023", "05", "15"})
	})

}
