package timeutil_test

import (
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/timeutil"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSdkTime_ParseTime(t *testing.T) {
	Convey("TestSdkTime_ParseTime", t, func() {

		t1, err := timeutil.ParseTime("\"2022-01-30T12:34:56Z\"")
		So(err, ShouldBeNil)
		So(t1.Equal(time.Time{}), ShouldBeFalse)

		_, err = timeutil.ParseTime("\"2022-01-30T12:34212:56Z\"")
		So(err, ShouldNotBeNil)

		t2, err := timeutil.ParseTime("\"2022-01-30T12:34:56\"")
		So(err, ShouldBeNil)
		So(t2.Equal(time.Time{}), ShouldBeFalse)

		t3, err := timeutil.ParseTime("\"2022-01-30 12:34:56\"")
		So(err, ShouldBeNil)
		So(t3.Equal(time.Time{}), ShouldBeFalse)

		t4, err := timeutil.ParseTime("\"2022-01-30T12:34:56+08:00\"")
		So(err, ShouldBeNil)
		So(t4.Equal(time.Time{}), ShouldBeFalse)

		t5, err := timeutil.ParseTime("\"2022-01-30T12:34:56.123456Z\"")
		So(err, ShouldBeNil)
		So(t5.Equal(time.Time{}), ShouldBeFalse)

	})

}

func TestFormatDuration(t *testing.T) {
	Convey("format duration", t, func() {
		So(timeutil.FormatDuration(20*time.Second), ShouldEqual, "20秒")
		So(timeutil.FormatDuration(225*time.Second), ShouldEqual, "3分45秒")
		So(timeutil.FormatDuration(3925*time.Second), ShouldEqual, "1小时5分25秒")
	})
}
