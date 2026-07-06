package mathutil_test

import (
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/mathutil"
)

func TestIntMaxMin(t *testing.T) {
	Convey("TestIntMax|Min", t, func() {
		So(mathutil.IntMin(5, 6), ShouldEqual, 5)
		So(mathutil.IntMax(5, 6), ShouldEqual, 6)

		d := mathutil.IntMax(uint(5), uint(6))
		So(reflect.TypeOf(d), ShouldResemble, reflect.TypeOf(uint(0)))

	})
}
