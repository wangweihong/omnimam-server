package mathutil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/mathutil"
)

func TestMin(t *testing.T) {
	Convey("", t, func() {
		So(mathutil.Max(1, 2), ShouldEqual, 2)
		So(mathutil.Min(1, 2), ShouldEqual, 1)
	})
}
