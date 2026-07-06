package maputil_test

import (
	"strings"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStringString_Init(t *testing.T) {
	Convey("TestStringString_Init", t, func() {
		var nilMap map[string]string

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			maputil.StringString(nilMap).Init()
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringString(nilMap).Init()
			So(nilMap, ShouldNotBeNil)
		})
	})
}

func TestStringString_Set(t *testing.T) {
	Convey("TestStringString_Set", t, func() {
		var nilMap map[string]string

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			maputil.StringString(nilMap).Set("1", "2")
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringString(nilMap).Set("a", "b").Set("c", "D")
			So(nilMap, ShouldNotBeNil)
			So(len(nilMap), ShouldEqual, 2)
		})
	})
}

func TestStringString_DeepCopy(t *testing.T) {
	Convey("TestStringString_Set", t, func() {
		var nilMap map[string]string

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringString(nilMap).DeepCopy()
			So(nilMap, ShouldNotBeNil)

			nilMap = maputil.StringString(nilMap).Set("a", "b")
			So(nilMap, ShouldNotBeNil)
			So(len(nilMap), ShouldEqual, 1)
			So(maputil.StringString(nilMap).Has("a"), ShouldBeTrue)
		})
	})
}

func TestStringString_Delete(t *testing.T) {
	Convey("TestStringString_Delete", t, func() {
		Convey("nil", func() {
			var nilMap map[string]string
			maputil.StringString(nilMap).Delete("a")
		})
		Convey("not nil", func() {
			d := make(map[string]string)
			d["a"] = "b"

			maputil.StringString(d).Delete("a")
			So(maputil.StringString(d).Has("a"), ShouldBeFalse)
		})
	})
}

func TestStringString_DeleteIfKey(t *testing.T) {
	Convey("TestStringString_DeleteIfKey", t, func() {
		condition := func(k string) bool {
			if strings.Contains(k, "b") {
				return true
			}
			return false
		}
		Convey("nil", func() {
			var nilMap map[string]string
			maputil.StringString(nilMap).DeleteIfKey(condition)
		})
		Convey("not nil", func() {
			d := make(map[string]string)
			d["ab"] = "ab"
			d["bb"] = "bb"
			d["cc"] = "cc"
			maputil.StringString(d).DeleteIfKey(condition)

			So(maputil.StringString(d).Has("ab"), ShouldBeFalse)
			So(maputil.StringString(d).Has("bb"), ShouldBeFalse)
			So(maputil.StringString(d).Has("cc"), ShouldBeTrue)
		})
	})
}

func TestStringString_DeleteIfValue(t *testing.T) {
	Convey("TestStringString_DeleteIfValue", t, func() {
		condition := func(k string) bool {
			if strings.Contains(k, "b") {
				return true
			}
			return false
		}
		Convey("nil", func() {
			var nilMap map[string]string
			maputil.StringString(nilMap).DeleteIfValue(condition)
		})
		Convey("not nil", func() {
			d := make(map[string]string)
			d["ab"] = "ab"
			d["bb"] = "bb"
			d["cc"] = "cc"
			maputil.StringString(d).DeleteIfValue(condition)
			So(maputil.StringString(d).Has("ab"), ShouldBeFalse)
			So(maputil.StringString(d).Has("bb"), ShouldBeFalse)
			So(maputil.StringString(d).Has("cc"), ShouldBeTrue)
		})
	})
}

func TestStringString_Get(t *testing.T) {
	Convey("TestStringString_Get", t, func() {
		Convey("nil", func() {
			var nilMap map[string]string

			So(maputil.StringString(nilMap).Get("notexist"), ShouldEqual, "")
		})
		Convey("not nil", func() {
			d := make(map[string]string)
			d["a"] = "b"

			So(maputil.StringString(d).Get("a"), ShouldEqual, "b")
			So(maputil.StringString(d).Get("notexist"), ShouldEqual, "")
		})
	})
}

func TestStringString_Keys(t *testing.T) {
	Convey("TestStringString_Keys", t, func() {
		Convey("nil", func() {
			var nilMap map[string]string
			keys := maputil.StringString(nilMap).Keys()

			So(len(keys), ShouldEqual, 0)
		})
		Convey("not nil", func() {
			d := make(map[string]string)
			d["a"] = "1"
			d["b"] = "2"

			keys := maputil.StringString(d).Keys()
			So(len(keys), ShouldEqual, 2)
			So(sets.NewString(keys...).Equal(sets.NewString("a", "b")), ShouldBeTrue)
		})
	})
}
