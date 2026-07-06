package validator_test

import (
	"fmt"
	"testing"

	gvalidator "github.com/go-playground/validator/v10"
	. "github.com/smartystreets/goconvey/convey"

	"github.com/wangweihong/omnimam/backend/pkg/validator"
)

type Address struct {
	City string `binding:"required"`
}

func (a Address) Validate() error {
	if len(a.City) < 3 {
		return fmt.Errorf("city must be at least 3 characters")
	}
	return nil
}

type Profile struct {
	Age      int `binding:"min=18"`
	Metadata map[string]string
}

func (p *Profile) Validate() error {
	if p == nil {
		return fmt.Errorf("profile cannot be nil")
	}
	if p.Age > 120 {
		return fmt.Errorf("invalid age")
	}
	return nil
}

type User struct {
	Name     string   `binding:"alpha"`
	Email    string   `binding:"email"`
	Address  Address  `binding:"required"` // 嵌套结构体
	Profile  *Profile // 嵌套指针
	Settings struct { // 匿名结构体
		DarkMode bool
	}
}

func TestValidateAll(t *testing.T) {
	Convey("TestValidateAll", t, func() {
		SkipConvey("TestValidateAll-binding", func() {
			user := &User{
				Name:  "John123", // 无效：包含数字
				Email: "invalid-email",
				Address: Address{
					City: "NY", // 长度不足3
				},
				Profile: &Profile{
					Age: 150, // 超过120
				},
			}
			err := validator.ValidateAll(user)
			if err != nil {
				fmt.Println("Combined validation failed:")
				verr, ok := err.(gvalidator.ValidationErrors)
				So(ok, ShouldBeTrue)
				So(len(verr), ShouldNotEqual, 0)

				for _, e := range verr {
					t.Logf("- %s: %s\n", e.Field(), e.Tag())
				}
			}
		})
		Convey("TestValidateAll-interface", func() {
			user := &User{
				Name:  "John",
				Email: "test@126.com",
				Address: Address{
					City: "NY3",
				},
				Profile: &Profile{
					Age: 150, // 超过120
				},
			}
			err := validator.ValidateAll(user)
			So(err, ShouldNotBeNil)
			_, ok := err.(gvalidator.ValidationErrors)
			So(ok, ShouldBeFalse)
			fmt.Println(err)

		})

	})

}
