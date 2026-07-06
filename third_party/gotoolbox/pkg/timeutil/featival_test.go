package timeutil_test

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wangweihong/gotoolbox/pkg/timeutil"
)

func TestGetLunarSolarTime(t *testing.T) {
	Convey("TestGetSpringFestivalSolarTime", t, func() {
		c := time.Date(2024, 11, 11, 0, 0, 0, 0, time.UTC)

		wt := timeutil.LunarSpringFestivalSolarTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2024-02-10")

		wt = timeutil.LunarDragonBoatFestivalSolarTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2024-06-10")

		wt = timeutil.LunarMidAutumnFestivalSolarTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2024-09-17")

		c = time.Date(2023, 11, 11, 0, 0, 0, 0, time.UTC)

		wt = timeutil.LunarSpringFestivalSolarTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2023-01-22")

		wt = timeutil.LunarDragonBoatFestivalSolarTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2023-06-22")

		wt = timeutil.LunarMidAutumnFestivalSolarTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2023-09-29")
	})
}

func TestGetQingMingTime(t *testing.T) {
	Convey("TestGetQingMingTime", t, func() {
		c := time.Date(2024, 11, 11, 0, 0, 0, 0, time.UTC)
		wt := timeutil.GetQingMingTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2024-04-04")

		c = time.Date(2023, 11, 11, 0, 0, 0, 0, time.UTC)
		wt = timeutil.GetQingMingTime(c)
		So(wt.Format(timeutil.LayoutYearMonthDay), ShouldEqual, "2023-04-05")

	})
}
