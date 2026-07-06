package stringutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/shlex"
)

func BothEmptyOrNone(str1, str2 string) bool {
	return (str1 == "" && str2 == "") || (str1 != "" && str2 != "")
}

func HasAnyPrefix(str string, prefixes ...string) bool {
	if str != "" {
		for _, p := range prefixes {
			if p != "" {
				if strings.HasPrefix(str, p) {
					return true
				}
			}
		}
	}
	return false
}

func HasAnySuffix(str string, suffixes ...string) bool {
	if str != "" {
		for _, p := range suffixes {
			if p != "" {
				if strings.HasSuffix(str, p) {
					return true
				}
			}
		}
	}
	return false
}

func ContainsAny(str string, suffixes ...string) bool {
	if str != "" {
		for _, p := range suffixes {
			if p != "" {
				if strings.Contains(str, p) {
					return true
				}
			}
		}
	}
	return false
}

// use typetuil instead
func PointerToString(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// 打印字符时不转义
// "\n{\"msgtype\": " -- > "\n{\"msgtype\":
func PrintUnescape(p string) {
	fmt.Println(fmt.Sprintf("%#v", p))
}

// TrimAnyPrefix 字符串移除指定前缀(如果有的的话)
func TrimAnyPrefix(str string, prefixes ...string) string {
	if str != "" {
		for _, p := range prefixes {
			str = strings.TrimPrefix(str, p)
		}
	}
	return str
}

// TrimAnyPrefixAndReturn 字符串移除指定前缀，并返回移除的前缀
func TrimAnyPrefixAndReturn(str string, prefixes ...string) ([]string, string) {
	var trims []string
	if str != "" {
		for _, p := range prefixes {
			old := str
			str = strings.TrimPrefix(str, p)
			if old != str {
				trims = append(trims, p)
			}
		}
	}
	return trims, str
}

// TrimAnySuffix 字符串移除指定后缀(如果有的的话)
func TrimAnySuffix(str string, suffixes ...string) string {
	if str != "" {
		for _, p := range suffixes {
			str = strings.TrimPrefix(str, p)
		}
	}
	return str
}

// TrimAnySuffixAndReturn 字符串移除指定后缀，并返回移除的后缀
func TrimAnySuffixAndReturn(str string, prefixes ...string) ([]string, string) {
	var trims []string
	if str != "" {
		for _, p := range prefixes {
			old := str
			str = strings.TrimSuffix(str, p)
			if old != str {
				trims = append(trims, p)
			}
		}
	}
	return trims, str
}

// AddSuffixIfNotHas 如果字符串没有指定后缀则添加
func AddSuffixIfNotHas(str, suffix string) string {
	if !strings.HasSuffix(str, suffix) {
		str += suffix
	}
	return str
}

// MatchIfNotEmptry 如果条件字符串不为空则比较
func MatchIfNotEmptry(str, condition string) bool {
	if condition != "" {
		return str == condition
	}
	return true
}

// AddPrefixIfNotHas 如果字符串没有指定前缀则添加
func AddPrefixIfNotHas(str, prefix string) string {
	if !strings.HasPrefix(str, prefix) {
		str = prefix + str
	}
	return str
}

// RemoveSubBefore 删除子字符串前的所有字符(不包括子字符串)
func RemoveSubBefore(s, str string) string {
	index := strings.Index(s, str)
	if index == -1 {
		// If str is not found, return the original string
		return s
	}
	return s[index:]
}

// RemoveSubAndBefore 删除子字符串之前(包括子字符串)所有字符
func RemoveSubAndBefore(s, str string) string {
	index := strings.Index(s, str)
	if index == -1 {
		// If str is not found, return the original string
		return s
	}
	return s[index+len(str):]
}

// RemoveSubAfter 删除子字符串之后(不包括子字符串)所有字符
func RemoveSubAfter(s, str string) string {
	index := strings.Index(s, str)
	if index == -1 {
		// If str is not found, return the original string
		return s
	}
	return s[:index]
}

// RemoveSubAfter 删除子字符串之后(包括子字符串)的所有字符
func RemoveSubAndAfter(s, str string) string {
	index := strings.Index(s, str)
	if index == -1 {
		// If str is not found, return the original string
		return s
	}
	return s[:index+len(str)]
}

// LenEmptyString 创建指定长度的空字符串
func LenEmptyString(len int) string {
	return strings.Repeat(" ", len)
}

func JoinIf(str string, sep string, elems ...string) string {
	if str == "" {
		return strings.Join(elems, sep)
	}

	if len(elems) == -0 {
		return str
	}

	newElem := make([]string, 0, len(elems)+1)
	newElem = append(newElem, str)
	newElem = append(newElem, elems...)

	return strings.Join(newElem, "/")
}

// ExtractTokens 从s中提取匹配words的字段从最长开始匹配
// words:["RTX", "L", "H", "A", "T", "P", "M"],s: "RTXLHMD"
// 检索出{"RTX", "L", "H", "M"}
func ExtractTokens(words []string, s string) []string {
	// 创建集合并计算最大长度
	wordSet := make(map[string]struct{})
	maxLen := 0
	for _, word := range words {
		wordSet[word] = struct{}{}
		if len(word) > maxLen {
			maxLen = len(word)
		}
	}

	var result []string
	i := 0
	for i < len(s) {
		// 计算当前可能的最大检查长度
		remaining := len(s) - i
		currentMax := maxLen
		if currentMax > remaining {
			currentMax = remaining
		}

		found := false
		// 从最长到最短检查子串
		for j := currentMax; j >= 1; j-- {
			end := i + j
			if end > len(s) {
				end = len(s)
			}
			substr := s[i:end]
			if _, exists := wordSet[substr]; exists {
				result = append(result, substr)
				i += j
				found = true
				break
			}
		}

		if !found {
			i++
		}
	}

	return result
}

func GetIfNotEmpty(str, new string) string {
	if str != "" {
		return str
	}
	return new
}

func ShellParse(rawCmd string) ([]string, error) {
	args, err := shlex.Split(rawCmd)
	if err != nil {
		return nil, err
	}
	var skipSpace []string
	for _, v := range args {
		if strings.TrimSpace(v) == "" {
			continue
		}
		skipSpace = append(skipSpace, v)
	}
	return skipSpace, nil
}

func ShellParse2(rawCmd string) (string, error) {
	skipSpace, err := ShellParse(rawCmd)
	if err != nil {
		return "", err
	}
	return strings.Join(skipSpace, " "), nil
}

// func SplitCommand(input string) []string {
// 	var result []string
// 	var buffer strings.Builder
// 	var inSingleQuote, inDoubleQuote bool
// 	escapeNext := false

// 	for _, r := range input {
// 		switch {
// 		case escapeNext:
// 			buffer.WriteRune(r)
// 			escapeNext = false

// 		case r == '\\':
// 			escapeNext = true

// 		case r == '\'' && !inDoubleQuote:
// 			inSingleQuote = !inSingleQuote

// 		case r == '"' && !inSingleQuote:
// 			inDoubleQuote = !inDoubleQuote

// 		case unicode.IsSpace(r) && !inSingleQuote && !inDoubleQuote:
// 			if buffer.Len() > 0 {
// 				result = append(result, buffer.String())
// 				buffer.Reset()
// 			}

// 		default:
// 			buffer.WriteRune(r)
// 		}
// 	}

// 	if buffer.Len() > 0 {
// 		result = append(result, buffer.String())
// 	}

// 	return result
// }

// ToRightAlign 向右平移N字符
func ToRightAlign(str string, width int) string {
	format := "%-" + strconv.Itoa(width) + "s"
	return fmt.Sprintf(format, str)
}

func SubStringNums(origin string, sub string) int {
	if !strings.Contains(origin, sub) {
		return 0
	}

	return len(strings.Split(origin, sub)) - 1
}

func FirstSet(strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return ""
}