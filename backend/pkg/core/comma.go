package core

import "strings"

// CommaSeparatedList 自定义类型，用于解析逗号分隔的字符串为切片
// 注意不能直接定义type CommaSeparatedList []string, 会导致binding不会调用BindUnmarshaler解析
type CommaSeparatedList struct {
	Items []string
}

// UnmarshalParam 实现了gin binding.BindUnmarshaler
func (csl *CommaSeparatedList) UnmarshalParam(text string) error {
	*&csl.Items = strings.Split(string(text), ",")
	return nil
}
