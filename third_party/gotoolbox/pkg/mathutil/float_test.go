package mathutil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/mathutil"
)

func TestFloatToString(t *testing.T) {
	Convey("TestFloatToString", t, func() {
		a := 1.0 / 3.0
		So(mathutil.FloatToString(a, 1), ShouldEqual, "0.3")
		So(mathutil.FloatToString(a, 2), ShouldEqual, "0.33")
		So(mathutil.FloatToString(a, 9), ShouldEqual, "0.333333333")
	})
}
