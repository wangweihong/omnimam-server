package sliceutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/sliceutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIsSliceOfStructs(t *testing.T) {
	Convey("TestIsSliceOfStructs", t, func() {

		type Example struct {
			A string
		}
		So(sliceutil.IsSliceOfStructs([]string{"a", "b"}), ShouldBeFalse)
		So(sliceutil.IsSliceOfStructs([]Example{{A: "a"}, {A: "b"}}), ShouldBeTrue)
		So(sliceutil.IsSliceOfStructs("123"), ShouldBeFalse)
	})
}
