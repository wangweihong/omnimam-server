package maputil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/maputil"
)

func TestGet(t *testing.T) {
	Convey("GetFromMap", t, func() {
		st := map[string]int{
			"a": 1,
			"b": 2,
		}

		So(maputil.Get(st, "a"), ShouldEqual, 1)
		So(maputil.Has(st, "a"), ShouldBeTrue)
		So(maputil.Get(st, "c"), ShouldEqual, 0)
		So(maputil.Has(st, "c"), ShouldBeFalse)

		m2 := map[string]string{
			"a": "1",
			"b": "2",
		}

		So(maputil.Get(m2, "a"), ShouldEqual, "1")
		So(maputil.Has(m2, "a"), ShouldBeTrue)
		So(maputil.Get(m2, "c"), ShouldEqual, "")
		So(maputil.Has(m2, "c"), ShouldBeFalse)

		m3 := map[string]any{
			"a": "1",
			"b": 2,
		}

		So(maputil.Get(m3, "a"), ShouldEqual, "1")
		So(maputil.Has(m3, "a"), ShouldBeTrue)
		So(maputil.Get(m3, "b"), ShouldEqual, 2)
		So(maputil.Has(m3, "b"), ShouldBeTrue)
		So(maputil.Get(m3, "c"), ShouldBeNil)
		So(maputil.Has(m3, "c"), ShouldBeFalse)

	})
}

func TestTypedGet(t *testing.T) {
	Convey("TestTypedGet", t, func() {
		st := map[string]any{
			"a":      1,
			"pi":     3.14,
			"name":   "alice",
			"active": true,
		}

		So(maputil.TypedGet[string, int](st, "a"), ShouldEqual, 1)
		So(maputil.TypedGet[string, float64](st, "pi"), ShouldEqual, 3.14)
		So(maputil.TypedGet[string, string](st, "name"), ShouldEqual, "alice")
		So(maputil.TypedGet[string, bool](st, "active"), ShouldEqual, true)

	})
}

func TestDelete(t *testing.T) {
	Convey("Delete", t, func() {
		st := map[string]int{
			"a": 1,
			"b": 2,
		}

		maputil.Delete(st, "a")
		So(len(st), ShouldEqual, 1)
		maputil.Delete(st, "noexist")
		So(len(st), ShouldEqual, 1)

		m2 := map[string]string{
			"a": "1",
			"b": "2",
		}
		maputil.Delete(m2, "a")
		So(len(m2), ShouldEqual, 1)
	})
}

func TestInsert(t *testing.T) {
	Convey("Insert", t, func() {
		st := map[string]int{}

		st = maputil.Insert(st, "a", 1)
		So(len(st), ShouldEqual, 1)

		m2 := map[string]any{}

		m2 = maputil.Insert(m2, "a", 1)
		m2 = maputil.Insert(m2, "b", "3")
		So(len(m2), ShouldEqual, 2)

		So(maputil.Get(m2, "a"), ShouldEqual, 1)
		So(maputil.Get(m2, "b"), ShouldEqual, "3")
	})
}

func TestClone(t *testing.T) {
	Convey("Clone", t, func() {
		st := map[string]int{
			"a": 1,
			"b": 2,
		}
		n := maputil.Clone(st)
		So(len(n), ShouldEqual, 2)

		maputil.Delete(n, "a")
		So(len(n), ShouldEqual, 1)
		So(len(st), ShouldEqual, 2)
	})
}

func TestToString(t *testing.T) {
	Convey("TestGet*FromMapInterface", t, func() {
		msi := make(map[string]int)
		msi["k"] = 0
		msi["v"] = 1

		So(maputil.ToString(msi), ShouldEqual, "k=0,v=1")

		mss := make(map[string]string)
		mss["k"] = "i"
		mss["v"] = "j"
		So(maputil.ToString(mss), ShouldEqual, "k=i,v=j")
	})
}

func TestKeys(t *testing.T) {
	Convey("Keys", t, func() {
		st := map[string]int{
			"a": 1,
			"b": 2,
		}
		n := maputil.Keys(st)
		So(len(n), ShouldEqual, 2)
		So(n, ShouldResemble, []string{"a", "b"})

		m2 := map[int]string{
			1: "1",
			2: "2",
		}
		n2 := maputil.Keys(m2)
		So(len(n2), ShouldEqual, 2)
		So(n2, ShouldResemble, []int{1, 2})

	})
}
