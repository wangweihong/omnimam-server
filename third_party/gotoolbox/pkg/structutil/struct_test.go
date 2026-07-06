package structutil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/structutil"
)

func TestInitializeStruct(t *testing.T) {
	Convey("TestNewSequentialList_Filter", t, func() {
		type Example struct {
			A int
			B string
		}
		var i int
		So(structutil.InitializeStruct(&Example{}), ShouldResemble, Example{})
		So(structutil.InitializeStruct(Example{}), ShouldResemble, Example{})
		So(structutil.InitializeStruct(i), ShouldBeNil)
	})
}
