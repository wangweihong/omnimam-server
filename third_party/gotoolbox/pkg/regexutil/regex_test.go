package regexutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/regexutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestExtractNum(t *testing.T) {
	Convey("提取数字", t, func() {
		So(regexutil.ExtractNumbers("xxx_2411周期"), ShouldEqual, "2411")
		So(regexutil.ExtractNumbers("xxx_v3.6.5周期"), ShouldEqual, "365")
	})
}
