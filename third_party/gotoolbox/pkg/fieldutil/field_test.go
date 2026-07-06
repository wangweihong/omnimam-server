package fieldutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/fieldutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParseStructField(t *testing.T) {
	Convey("提取结构体中的字段信息", t, func() {
		So(fieldutil.ParseStructFields(nil), ShouldBeNil)
		So(fieldutil.ParseStructFields("str"), ShouldBeNil)
		So(fieldutil.ParseStructFields(0), ShouldBeNil)

		type Embedded struct {
			B string `json:"b"`
		}
		type Mystruct struct {
			Embedded
			A string `json:"a"`
			c string `json:"c"`
		}
		var myP *Mystruct

		pfs := fieldutil.ParseStructFields(myP)
		So(pfs, ShouldBeNil)

		myP = &Mystruct{
			Embedded: Embedded{B: "b"},
			A:        "123",
			c:        "o",
		}

		fs := fieldutil.ParseStructFields(myP)
		So(fs, ShouldNotBeNil)
		So(len(fs), ShouldEqual, 3)
	})
}

func TestParseStructFieldValues(t *testing.T) {
	Convey("提取结构体中的字段信息和值", t, func() {
		So(fieldutil.ParseStructFieldValues(nil), ShouldBeNil)
		So(fieldutil.ParseStructFieldValues("str"), ShouldBeNil)
		So(fieldutil.ParseStructFieldValues(0), ShouldBeNil)

		type Embedded struct {
			B string `json:"b"`
		}
		type Mystruct struct {
			Embedded
			A string `json:"a"`
			c string `json:"c"`
		}
		var myP *Mystruct

		pfs := fieldutil.ParseStructFieldValues(myP)
		So(pfs, ShouldBeNil)

		myP = &Mystruct{
			Embedded: Embedded{B: "b"},
			A:        "123",
			c:        "o",
		}

		fs := fieldutil.ParseStructFieldValues(myP)
		So(fs, ShouldNotBeNil)
		So(len(fs), ShouldEqual, 3)
		So(fs[0].T.Name, ShouldEqual, "Embedded")
		So(fs[0].V.Interface(), ShouldResemble, Embedded{B: "b"})

		So(fs.FieldByName("B"), ShouldBeNil)
		So(fs.FieldByName("B", fieldutil.WithIterate()), ShouldNotBeNil)
		So(fs.FieldByName("A"), ShouldNotBeNil)
		So(fs.FieldByName("c"), ShouldNotBeNil)
		So(fs.FieldByName("c", fieldutil.WithExport()), ShouldBeNil)

	})
}
