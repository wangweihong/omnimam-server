package validation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/validation"
	"github.com/wangweihong/gotoolbox/pkg/validation/field"
)

func TestValidate(t *testing.T) {
	type SubReq struct {
		UserName string `validate:"name" json:"username"`
	}
	type AddUserReq struct {
		Username string `validate:"name" comment:"用户名" json:"username"`
		Nickname string `validate:"omitempty,gt=6,lt=16" comment:"昵称" json:"nickname"`
		Age      int    `validate:"required,min=5,max=99" comment:"年龄" json:"age"`
		File     string `validate:"file" comment:"文件" json:"file"`
		Sub      SubReq `json:"sub"`
	}

	Convey("Test Validate", t, func() {

		r := AddUserReq{}
		r.Username = "aaaa_____***"
		r.Nickname = "aaaa"
		r.Age = 4
		r.File = "a.txt"
		val := validation.NewValidator()
		val.SetDefaultCustomValidator()
		err := val.Validate(r)
		// [AddUserReq.Username: Invalid value: "is not a valid name", AddUserReq.Nickname: Invalid value: "Nickname must be greater than 6 characters in length", AddUserReq.Age: Invalid value: "Age must be 5 or greater", AddUserReq.File: Invalid value: "File must point to an existing file, but found 'a.txt'", AddUserReq.Sub.UserName: Invalid value: "is not a valid name"]
		t.Log(err.ToAggregate().Error())

		val2 := validation.NewValidator()
		val2.SetDefaultCustomValidator()
		val2.SetDefaultTranslater(validation.LangZH)
		field.SetLanguage(field.LanguageZH)

		err = val2.Validate(r)
		// [用户名: 无效值: "不是一个有效的名称", 昵称: 无效值: "昵称长度必须大于6个字符", 年龄: 无效值: "年龄最小只能为5", 文件: 无效值: "文件必须指向一个存在的文件，但找到了'a.txt'", UserName: 无效值: "不是一个有效的名称"]
		t.Log(err.ToAggregate().Error())
	})
}
