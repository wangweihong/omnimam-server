package sliceutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStringSlice_DeepCopy(t *testing.T) {
	Convey("TestStringSlice_DeepCopy", t, func() {
		Convey("nil", func() {
			var d []string
			b := sliceutil.StringSlice(d).DeepCopy()

			So(d, ShouldBeNil)
			So(b, ShouldNotBeNil)
		})

		Convey("not nil", func() {
			d := []string{"aaa"}
			b := sliceutil.StringSlice(d).DeepCopy()
			d = append(d, "c")

			So(b, ShouldNotBeNil)
			So(len(b), ShouldEqual, 1)
		})
	})
}

func TestStringSlice_HasRepeat(t *testing.T) {
	Convey("TestStringSlice_HasRepeat", t, func() {
		var nilS []string
		So(sliceutil.StringSlice(nilS).HasRepeat(), ShouldBeFalse)
		So(sliceutil.StringSlice([]string{"a", "a"}).HasRepeat(), ShouldBeTrue)
		So(sliceutil.StringSlice([]string{"b", "a"}).HasRepeat(), ShouldBeFalse)
	})
}

func TestStringSlice_GetRepeat(t *testing.T) {
	Convey("TestStringSlice_GetRepeat", t, func() {
		var nilS []string
		var rm map[string]int
		var repeated bool

		rm, repeated = sliceutil.StringSlice(nilS).GetRepeat()
		So(rm, ShouldBeNil)
		So(repeated, ShouldBeFalse)

		rm, repeated = sliceutil.StringSlice([]string{"a", "a", "a", "b"}).GetRepeat()
		So(rm, ShouldNotBeNil)
		So(repeated, ShouldBeTrue)
		d, _ := rm["a"]
		So(d, ShouldEqual, 3)

		rm, repeated = sliceutil.StringSlice([]string{"b", "a"}).GetRepeat()
		So(rm, ShouldBeNil)
		So(repeated, ShouldBeFalse)
	})
}

func TestStringSlice_Sort(t *testing.T) {
	Convey("TestStringSlice_Sort", t, func() {
		var nilS []string
		So(sliceutil.StringSlice(nilS).SortAsc(), ShouldBeNil)
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).SortAsc(), ShouldResemble, []string{"a", "b", "c"})
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).SortAsc(), ShouldNotResemble, []string{"a", "c", "b"})

		So(sliceutil.StringSlice(nilS).SortDesc(), ShouldBeNil)
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).SortDesc(), ShouldResemble, []string{"c", "b", "a"})
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).SortDesc(), ShouldNotResemble, []string{"a", "c", "b"})

	})
}

func TestStringSlice_HasEmpty(t *testing.T) {
	Convey("TestStringSlice_HasEmpty", t, func() {
		var nilS []string
		num, hasEmpty := sliceutil.StringSlice(nilS).HasEmpty()
		So(num, ShouldEqual, 0)
		So(hasEmpty, ShouldBeFalse)

		num, hasEmpty = sliceutil.StringSlice([]string{"", ""}).HasEmpty()
		So(num, ShouldEqual, 2)
		So(hasEmpty, ShouldBeTrue)

		num, hasEmpty = sliceutil.StringSlice([]string{"a", "b"}).HasEmpty()
		So(num, ShouldEqual, 0)
		So(hasEmpty, ShouldBeFalse)

		num, hasEmpty = sliceutil.StringSlice([]string{"", "b"}).HasEmpty()
		So(num, ShouldEqual, 1)
		So(hasEmpty, ShouldBeTrue)
	})
}

func TestStringSlice_Cut(t *testing.T) {
	Convey("TestStringSlice_Cut", t, func() {
		var nilS []string
		So(sliceutil.StringSlice(nilS).Cut("x"), ShouldBeNil)
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).Cut("a"), ShouldResemble, []string{"c", "b"})
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).Cut("c"), ShouldResemble, []string{"a", "b"})
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).Cut("b"), ShouldResemble, []string{"a", "c"})
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).Cut("d"), ShouldResemble, []string{"a", "c", "b"})
	})
}

func TestStringSlice_FallBack(t *testing.T) {
	Convey("TestStringSlice_FallBack", t, func() {
		var nilS []string
		So(sliceutil.StringSlice(nilS).FallBehind("x"), ShouldResemble, []string{"x"})
		So(sliceutil.StringSlice([]string{"a", "c", "b"}).FallBehind("a"), ShouldResemble, []string{"c", "b", "a"})
	})
}

func TestStringSlice_TrimSpace(t *testing.T) {
	Convey("TestStringSlice_TrimSpace", t, func() {
		var nilS []string
		So(sliceutil.StringSlice(nilS).TrimSpace(), ShouldBeNil)
		So(sliceutil.StringSlice([]string{"a", "", " ", "  ", "b"}).TrimSpace(), ShouldResemble, []string{"a", "b"})
	})
}

func TestStringSlice_RemoveIf(t *testing.T) {
	Convey("TestStringSlice_RemoveIf", t, func() {
		condition := func(s string) bool {
			if s == "b" {
				return true
			}
			return false
		}
		So(sliceutil.StringSlice([]string{"a", "b", "c"}).RemoveIf(condition), ShouldResemble, []string{"a", "c"})
	})
}

func TestStringSlice_AppendIf(t *testing.T) {
	Convey("TestStringSlice_AppendIf", t, func() {
		condition := func(s string) bool {
			if s == "b" {
				return true
			}
			return false
		}
		s := []string{"a", "b"}
		d := sliceutil.StringSlice(s).AppendIf(condition, []string{"a", "b", "c"})
		So(s, ShouldResemble, []string{"a", "b"})
		So(d, ShouldResemble, []string{"a", "b", "b"})
	})
}

func TestStringSlice_Index(t *testing.T) {
	Convey("TestStringSlice_Index", t, func() {
		s := []string{"a", "b", "c"}
		ss := sliceutil.StringSlice(s)
		So(ss.Index("b"), ShouldEqual, 1)
		So(ss.Index("d"), ShouldEqual, -1)
	})
}

func TestStringSlice_MoveFirst(t *testing.T) {
	Convey("TestStringSlice_MoveFirst", t, func() {
		s := []string{"a", "b", "c"}
		ss := sliceutil.StringSlice(s)
		So(ss.MoveFirst("c"), ShouldResemble, []string{"c", "a", "b"})
		So(ss.MoveFirst("a"), ShouldResemble, []string{"a", "b", "c"})
		So(ss.MoveFirst("d"), ShouldResemble, []string{"a", "b", "c"})
	})
}

func TestStringSlice_String(t *testing.T) {
	Convey("TestStringSlice_String", t, func() {
		s := []string{"a", "b", "c"}
		ss := sliceutil.StringSlice(s)
		So(ss.String(), ShouldResemble, "[a,b,c]")
		So(sliceutil.StringSlice(nil).String(), ShouldResemble, "nil")
	})
}

func TestStringSlice_Max(t *testing.T) {
	Convey("TestStringSlice_Max", t, func() {
		s := []string{"1", "12", "15"}
		ss := sliceutil.StringSlice(s)
		So(ss.Max(), ShouldEqual, "15")
	})
}
