package pathutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/pathutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPathsSort(t *testing.T) {
	Convey("path depth sort", t, func() {
		Convey("文件-根目录-子目录", func() {
			paths := []string{
				"d1/",
				"d1/d2/",
				"d1/f2",
				"d1/f3",
				"d1/d2/ff1",
				"d1/d2/d3/",
			}

			d := []string{
				"d1/f2",
				"d1/f3",
				"d1/",
				"d1/d2/ff1",
				"d1/d2/",
				"d1/d2/d3/",
			}

			So(pathutil.ToFileFirstDepthPaths(paths).Sort().ToSlice(), ShouldResemble, d)
		})
		Convey("根目录最后", func() {
			paths := []string{
				"d1/",
				"d1/d2/",
				"d1/f2",
				"d1/f3",
				"d1/d2/ff1",
				"d1/d2/d3/",
			}

			d := []string{
				"d1/d2/d3/",
				"d1/d2/ff1",
				"d1/d2/",
				"d1/f2",
				"d1/f3",
				"d1/",
			}

			So(pathutil.ToDirLastDepthPaths(paths).Sort().ToSlice(), ShouldResemble, d)
		})

	})
}
