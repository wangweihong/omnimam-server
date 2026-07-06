package sequential_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/sequential"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewSequentialList(t *testing.T) {
	Convey("TestNewSequentialList", t, func() {
		s := sequential.NewSequentialList("a", "a", "b")
		So(s.Len(), ShouldEqual, 3)
		So(s.Has("a"), ShouldBeTrue)
		So(len(s.Indices("a")), ShouldEqual, 2)
		So(s.List(), ShouldResemble, []interface{}{"a", "a", "b"})

		var b []interface{}
		err := s.ForEach(func(f interface{}) error {
			b = append(b, f)
			return nil
		})
		So(err, ShouldBeNil)
		So(b, ShouldResemble, s.List())

		s.Inject("a")
		So(len(s.Indices("a")), ShouldEqual, 3)
		So(s.List(), ShouldResemble, []interface{}{"a", "a", "b", "a"})

		s.DeleteAtIndex(0)
		So(len(s.Indices("a")), ShouldEqual, 2)
		So(s.Indices("a"), ShouldResemble, []int{0, 2})
		So(s.Indices("b"), ShouldResemble, []int{1})
		So(s.List(), ShouldResemble, []interface{}{"a", "b", "a"})

		s.Delete("a")
		So(len(s.Indices("a")), ShouldEqual, 0)
		So(s.Has("a"), ShouldBeFalse)
		So(s.List(), ShouldResemble, []interface{}{"b"})
	})
}

func TestNewLimitSequentialList(t *testing.T) {
	Convey("TestNewLimitSequentialMap", t, func() {
		s := sequential.NewLimitSequentialList(2, "a", "b", "c")
		So(s.Len(), ShouldEqual, 2)
		So(s.Has("a"), ShouldBeFalse)
		So(s.Has("b"), ShouldBeTrue)
		So(s.Has("c"), ShouldBeTrue)
		So(s.Get(1), ShouldResemble, "c")
		So(s.Get(2), ShouldBeNil)

		s.Clear()
		s.Inject("a")
		s.Inject("a")
		So(s.Len(), ShouldEqual, 2)
		So(s.List(), ShouldResemble, []interface{}{"a", "a"})

		s.Delete("a")
		So(s.Len(), ShouldEqual, 0)

		s.Clear()
		So(s.Inject("a"), ShouldEqual, 0)
		So(s.Inject("a"), ShouldEqual, 1)
		So(s.Inject("a"), ShouldEqual, 1)
		So(s.Indices("a"), ShouldResemble, []int{0, 1})

		l := make([]*string, 0)
		l = append(l, nil)
		m := make(map[*string]struct{})
		m[nil] = struct{}{}
	})
}

func TestNewSequentialList_Update(t *testing.T) {
	Convey("TestNewSequentialList_Update", t, func() {
		s := sequential.NewSequentialList("a", "b", "c")
		s.Update(0, "c")
		So(s.List(), ShouldResemble, []interface{}{"c", "b", "c"})
		So(s.Has("a"), ShouldBeFalse)
		So(s.Indices("c"), ShouldResemble, []int{0, 2})
		So(s.Indices("b"), ShouldResemble, []int{1})

		s.Clear()
		s.InjectList("a", "b", "c")
		s.Update(0, "a")
		So(s.Has("a"), ShouldBeTrue)
		So(s.Indices("c"), ShouldResemble, []int{2})
		So(s.Indices("b"), ShouldResemble, []int{1})
		So(s.Indices("a"), ShouldResemble, []int{0})
	})
}

func TestNewSequentialList_DeepCopy(t *testing.T) {
	Convey("TestNewSequentialList_DeepCopy", t, func() {
		s := sequential.NewSequentialList("a", "b", "c")
		s1 := s.DeepCopy()
		So(s, ShouldResemble, s1)
	})
}

func TestNewSequentialList_DeleteIf(t *testing.T) {
	Convey("TestNewSequentialList_DeleteIf", t, func() {
		s := sequential.NewSequentialList("a", "b", "c", nil, nil)
		So(s.Len(), ShouldEqual, 5)
		s.DeleteIf(func(value interface{}) bool {
			if value == nil {
				return true
			}
			return false
		})

		So(s.Len(), ShouldEqual, 3)
		So(s.List(), ShouldResemble, []interface{}{"a", "b", "c"})
	})
}

func TestNewSequentialList_MoveFront(t *testing.T) {
	Convey("TestNewSequentialList_MoveFront", t, func() {

		Convey("TestNewSequentialList_MoveFrontIF", func() {
			s := sequential.NewSequentialList("a", "b", "c", nil, nil)
			So(s.Len(), ShouldEqual, 5)
			s.MoveFrontIf(func(value interface{}) bool {
				if value == nil {
					return true
				}
				return false
			})

			So(s.Len(), ShouldEqual, 5)
			So(s.List(), ShouldResemble, []interface{}{nil, nil, "a", "b", "c"})
		})
		Convey("TestNewSequentialList_MoveFrontNumIF", func() {
			s := sequential.NewSequentialList("a", "b", "c", nil, nil)
			So(s.Len(), ShouldEqual, 5)
			s.MoveFrontNumIf(func(value interface{}) bool {
				if value == nil {
					return true
				}
				return false
			}, 1)

			So(s.Len(), ShouldEqual, 5)
			So(s.List(), ShouldResemble, []interface{}{nil, "a", "b", "c", nil})
		})
		Convey("TestNewSequentialList_MoveFront", func() {
			s := sequential.NewSequentialList("a", "b", "c", nil, nil)
			So(s.Len(), ShouldEqual, 5)
			s.MoveFront("b")

			So(s.Len(), ShouldEqual, 5)
			So(s.List(), ShouldResemble, []interface{}{"b", "a", "c", nil, nil})
		})
	})
}

func TestNewSequentialList_MoveAfter(t *testing.T) {
	Convey("TestNewSequentialList_MoveAfter", t, func() {

		Convey("TestNewSequentialList_MoveAfterIF", func() {
			s := sequential.NewSequentialList(nil, nil, "a", "b", "c")
			So(s.Len(), ShouldEqual, 5)
			s.MoveAfterIf(func(value interface{}) bool {
				if value == nil {
					return true
				}
				return false
			})

			So(s.Len(), ShouldEqual, 5)
			So(s.List(), ShouldResemble, []interface{}{"a", "b", "c", nil, nil})
		})
		Convey("TestNewSequentialList_MoveAfter", func() {
			s := sequential.NewSequentialList("a", "b", "c", nil, nil)
			So(s.Len(), ShouldEqual, 5)
			s.MoveAfter("b")

			So(s.Len(), ShouldEqual, 5)
			So(s.List(), ShouldResemble, []interface{}{"a", "c", nil, nil, "b"})
		})
	})
}
