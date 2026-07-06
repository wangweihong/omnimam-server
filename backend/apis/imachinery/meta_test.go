package imachinery_test

import (
	"strings"
	"testing"

	gvalidator "github.com/go-playground/validator/v10"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"
	"github.com/wangweihong/gotoolbox/pkg/validation"

	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/pkg/validator"
)

func TestMetaValidate(t *testing.T) {
	val := gvalidator.New()
	val.SetTagName("binding")
	val.RegisterValidation("name", validator.ValidateName)
	val.RegisterValidation("description", validation.ValidateDescription)

	Convey("Test Meta ", t, func() {
		Convey("Test Meta name ok", func() {
			validateNames := []string{
				"Meta",
				"MetaList",
				"Harbor1",
				"Harbor2",
				"harbor-1323",
			}
			invalidNames := []string{
				"",
				"--_MetaList",
				"**",
				"_MetaList))",
				"harbor__",
				"example.com",
			}
			for _, name := range validateNames {
				Convey("It should return success for "+name, func() {
					d := imachinery.ObjectMeta{Name: name}
					So(val.Struct(d), ShouldBeNil)
				})
			}
			for _, name := range invalidNames {
				Convey("It should return fail for "+name, func() {
					d := imachinery.ObjectMeta{Name: name}
					So(val.Struct(d), ShouldNotBeNil)
				})
			}
		})

		Convey("Test Meta description ", func() {
			d := imachinery.ObjectMeta{Name: "test"}

			validDesc := []string{
				"",
				"test",
				"Harbor1",
				"Harbor2",
				"harbor-1323",
				stringutil.LenEmptyString(255),
			}

			invalidDesc := []string{
				stringutil.LenEmptyString(256),
			}

			for _, desc := range validDesc {
				Convey("It should return success for "+desc, func() {
					d.Description = desc
					So(val.Struct(d), ShouldBeNil)
				})
			}
			for _, desc := range invalidDesc {
				Convey("It should return fail for "+desc, func() {
					d.Description = desc
					err := val.Struct(d)
					So(err, ShouldNotBeNil)
					t.Log(err)
				})
			}
		})
	})
}

func TestMetaValidate2(t *testing.T) {
	val := validator.NewCustomValidator("en")
	val.Engine()

	Convey("Test Meta ", t, func() {
		Convey("Test Meta name ok", func() {
			validateNames := []string{
				"Meta",
				"MetaList",
				"Harbor1",
				"Harbor2",
				"harbor-1323",
			}
			invalidNames := []string{
				"",
				"a",
				strings.Repeat("a", 17),
				"--_MetaList",
				"**",
				"_MetaList))",
				"harbor__",
				"example.com",
			}
			for _, name := range validateNames {
				Convey("It should return success for "+name, func() {
					d := imachinery.ObjectMeta{Name: name}
					So(val.Validate(d), ShouldBeNil)
				})
			}
			for _, name := range invalidNames {
				Convey("It should return fail for "+name, func() {
					d := imachinery.ObjectMeta{Name: name}
					err := val.Validate(d)
					So(err, ShouldNotBeNil)
					t.Log(err)
				})
			}
		})

		Convey("Test Meta description ", func() {
			d := imachinery.ObjectMeta{Name: "test"}

			validDesc := []string{
				"",
				"test",
				"Harbor1",
				"Harbor2",
				"harbor-1323",
				stringutil.LenEmptyString(255),
			}

			for _, desc := range validDesc {
				Convey("It should return success for "+desc, func() {
					d.Description = desc
					So(val.Validate(d), ShouldBeNil)
				})
			}

			invalidDesc := []string{
				stringutil.LenEmptyString(257),
			}

			for _, desc := range invalidDesc {
				Convey("It should return fail for "+desc, func() {
					d.Description = desc
					err := val.Validate(d)
					So(err, ShouldNotBeNil)
					t.Log(err)
				})
			}
		})
	})
}
