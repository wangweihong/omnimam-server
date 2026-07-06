package timeutil

import (
	"fmt"
	"time"

	"github.com/Lofanmi/chinese-calendar-golang/calendar"
)

// LunarSpringFestivalSolarTime 获取春节新历时间
func LunarSpringFestivalSolarTime(t time.Time) time.Time {
	year := t.Year()
	c := calendar.ByLunar(int64(year), 1, 1, 0, 0, 0, IsLeapYear(year))
	ymd := fmt.Sprintf("%v-%02d-%02d", c.Solar.GetYear(), c.Solar.GetMonth(), c.Solar.GetDay())
	wt, _ := ParseTime(ymd)
	return wt
}

// LunarDragonBoatFestivalSolarTime 获取端午新历时间
func LunarDragonBoatFestivalSolarTime(t time.Time) time.Time {
	year := t.Year()
	c := calendar.ByLunar(int64(year), 5, 5, 0, 0, 0, IsLeapYear(year))
	ymd := fmt.Sprintf("%v-%02d-%02d", c.Solar.GetYear(), c.Solar.GetMonth(), c.Solar.GetDay())
	wt, _ := ParseTime(ymd)
	return wt
}

// LunarMidAutumnFestivalSolarTime 获取中秋新历时间
func LunarMidAutumnFestivalSolarTime(t time.Time) time.Time {
	year := t.Year()
	c := calendar.ByLunar(int64(year), 8, 15, 0, 0, 0, IsLeapYear(year))
	ymd := fmt.Sprintf("%v-%02d-%02d", c.Solar.GetYear(), c.Solar.GetMonth(), c.Solar.GetDay())
	wt, _ := ParseTime(ymd)
	return wt
}

func GetQingMingTime(t time.Time) time.Time {
	// 如果是闰年，清明节在4月4日；否则在4月5日
	day := 5
	if IsLeapYear(t.Year()) {
		day = 4
	}
	return time.Date(t.Year(), time.April, day, 0, 0, 0, 0, time.Local)
}
