package sortutil_test

import (
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/typeutil"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/sortutil"
)

func TestSort_MasterBasedSort(t *testing.T) {
	Convey("TestSort_MasterBasedSort", t, func() {
		master := []int{1, 3, 2}
		slave1 := []string{"上海", "北京", "广州"}
		slave2 := []string{"苹果", "香蕉", "橘子"}

		masterI := typeutil.SliceIntToInterfaceType(master...)
		slave1I := typeutil.SliceStringToInterfaceType(slave1...)
		slave2I := typeutil.SliceStringToInterfaceType(slave2...)

		sorter := sortutil.NewMasterBasedSorter(
			masterI,
			func(a, b any) bool {
				ai := typeutil.InterfaceToInt(a)
				bi := typeutil.InterfaceToInt(b)
				return ai > bi
			},
		)
		sorter.Sort(&masterI, &slave1I, &slave2I)

		master = typeutil.SliceInterfaceToIntType(masterI...)
		slave1 = typeutil.SliceInterfaceToStringType(slave1I...)
		slave2 = typeutil.SliceInterfaceToStringType(slave2I...)

		So(master, ShouldResemble, []int{3, 2, 1})
		So(slave1, ShouldResemble, []string{"北京", "广州", "上海"})
		So(slave2, ShouldResemble, []string{"香蕉", "橘子", "苹果"})
	})
}
