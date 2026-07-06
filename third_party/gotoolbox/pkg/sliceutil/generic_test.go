package sliceutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestUnique(t *testing.T) {
	Convey("TestRemoveDuplicates", t, func() {
		is := []int{1, 2, 3, 4, 4, 5, 1, 2}
		ss := []string{"a", "b", "c", "a", "e"}
		So(sliceutil.Unique(is), ShouldResemble, []int{1, 2, 3, 4, 5})
		So(sliceutil.Unique(ss), ShouldResemble, []string{"a", "b", "c", "e"})
		ss = sliceutil.Push(ss, "f")
		So(ss, ShouldResemble, []string{"a", "b", "c", "a", "e", "f"})
		ns, elem, _ := sliceutil.Pop(ss)
		So(ns, ShouldResemble, []string{"a", "b", "c", "a", "e"})
		So(elem, ShouldResemble, "f")
	})
}

func TestMin(t *testing.T) {
	Convey("TestMin", t, func() {
		is := []int{1, 2, 3, 4, 4, 5, 1, 2}
		So(sliceutil.Min(is, func(a, b int) bool {
			return a < b
		}), ShouldEqual, 1)
	})

	Convey("TestMin", t, func() {
		type Example struct {
			Name string
			Age  int
			Num  int
		}
		var is []Example
		is = append(is, Example{
			Name: "tony",
			Age:  12,
			Num:  10,
		})
		is = append(is, Example{
			Name: "jack",
			Age:  13,
			Num:  3,
		})
		So(sliceutil.Min(is, func(a, b Example) bool {
			return a.Age < b.Age
		}).Name, ShouldEqual, "tony")

		So(sliceutil.Min(is, func(a, b Example) bool {
			return a.Num < b.Num
		}).Name, ShouldEqual, "jack")
	})
}

func TestSliceCount(t *testing.T) {
	Convey("TestSliceCount", t, func() {
		s := []int{1, 2, 3}
		So(sliceutil.SliceContent(s, 5), ShouldResemble, s)
		So(sliceutil.SliceContent(s, 1), ShouldResemble, []int{1})
	})
}

func TestZeroCount(t *testing.T) {
	Convey("TestZeroCount", t, func() {
		s := []int{1, 0, 3, 0}
		So(sliceutil.ZeroCount(s), ShouldResemble, 2)

		s2 := []string{"1", "", "3", "4"}
		So(sliceutil.ZeroCount(s2), ShouldResemble, 1)
	})
}
