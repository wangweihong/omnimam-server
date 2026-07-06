package sets_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/sets"
)

func TestGenericSet(t *testing.T) {
	Convey("", t, func() {
		s := sets.NewGenericSet[string]()
		So(s.Has("b"), ShouldBeFalse)

		s.Insert("a")
		s.Insert("b")
		So(s.Len(), ShouldEqual, 2)
		s.InsertIf(func(s string) bool {
			return s == "d"
		}, "d", "e")
		So(s.Len(), ShouldEqual, 3)

		So(s.Delete("noexist").Len(), ShouldEqual, 3)
		s.DeleteIf(func(s string) bool {
			return s == "d"
		})
		So(s.Len(), ShouldEqual, 2)

	})
}

func TestGenericSet_BeSuufix(t *testing.T) {
	Convey("", t, func() {
		s := sets.NewGenericSet[string]()
		s.Insert("M")
		s.Insert("G")
		s.Insert("T")

		So(s.BeSuffix("100T"), ShouldBeTrue)
		So(s.BeSuffix("100G"), ShouldBeTrue)
		So(s.BeSuffix("100B"), ShouldBeFalse)
		So(s.BeSuffix("100"), ShouldBeFalse)
	})
}
