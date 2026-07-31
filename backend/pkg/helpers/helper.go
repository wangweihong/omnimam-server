package helpers

import "strings"

// ApplyString 在可选更新值非空时覆盖目标字符串。
func ApplyString(target *string, value *string) {
	if value != nil {
		*target = *value
	}
}

// ApplyInt 在可选更新值非空时覆盖目标整数。
func ApplyInt(target *int, value *int) {
	if value != nil {
		*target = *value
	}
}

// MatchesKeyword 判断关键字是否匹配任一候选字符串，匹配不区分大小写。
func MatchesKeyword(keyword string, values ...string) bool {
	if keyword == "" {
		return true
	}
	keyword = strings.ToLower(keyword)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true
		}
	}
	return false
}
