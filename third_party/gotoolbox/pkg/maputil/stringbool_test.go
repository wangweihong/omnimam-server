package maputil_test

import (
	"strings"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/maputil"
	"github.com/wangweihong/gotoolbox/pkg/sets"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStringBool_Init(t *testing.T) {
	Convey("TestStringBool_Init", t, func() {
		var nilMap map[string]bool

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			maputil.StringBool(nilMap).Init()
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringBool(nilMap).Init()
			So(nilMap, ShouldNotBeNil)
		})
	})
}

func TestStringBool_Set(t *testing.T) {
	Convey("TestStringBool_Set", t, func() {
		var nilMap map[string]bool

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			maputil.StringBool(nilMap).Set("1", true)
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringBool(nilMap).Set("a", true).Set("c", true)
			So(nilMap, ShouldNotBeNil)
			So(len(nilMap), ShouldEqual, 2)
		})
	})
}

func TestStringBool_DeepCopy(t *testing.T) {
	Convey("TestStringBool_Set", t, func() {
		var nilMap map[string]bool

		Convey("not nil", func() {
			So(nilMap, ShouldBeNil)

			nilMap = maputil.StringBool(nilMap).DeepCopy()
			So(nilMap, ShouldNotBeNil)

			nilMap = maputil.StringBool(nilMap).Set("a", true)
			So(nilMap, ShouldNotBeNil)
			So(len(nilMap), ShouldEqual, 1)
			So(maputil.StringBool(nilMap).Has("a"), ShouldBeTrue)
		})
	})
}

func TestStringBool_Delete(t *testing.T) {
	Convey("TestStringBool_Delete", t, func() {
		Convey("nil", func() {
			var nilMap map[string]bool
			maputil.StringBool(nilMap).Delete("a")
		})
		Convey("not nil", func() {
			d := make(map[string]bool)
			d["a"] = true

			maputil.StringBool(d).Delete("a")
			So(maputil.StringBool(d).Has("a"), ShouldBeFalse)
		})
	})
}

func TestStringBool_DeleteIfKey(t *testing.T) {
	Convey("TestStringBool_DeleteIfKey", t, func() {
		condition := func(k string) bool {
			if strings.Contains(k, "b") {
				return true
			}
			return false
		}
		Convey("nil", func() {
			var nilMap map[string]bool
			maputil.StringBool(nilMap).DeleteIfKey(condition)
		})
		Convey("not nil", func() {
			d := make(map[string]bool)
			d["ab"] = true
			d["bb"] = true
			d["cc"] = true
			maputil.StringBool(d).DeleteIfKey(condition)

			So(maputil.StringBool(d).Has("ab"), ShouldBeFalse)
			So(maputil.StringBool(d).Has("bb"), ShouldBeFalse)
			So(maputil.StringBool(d).Has("cc"), ShouldBeTrue)
		})
	})
}

func TestStringBool_DeleteIfValue(t *testing.T) {
	Convey("TestStringBool_DeleteIfValue", t, func() {
		condition := func(k bool) bool {
			return k
		}
		Convey("nil", func() {
			var nilMap map[string]bool
			maputil.StringBool(nilMap).DeleteIfValue(condition)
		})
		Convey("not nil", func() {
			d := make(map[string]bool)
			d["ab"] = true
			d["bb"] = false
			d["cc"] = true
			maputil.StringBool(d).DeleteIfValue(condition)
			So(maputil.StringBool(d).Has("ab"), ShouldBeFalse)
			So(maputil.StringBool(d).Has("bb"), ShouldBeTrue)
			So(maputil.StringBool(d).Has("cc"), ShouldBeFalse)
		})
	})
}

func TestStringBool_Get(t *testing.T) {
	Convey("TestStringBool_Get", t, func() {
		Convey("nil", func() {
			var nilMap map[string]bool
			So(maputil.StringBool(nilMap).Get("notexist"), ShouldBeFalse)
		})
		Convey("not nil", func() {
			d := make(map[string]bool)
			d["a"] = true

			So(maputil.StringBool(d).Get("a"), ShouldBeTrue)
			So(maputil.StringBool(d).Get("noexist"), ShouldBeFalse)
		})
	})
}

func TestStringBool_Keys(t *testing.T) {
	Convey("TestStringBool_Keys", t, func() {
		Convey("nil", func() {
			var nilMap map[string]bool
			keys := maputil.StringBool(nilMap).Keys()

			So(len(keys), ShouldEqual, 0)
		})
		Convey("not nil", func() {
			d := make(map[string]bool)
			d["a"] = true
			d["b"] = true

			keys := maputil.StringBool(d).Keys()
			So(len(keys), ShouldEqual, 2)
			So(sets.NewString(keys...).Equal(sets.NewString("a", "b")), ShouldBeTrue)
		})
	})
}

func TestStringBool_ToSetString(t *testing.T) {
	Convey("TestStringBool_ToSetString", t, func() {
		So(maputil.StringBool(nil).ToSetString(), ShouldNotBeNil)

		d := make(map[string]bool)
		d["a"] = true
		d["b"] = true

		ss := maputil.StringBool(d).ToSetString()
		So(ss.Len(), ShouldEqual, 2)
	})
}
