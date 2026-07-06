package timeutil

import (
	"fmt"
	"strings"
	"time"
)

const (
	LayoutDateTimeWithMillisecondsAndTZOffset = "2006-01-02T15:04:05.000-0700"
	LayoutYearMonthDay                        = "2006-01-02"
	// 时间精确到纳秒进度,并包含时区信息。 -0700: 时区偏移（表示 UTC-7 小时）, MST:时区缩写
	LayoutFullDateTimeWithNanosecondsAndTZName = "2006-01-02 15:04:05.999999999 -0700 MST"
	LayoutISO8601BasicDateTime                 = "2006-01-02T15:04:05"
	LayoutSimpleDateTime                       = "2006-01-02 15:04:05"
	// +08:00表示时区偏移量
	LayoutISO8601DateTimeWithTZOffset = "2006-01-02T15:04:05+08:00"
	// z表示UTC时区
	LayoutISO8601DateTimeWithZulu = "2006-01-02T15:04:05Z"
)

type SdkTime time.Time

func (t *SdkTime) UnmarshalJSON(data []byte) error {
	tmp := strings.Trim(string(data[:]), "\"")

	now, err := time.ParseInLocation(`2006-01-02T15:04:05Z`, tmp, time.UTC)
	if err == nil {
		*t = SdkTime(now)
		return err
	}

	now, err = time.ParseInLocation(`2006-01-02T15:04:05`, tmp, time.UTC)
	if err == nil {
		*t = SdkTime(now)
		return err
	}

	now, err = time.ParseInLocation(`2006-01-02 15:04:05`, tmp, time.UTC)
	if err == nil {
		*t = SdkTime(now)
		return err
	}

	now, err = time.ParseInLocation(`2006-01-02T15:04:05+08:00`, tmp, time.UTC)
	if err == nil {
		*t = SdkTime(now)
		return err
	}

	now, err = time.ParseInLocation(time.RFC3339, tmp, time.UTC)
	if err == nil {
		*t = SdkTime(now)
		return err
	}

	now, err = time.ParseInLocation(time.RFC3339Nano, tmp, time.UTC)
	if err == nil {
		*t = SdkTime(now)
		return err
	}

	//"2024-07-18 12:30:57.4718552 +0800 CST"
	now, err = time.Parse(LayoutFullDateTimeWithNanosecondsAndTZName, tmp)
	if err == nil {
		*t = SdkTime(now)
		return err
	}

	return err
}

func (t SdkTime) MarshalJSON() ([]byte, error) {
	rs := []byte(fmt.Sprintf(`"%s"`, t.String()))
	return rs, nil
}

func (t SdkTime) String() string {
	return time.Time(t).Format(`2006-01-02T15:04:05Z`)
}

func ParseTime(data string) (time.Time, error) {
	tmp := strings.Trim(data[:], "\"")

	now, err := time.ParseInLocation(`2006-01-02T15:04:05Z`, tmp, time.UTC)
	if err == nil {
		return now, nil
	}

	now, err = time.ParseInLocation(`2006-01-02T15:04:05`, tmp, time.UTC)
	if err == nil {
		return now, nil
	}

	now, err = time.ParseInLocation(`2006-01-02 15:04:05`, tmp, time.UTC)
	if err == nil {
		return now, nil
	}

	now, err = time.ParseInLocation(`2006-01-02T15:04:05+08:00`, tmp, time.UTC)
	if err == nil {
		return now, nil
	}

	now, err = time.ParseInLocation(time.RFC3339, tmp, time.UTC)
	if err == nil {
		return now, nil
	}

	now, err = time.ParseInLocation(time.RFC3339Nano, tmp, time.UTC)
	if err == nil {
		return now, nil
	}

	//"2024-07-18 12:30:57.4718552 +0800 CST"
	layout := "2006-01-02 15:04:05.999999999 -0700 MST"
	now, err = time.Parse(layout, data)
	if err == nil {
		return now, nil
	}

	layout2 := "2006-01-02T15:04:05.000-0700"
	now, err = time.Parse(layout2, data)
	if err == nil {
		return now, nil
	}

	layout3 := "2006-01-02 15:04:05.999999999 -0700 MST m=+0.000000000"
	now, err = time.Parse(layout3, data)
	if err == nil {
		return now, nil
	}

	layout4 := "2006-01-02"
	now, err = time.Parse(layout4, data)
	if err == nil {
		return now, nil
	}

	return time.Time{}, err
}

func MustParseTime(data string) time.Time {
	t, _ := ParseTime(data)
	return t
}

