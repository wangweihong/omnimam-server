package sliceutil

import (
	"sort"
	"strings"
)

type StringSlice []string

func (m StringSlice) String() string {
	if m == nil {
		return "nil"
	}
	s := "["
	for i, v := range m {
		s += v
		if i != len(m)-1 {
			s += ","
		}
	}
	s += "]"
	return s
}

func (m StringSlice) DeepCopy() StringSlice {
	o := make([]string, 0, len(m))
	o = append(o, m...)
	return o
}

func (m StringSlice) Append(target ...string) StringSlice {
	if m == nil {
		o := make([]string, 0, len(target))
		return append(o, target...)
	}

	return append(m, target...)
}

// HasRepeat slice has repeated data
func (m StringSlice) HasRepeat() bool {
	if m != nil {
		s := make(map[string]struct{})
		for _, v := range m {
			if _, exist := s[v]; exist {
				return true
			}
			s[v] = struct{}{}
		}
	}

	return false
}

// GetRepeat find slice repeat data and repeat num
func (m StringSlice) GetRepeat() (map[string]int, bool) {
	if m != nil {
		var r map[string]int
		s := make(map[string]struct{})
		for _, v := range m {
			if _, exist := s[v]; exist {
				if r == nil {
					r = make(map[string]int)
				}
				num, _ := r[v]
				if num == 0 {
					num = 1
				}
				num++
				r[v] = num
			}
			s[v] = struct{}{}
		}

		return r, !(len(r) == 0)
	}

	return nil, false
}

// SortDesc Descending sort
func (m StringSlice) SortDesc() []string {
	if m != nil {
		sort.Slice(m, func(i, j int) bool {
			return m[i] > m[j]
		})
		return m
	}

	return nil
}

// Sort Ascascending sort
func (m StringSlice) SortAsc() []string {
	if m != nil {
		sort.Slice(m, func(i, j int) bool {
			return m[i] < m[j]
		})
		return m
	}

	return nil
}

func (m StringSlice) HasEmpty() (int, bool) {
	if m != nil {
		var eN int
		for _, v := range m {
			if v == "" {
				eN++
			}
		}
		return eN, eN != 0
	}

	return 0, false
}

func (m StringSlice) Cut(data string) []string {
	var index int = -1
	for i, v := range m {
		if v == data {
			index = i
			break
		}
	}

	if index == -1 {
		return m
	}

	return append(m[:index], m[index+1:]...)
}

func (m StringSlice) FallBehind(data string) []string {
	n := m.Cut(data)
	return append(n, data)
}

func (m StringSlice) TrimSpace() []string {
	if m == nil {
		return nil
	}

	n := make([]string, 0, len(m))
	for _, v := range m {
		if strings.TrimSpace(v) != "" {
			n = append(n, v)
		}
	}
	return n
}

// RemoveIf 移除符合数组中某个条件的元素
func (m StringSlice) RemoveIf(condition func(string) bool) []string {
	if m == nil {
		return nil
	}
	// 第一次迭代：标记要删除的元素
	marked := make([]bool, len(m))
	for i, num := range m {
		if condition(num) {
			marked[i] = true
		}
	}

	// 第二次迭代：删除标记为 true 的元素
	result := make([]string, 0, len(m))
	for i, num := range m {
		if !marked[i] {
			result = append(result, num)
		}
	}

	return result
}

// AppendIf 追加符合条件的元素到数组
func (m StringSlice) AppendIf(condition func(string) bool, sl []string) []string {
	if sl == nil {
		return m
	}

	result := make([]string, 0, len(m))
	for _, str := range m {
		result = append(result, str)
	}

	for _, s := range sl {
		if condition(s) {
			result = append(result, s)
		}
	}
	return result
}

// Index 查找某个值的索引
func (m StringSlice) Index(str string) int {
	for k, v := range m {
		if v == str {
			return k
		}
	}

	return -1
}

// MoveFirst 移动某个元素到队首
func (m StringSlice) MoveFirst(str string) []string {
	if str == "" {
		return m
	}
	index := m.Index(str)
	if index == -1 {
		return m
	}

	result := make([]string, 0, len(m)+1)
	result = append(result, str)
	result = append(result, m[:index]...)
	result = append(result, m[index+1:]...)

	return result
}

func (m StringSlice) Max() string {
	max := ""
	for _, v := range m {
		if v > max {
			max = v
		}
	}

	return max
}

func (m StringSlice) UpdateValueWithPrefix(prefix string) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = prefix + v
	}
}

func (m StringSlice) UpdateValueWithSuffix(suffix string) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = v + suffix
	}
}

func (m StringSlice) UpdateValueWithPrefixRemove(prefix string) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = strings.TrimPrefix(v, prefix)
	}
}

func (m StringSlice) UpdateValueWithSuffixRemove(suffix string) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = strings.TrimSuffix(v, suffix)
	}
}

func (m StringSlice) LenExceedCount(leng int) int {
	if m == nil {
		return 0
	}
	var count int
	for _, v := range m {
		if len(v) > leng {
			count++
		}
	}
	return count
}
