package randutil_test

import (
	"fmt"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/randutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRandString(t *testing.T) {
	Convey("randString", t, func() {
		fmt.Println(randutil.RandString([]rune("abcdefghijklmnopqrstuvwxyz12345678"), 32))
	})
}
