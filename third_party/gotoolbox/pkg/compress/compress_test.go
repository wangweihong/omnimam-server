package compress_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/compress"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCompressRar(t *testing.T) {
	Convey("rar", t, func() {
		Convey("find file name", func() {
			dirPath, err := compress.NewExtractor(".rar").FindDirPathInTar("./testdata/self-cognition.rar", "README.md")
			So(err, ShouldBeNil)
			So(dirPath, ShouldEqual, "self-cognition/")
		})
		Convey("find file format", func() {
			dirPath, err := compress.NewExtractor(".rar").FindFileFormatPathInTar("./testdata/self-cognition.rar", ".md", ".json")
			So(err, ShouldBeNil)
			So(dirPath, ShouldEqual, "self-cognition/")
		})
	})
}