func FormatDuration(duration time.Duration) string {
	seconds := int(duration.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainingSeconds := seconds % 60

	var result string

	if hours > 0 {
		result += fmt.Sprintf("%d小时", hours)
	}
	if minutes > 0 || hours > 0 {
		result += fmt.Sprintf("%d分", minutes)
	}
	result += fmt.Sprintf("%d秒", remainingSeconds)

	return result
}

func FormatDurationSeconds(duration time.Duration) string {
	seconds := int64(time.Second)
	if int64(duration) > seconds {
		return fmt.Sprintf("%vs", duration.Seconds())
	}

	return fmt.Sprintf("%vms", duration.Milliseconds())
}

func ToTime(timestamp int64) time.Time {
	if isNano(timestamp) {
		// time.Now().UnixNano()保留毫秒等单位
		return time.Unix(0, timestamp)
	}
	// time.Now().Unix只保留秒以上的数据
	return time.Unix(timestamp, 0)

}

func isNano(timestamp int64) bool {
	// 假设合理的 Unix 时间戳范围是 1970 年到 3000 年
	// Unix 时间戳的范围是 [0, 32503680000]
	if timestamp > 99999999999 {
		return true
	}
	return false
}

// 获取某个时间所在周的周一
func GetTimeWeekMonday(t time.Time) time.Time {
	weekday := int(t.Weekday())

	// 处理星期天的情况，因为 Go 的 Weekday 返回星期天为 0
	if weekday == 0 {
		weekday = 7
	}

	monday := t.AddDate(0, 0, -weekday+1)
	return monday
}

// 判断时间是否属于去年
func IsFromLastYear(t time.Time) bool {
	now := time.Now()

	// 计算去年的开始和结束时间
	lastYearStart := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, time.UTC)
	lastYearEnd := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	// 检查 t 是否在去年的范围内
	return t.After(lastYearStart) && t.Before(lastYearEnd)
}

// IsDateWithinLastWeek
func IsDateWithinLastWeek(t1, t2 time.Time) bool {
	t1 = t1.Truncate(24 * time.Hour)

	oneWeekAgo := t1.AddDate(0, 0, -7)
	t2 = t2.Truncate(24 * time.Hour)
	return t2.After(oneWeekAgo) && t2.Before(t1)
}

// GetMondays 获取当前时间到指定时间的所有周一时间
func GetMondays(startDate time.Time) []time.Time {
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	weekday := int(startDate.Weekday())

	// 处理星期天的情况，因为 Go 的 Weekday 返回星期天为 0
	if weekday == 0 {
		weekday = 7
	}

	monday := startDate.AddDate(0, 0, -weekday+1)
	var mondays []time.Time
	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for monday.Before(now) || monday.Equal(now) {
		mondays = append(mondays, monday)
		monday = monday.AddDate(0, 0, 7)
	}

	return mondays
}

func GetNDayTime(t time.Time, n int) time.Time {
	pt := t.AddDate(0, 0, n)
	return pt
}

func ToYearMonthDay(t time.Time) string {
	return t.Format("2006-01-02")
}

func IsDayInCurrentWeekAndMonth(day string, nowday string) (bool, bool, error) {
	date, err := ParseTime(day)
	if err != nil {
		return false, false, err
	}
	now, err := ParseTime(nowday)
	if err != nil {
		return false, false, err
	}
	isWeek, isMonth := IsTimeInCurrentWeekAndMonth(date, now)
	return isWeek, isMonth, nil
}

func IsTimeInCurrentWeekAndMonth(date time.Time, now time.Time) (bool, bool) {
	isInMonth := now.Year() == date.Year() && now.Month() == date.Month()
	currentWeekDay := int(now.Weekday())
	if currentWeekDay == 0 {
		currentWeekDay = 7
	}
	weekStart := now.AddDate(0, 0, -currentWeekDay+1)
	weekEnd := weekStart.AddDate(0, 0, 6)
	isInWeek := date == weekStart || date == weekEnd || date.After(weekStart) && date.Before(weekEnd.AddDate(0, 0, 1))

	return isInWeek, isInMonth
}

// GetWeekAllDays 	 获取当前周的日期
func GetWeekAllDays(date time.Time) []string {
	weekStart := date.AddDate(0, 0, -int(date.Weekday()-1)) // 当前周的周一
	var weekDates []string
	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		weekDates = append(weekDates, day.Format("2006-01-02"))
	}
	return weekDates
}

