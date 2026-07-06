package cryptoutil_test

import (
	"fmt"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/cryptoutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMd5(t *testing.T) {
	Convey("md5", t, func() {
		d, err := cryptoutil.Md5Encrypt("admin")
		So(err, ShouldBeNil)
		fmt.Println(d)
	})
}
