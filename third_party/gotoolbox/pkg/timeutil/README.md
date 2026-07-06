
# timeutil — 时间工具

**功能**：时间解析、格式化、农历节日计算（春节、端午、中秋、清明）。

| 函数/方法 | 说明 |
|---|---|
| `ParseTime(data) (time.Time, error)` | 解析多种格式时间字符串 |
| `SdkTime.UnmarshalJSON(data) error` | 自定义 JSON 时间反序列化 |
| `SdkTime.MarshalJSON() ([]byte, error)` | 自定义 JSON 时间序列化 |
| `LunarSpringFestivalSolarTime(t) time.Time` | 获取农历春节对应的公历日期 |
| `LunarDragonBoatFestivalSolarTime(t) time.Time` | 获取农历端午对应的公历日期 |
| `LunarMidAutumnFestivalSolarTime(t) time.Time` | 获取农历中秋对应的公历日期 |
| `GetQingMingTime(t) time.Time` | 获取清明节日期 |
| `IsLeapYear(year) bool` | 判断闰年 |
[← 返回包列表](../../README.md)
