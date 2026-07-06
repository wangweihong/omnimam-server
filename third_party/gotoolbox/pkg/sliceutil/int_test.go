package sliceutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIntSlice_DeepCopy(t *testing.T) {
	Convey("TestIntSlice_DeepCopy", t, func() {
		Convey("nil", func() {
			var d []int
			b := sliceutil.IntSlice(d).DeepCopy()

			So(d, ShouldBeNil)
			So(b, ShouldNotBeNil)
		})

		Convey("not nil", func() {
			d := []int{1}
			b := sliceutil.IntSlice(d).DeepCopy()
			d = append(d, 2)

			So(b, ShouldNotBeNil)
			So(len(b), ShouldEqual, 1)
		})
	})
}

func TestIntSlice_HasRepeat(t *testing.T) {
	Convey("TestIntSlice_HasRepeat", t, func() {
		var nilS []int
		So(sliceutil.IntSlice(nilS).HasRepeat(), ShouldBeFalse)
		So(sliceutil.IntSlice([]int{123, 123}).HasRepeat(), ShouldBeTrue)
		So(sliceutil.IntSlice([]int{123, 245}).HasRepeat(), ShouldBeFalse)
	})
}

func TestIntSlice_GetRepeat(t *testing.T) {
	Convey("TestIntSlice_GetRepeat", t, func() {
		var nilS []int
		var rm map[int]int
		var repeated bool

		rm, repeated = sliceutil.IntSlice(nilS).GetRepeat()
		So(rm, ShouldBeNil)
		So(repeated, ShouldBeFalse)

		rm, repeated = sliceutil.IntSlice([]int{12, 12, 12, 3}).GetRepeat()
		So(rm, ShouldNotBeNil)
		So(repeated, ShouldBeTrue)
		d, _ := rm[12]
		So(d, ShouldEqual, 3)

		rm, repeated = sliceutil.IntSlice([]int{12, 3}).GetRepeat()
		So(rm, ShouldBeNil)
		So(repeated, ShouldBeFalse)
	})
}

func TestIntSlice_Sort(t *testing.T) {
	Convey("TestIntSlice_Sort", t, func() {
		var nilS []int
		So(sliceutil.IntSlice(nilS).SortAsc(), ShouldBeNil)
		So(sliceutil.IntSlice([]int{1, 3, 2}).SortAsc(), ShouldResemble, []int{1, 2, 3})
		So(sliceutil.IntSlice([]int{1, 3, 2}).SortAsc(), ShouldNotResemble, []int{1, 3, 2})

		So(sliceutil.IntSlice(nilS).SortDesc(), ShouldBeNil)
		So(sliceutil.IntSlice([]int{1, 3, 2}).SortDesc(), ShouldResemble, []int{3, 2, 1})
		So(sliceutil.IntSlice([]int{1, 3, 2}).SortDesc(), ShouldNotResemble, []int{1, 3, 2})
	})
}

func TestIntSlice_RemoveIf(t *testing.T) {
	Convey("TestIntSlice_RemoveIf", t, func() {
		condition := func(s int) bool {
			if s%10 == 0 {
				return true
			}
			return false
		}
		So(sliceutil.IntSlice([]int{10, 11, 10}).RemoveIf(condition), ShouldResemble, []int{11})
	})
}

func TestIntSlice_AppendIf(t *testing.T) {
	Convey("TestIntSlice_AppendIf", t, func() {
		condition := func(s int) bool {
			if s%10 == 0 {
				return true
			}
			return false
		}
		s := []int{20, 22}
		d := sliceutil.IntSlice(s).AppendIf(condition, []int{30, 31})
		So(s, ShouldResemble, []int{20, 22})
		So(d, ShouldResemble, []int{20, 22, 30})
	})
}

func TestIntSlice_MoveFrontIf(t *testing.T) {
	Convey("TestIntSlice_RemoveIf", t, func() {
		condition := func(s int) bool {
			if s == 11 {
				return true
			}
			return false
		}
		So(sliceutil.IntSlice([]int{10, 11, 23}).MoveFront(condition), ShouldResemble, []int{11, 10, 23})
	})
}

func TestIntSlice_MoveAfterIf(t *testing.T) {
	Convey("TestIntSlice_MoveAfterIf", t, func() {
		condition := func(s int) bool {
			if s == 11 {
				return true
			}
			return false
		}
		So(sliceutil.IntSlice([]int{10, 11, 23}).MoveAfter(condition), ShouldResemble, []int{10, 23, 11})
	})
}

func TestIntSlice_MergeDifferentElementsFromEnd(t *testing.T) {
	Convey("MergeDifferentElementsFromEnd", t, func() {
		So(sliceutil.NewIntSlice(1, 2, 3).MergeDifferentElementsFromEnd([]int{4, 2, 3}), ShouldResemble, sliceutil.IntSlice{1, 4, 2, 3})
		So(sliceutil.NewIntSlice(1, 2, 3).MergeDifferentElementsFromEnd([]int{4, 1, 2, 3}), ShouldResemble, sliceutil.IntSlice{4, 1, 2, 3})
		So(sliceutil.NewIntSlice(5, 4, 1, 2, 3).MergeDifferentElementsFromEnd([]int{1, 2, 3}), ShouldResemble, sliceutil.IntSlice{5, 4, 1, 2, 3})
		So(sliceutil.NewIntSlice(5, 4, 1, 2, 3).MergeDifferentElementsFromEnd([]int{7, 1, 2, 3}), ShouldResemble, sliceutil.IntSlice{5, 4, 7, 1, 2, 3})
	})
}
