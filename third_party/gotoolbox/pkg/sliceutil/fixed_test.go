package sliceutil_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
)

func TestFixedSlice(t *testing.T) {
	Convey("TestFixedSlice", t, func() {
		// fmt.Println(0 % 3)
		// fmt.Println(1 % 3)
		// fmt.Println(2 % 3)
		// fmt.Println(3 % 3)
		// fmt.Println(4 % 3)

		fs := sliceutil.NewFixedSlice[int](3)
		// 添加元素
		fs.Append(1)
		fs.Append(2)
		fs.Append(3) // [1 2 3]
		So(fs.GetAll(), ShouldResemble, []int{1, 2, 3})
		fs.Append(4)
		So(fs.GetAll(), ShouldResemble, []int{2, 3, 4})
		fs.Append(5)
		fs.Append(6)
		So(fs.GetAll(), ShouldResemble, []int{4, 5, 6})
	})
}
