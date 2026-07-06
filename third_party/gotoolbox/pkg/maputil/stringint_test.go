package maputil_test

import (
	"strings"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStringInt_Init(t *testing.T) {
	Convey("TestStringInt_Init", t, func() {
		var nilMap map[string]int

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			maputil.StringInt(nilMap).Init()
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringInt(nilMap).Init()
			So(nilMap, ShouldNotBeNil)
		})
	})
}

func TestStringInt_Set(t *testing.T) {
	Convey("TestStringInt_Set", t, func() {
		var nilMap map[string]int

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			maputil.StringInt(nilMap).Set("1", 2)
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringInt(nilMap).Set("a", 3).Set("c", 3)
			So(nilMap, ShouldNotBeNil)
			So(len(nilMap), ShouldEqual, 2)
		})
	})
}

func TestStringInt_DeepCopy(t *testing.T) {
	Convey("TestStringInt_Set", t, func() {
		var nilMap map[string]int

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringInt(nilMap).DeepCopy()
			So(nilMap, ShouldNotBeNil)

			nilMap = maputil.StringInt(nilMap).Set("a", 4)
			So(nilMap, ShouldNotBeNil)
			So(len(nilMap), ShouldEqual, 1)
			So(maputil.StringInt(nilMap).Has("a"), ShouldBeTrue)
		})
	})
}

func TestStringInt_Delete(t *testing.T) {
	Convey("TestStringInt_Delete", t, func() {
		Convey("nil", func() {
			var nilMap map[string]int
			maputil.StringInt(nilMap).Delete("a")
		})
		Convey("not nil", func() {
			d := make(map[string]int)
			d["a"] = 3

			maputil.StringInt(d).Delete("a")
			So(maputil.StringInt(d).Has("a"), ShouldBeFalse)
		})
	})
}

func TestStringInt_DeleteIfKey(t *testing.T) {
	Convey("TestStringInt_DeleteIfKey", t, func() {
		condition := func(k string) bool {
			if strings.Contains(k, "b") {
				return true
			}
			return false
		}
		Convey("nil", func() {
			var nilMap map[string]int
			maputil.StringInt(nilMap).DeleteIfKey(condition)
		})
		Convey("not nil", func() {
			d := make(map[string]int)
			d["ab"] = 10
			d["bb"] = 20
			d["cc"] = 31
			maputil.StringInt(d).DeleteIfKey(condition)

			So(maputil.StringInt(d).Has("ab"), ShouldBeFalse)
			So(maputil.StringInt(d).Has("bb"), ShouldBeFalse)
			So(maputil.StringInt(d).Has("cc"), ShouldBeTrue)
		})
	})
}

func TestStringInt_DeleteIfValue(t *testing.T) {
	Convey("TestStringInt_DeleteIfValue", t, func() {
		condition := func(k int) bool {
			if k%10 == 0 {
				return true
			}
			return false
		}
		Convey("nil", func() {
			var nilMap map[string]int
			maputil.StringInt(nilMap).DeleteIfValue(condition)
		})
		Convey("not nil", func() {
			d := make(map[string]int)
			d["ab"] = 10
			d["bb"] = 20
			d["cc"] = 31
			maputil.StringInt(d).DeleteIfValue(condition)
			So(maputil.StringInt(d).Has("ab"), ShouldBeFalse)
			So(maputil.StringInt(d).Has("bb"), ShouldBeFalse)
			So(maputil.StringInt(d).Has("cc"), ShouldBeTrue)
		})
	})
}

func TestStringInt_Get(t *testing.T) {
	Convey("TestStringInt_Get", t, func() {
		Convey("nil", func() {
			var nilMap map[string]int

			So(maputil.StringInt(nilMap).Get("notexist"), ShouldEqual, 0)
		})
		Convey("not nil", func() {
			d := make(map[string]int)
			d["a"] = 2

			So(maputil.StringInt(d).Get("a"), ShouldEqual, 2)
			So(maputil.StringInt(d).Get("notexist"), ShouldEqual, 0)
		})
	})
}

func TestStringInt_Keys(t *testing.T) {
	Convey("TestStringInt_Keys", t, func() {
		Convey("nil", func() {
			var nilMap map[string]int
			keys := maputil.StringInt(nilMap).Keys()

			So(len(keys), ShouldEqual, 0)
		})
		Convey("not nil", func() {
			d := make(map[string]int)
			d["a"] = 1
			d["b"] = 2

			keys := maputil.StringInt(d).Keys()
			So(len(keys), ShouldEqual, 2)
			So(sets.NewString(keys...).Equal(sets.NewString("a", "b")), ShouldBeTrue)
		})
	})
}