// GetWeekWorkDays 	 获取当前周的工作日日期
func GetWeekWorkDays(date time.Time) []string {
	weekStart := date.AddDate(0, 0, -int(date.Weekday()-1)) // 当前周的周一
	var weekDates []string
	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			weekDates = append(weekDates, day.Format("2006-01-02"))
		}
	}
	return weekDates
}

// GetMonthAllDays 	 获取当前月的日期
func GetMonthAllDays(date time.Time) []string {
	year := date.Year()
	month := date.Month()

	// 判断闰年
	isLeapYear := year%4 == 0 && (year%100 != 0 || year%400 == 0)

	daysInMonth := 31
	switch month {
	case time.April, time.June, time.September, time.November:
		daysInMonth = 30
	case time.February:
		if isLeapYear {
			daysInMonth = 29
		} else {
			daysInMonth = 28
		}
	}

	dates := make([]string, 0, daysInMonth)
	for day := 1; day <= daysInMonth; day++ {
		dates = append(dates, time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
	}
	return dates
}

// IsLeapYear 是否为闰年
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func GetMonthWorkDays(date time.Time) []string {
	year := date.Year()
	month := date.Month()

	// 判断闰年
	isLeapYear := IsLeapYear(year)

	daysInMonth := 31
	switch month {
	case time.April, time.June, time.September, time.November:
		daysInMonth = 30
	case time.February:
		if isLeapYear {
			daysInMonth = 29
		} else {
			daysInMonth = 28
		}
	}

	dates := make([]string, 0, daysInMonth)
	for day := 1; day <= daysInMonth; day++ {
		t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			dates = append(dates, time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
		}
	}
	return dates
}

// DateInRange 判断某个日期是否在给定的起始和结束日期之间，包括边界
func DateInRange(date, start, end string) (bool, error) {
	const layout = "2006-01-02" // 日期格式化字符串

	// 解析字符串日期
	d, err := time.Parse(layout, date)
	if err != nil {
		return false, fmt.Errorf("invalid date format for date: %v", err)
	}
	s, err := time.Parse(layout, start)
	if err != nil {
		return false, fmt.Errorf("invalid date format for start: %v", err)
	}
	e, err := time.Parse(layout, end)
	if err != nil {
		return false, fmt.Errorf("invalid date format for end: %v", err)
	}

	// 判断日期是否在范围内（包括边界）
	return (d.Equal(s) || d.After(s)) && (d.Equal(e) || d.Before(e)), nil
}

// GetLastDayOfWeekOrMonth 如果本周日和今天不在同一个月，则获取本周月最后一天的日期，否则本周日的日期
func GetLastDayOfWeekOrMonth(now time.Time) time.Time {
	weekday := now.Weekday()
	daysToSunday := 7 - int(weekday)
	thisSunday := now.AddDate(0, 0, daysToSunday)
	if thisSunday.Month() == now.Month() {
		daysInMonth := 31
		currentMonth := now.Month()
		currentYear := now.Year()
		switch currentMonth {
		case time.April, time.June, time.September, time.November:
			daysInMonth = 30
		case time.February:
			if currentYear%4 == 0 && (currentYear%100 != 0 || currentYear%400 == 0) {
				daysInMonth = 29
			} else {
				daysInMonth = 28
			}
		}
		return time.Date(currentYear, currentMonth, daysInMonth, 0, 0, 0, 0, time.UTC)
	}
	return thisSunday
}

func DaysSinceMondayAndFirstOfMonth(now time.Time) (int, int) {
	wds, mds := TimesSinceMondayAndFirstOfMonth(now)
	return len(wds), len(mds)
}

// TimesSinceMondayAndFirstOfMonth 计算某日到周一,以及某日到本月1号的所有时间
func TimesSinceMondayAndFirstOfMonth(now time.Time) ([]time.Time, []time.Time) {
	weekday := now.Weekday()
	daysSinceMonday := 0
	if weekday != time.Monday {
		daysSinceMonday = int(weekday - time.Monday)
		if daysSinceMonday < 0 {
			daysSinceMonday += 7
		}
	}
	timesToMonday := []time.Time{}
	for i := daysSinceMonday; i >= 0; i-- {
		timesToMonday = append(timesToMonday, now.AddDate(0, 0, -i))
	}

	daysSinceFirst := now.Day() - 1
	timesToFirst := []time.Time{}
	for i := daysSinceFirst; i >= 0; i-- {
		timesToFirst = append(timesToFirst, now.AddDate(0, 0, -i))
	}

	return timesToMonday, timesToFirst
}
