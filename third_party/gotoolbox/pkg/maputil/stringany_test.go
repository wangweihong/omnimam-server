package maputil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
)

func TestStringAny_Equal(t *testing.T) {
	Convey("TestStringAny_Equal", t, func() {
		m1 := maputil.NewStringAny().Set("a", 1)
		m2 := maputil.NewStringAny().Set("a", 1).Set("b", "5")
		m3 := maputil.NewStringAny().Set("a", "1")

		Convey("not equal", func() {
			So(m1.Equal(m2), ShouldBeFalse)
			So(m1.Equal(m3), ShouldBeFalse)
			So(m2.Equal(m3), ShouldBeFalse)
		})
		Convey("no equal", func() {
			So(m1.Set("b", "5").Equal(m2), ShouldBeTrue)
		})
	})
}

func TestStringAny_Decode(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}
	Convey("TestStringAny_Decode", t, func() {
		var u User
		m1 := maputil.NewStringAny().Set("Name", "test").Set("Age", 14)
		err := m1.Decode(&u)
		So(err, ShouldBeNil)
		So(u.Name, ShouldEqual, "test")
		So(u.Age, ShouldEqual, 14)
	})
}
