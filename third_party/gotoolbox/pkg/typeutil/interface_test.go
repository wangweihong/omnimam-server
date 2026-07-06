package typeutil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/typeutil"
)

func TestInterfaceTo(t *testing.T) {
	Convey("TestNewSequentialList_Filter", t, func() {
		So(typeutil.InterfaceToInt(nil), ShouldEqual, 0)
		So(typeutil.InterfaceToInt(6), ShouldEqual, 6)
		So(typeutil.InterfaceToString(nil), ShouldEqual, "")
		So(typeutil.InterfaceToString("b"), ShouldEqual, "b")
		So(typeutil.InterfaceToMapStringInterface(nil), ShouldEqual, map[string]any{})

		So(typeutil.SliceInterfaceToIntType(), ShouldEqual, []int{})
		So(typeutil.SliceInterfaceToIntType(1, 3, "d"), ShouldEqual, []int{1, 3, 0})
		So(typeutil.SliceInterfaceToStringType(), ShouldEqual, []string{})
	})
}
