package sequential_test

import (
	"fmt"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/sequential"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewSequentialMap(t *testing.T) {
	Convey("TestNewSequentialMap", t, func() {
		nonexist := "nonexist"
		s := sequential.NewSequentialMap()
		s.Inject("a", 123)
		s.Inject("b", 321)
		s.Inject("c", 456)

		So(s.Len(), ShouldEqual, 3)
		So(s.Keys(), ShouldResemble, []interface{}{"a", "b", "c"})
		So(s.Values(), ShouldResemble, []interface{}{123, 321, 456})
		So(s.Has("a"), ShouldBeTrue)
		So(s.Has(nonexist), ShouldBeFalse)
		So(s.Get("a"), ShouldEqual, 123)
		So(s.Get(nonexist), ShouldEqual, nil)

		s.Delete(nonexist)
		So(s.Len(), ShouldEqual, 3)
		So(s.Keys(), ShouldResemble, []interface{}{"a", "b", "c"})
		So(s.Values(), ShouldResemble, []interface{}{123, 321, 456})

		s.Delete("b")
		So(s.Len(), ShouldEqual, 2)
		So(s.Keys(), ShouldResemble, []interface{}{"a", "c"})
		So(s.Values(), ShouldResemble, []interface{}{123, 456})

		err := s.ForEach(func(value interface{}) error {
			fmt.Println(value)
			return nil
		})
		So(err, ShouldBeNil)

		s.Inject("b", 321)
		So(s.Len(), ShouldEqual, 3)
		So(s.Keys(), ShouldResemble, []interface{}{"a", "c", "b"})
		So(s.Values(), ShouldResemble, []interface{}{123, 456, 321})

		s.Inject("b", 789)
		So(s.Len(), ShouldEqual, 3)
		So(s.Keys(), ShouldResemble, []interface{}{"a", "c", "b"})
		So(s.Values(), ShouldResemble, []interface{}{123, 456, 789})
	})
}

func TestNewLimitSequentialMap(t *testing.T) {
	Convey("TestNewLimitSequentialMap", t, func() {
		s := sequential.NewLimitSequentialMap(2)
		s.Inject("a", 123)
		s.Inject("b", 321)
		s.Inject("c", 456)

		So(s.Len(), ShouldEqual, 2)
		So(s.Has("a"), ShouldBeFalse)
		So(s.Has("b"), ShouldBeTrue)
		So(s.Has("c"), ShouldBeTrue)

		s.Clear()
		s.Inject("a", 123)
		s.Inject("b", 321)
		s.Inject("a", 123)
		So(s.Len(), ShouldEqual, 2)
		So(s.Keys(), ShouldResemble, []interface{}{"a", "b"})
	})
}

func TestNewSequentialMap_Update(t *testing.T) {
	Convey("TestNewSequentialMap", t, func() {
		s := sequential.NewSequentialMap()
		s.Inject("a", 123)
		s.Inject("b", 321)
		s.Inject("a", 456)

		So(s.Len(), ShouldEqual, 2)
		So(s.Get("a"), ShouldEqual, 456)
	})
}

func TestNewSequentialMap_DeepCopy(t *testing.T) {
	Convey("TestNewSequentialMap_DeepCopy", t, func() {
		s := sequential.NewSequentialMap()
		s.Inject("a", 123)
		s.Inject("b", 321)
		s.Inject("a", 456)

		s1 := s.DeepCopy()
		So(s, ShouldResemble, s1)
	})
}

func TestNewSequentialMap_DeleteIf(t *testing.T) {
	Convey("TestNewSequentialMap_DeleteIf", t, func() {
		s := sequential.NewSequentialMap()
		s.Inject("a", 123)
		s.Inject("b", 321)
		s.Inject("c", 456)
		s.Inject("d", nil)

		So(s.Len(), ShouldEqual, 4)
		So(s.HasValue(123), ShouldBeTrue)
		s.DeleteIfValue(func(value interface{}) bool {
			if value == nil {
				return true
			}
			return false
		})
		So(s.Len(), ShouldEqual, 3)
		s.DeleteIfKey(func(key interface{}) bool {
			if key == "a" {
				return true
			}
			return false
		})
		So(s.Len(), ShouldEqual, 2)
		So(s.Keys(), ShouldResemble, []interface{}{"b", "c"})
		So(s.Values(), ShouldResemble, []interface{}{321, 456})
	})
}
