package sliceutil

import (
	"sort"
)

type IntSlice []int

func NewIntSlice(e ...int) IntSlice {
	return e
}

func (m IntSlice) DeepCopy() IntSlice {
	o := make([]int, 0, len(m))
	o = append(o, m...)
	return o
}

func (m IntSlice) Append(target ...int) IntSlice {
	if m == nil {
		o := make([]int, 0, len(target))
		return append(o, target...)
	}

	return append(m, target...)
}

// HasRepeat slice has repeated data
func (m IntSlice) HasRepeat() bool {
	if m != nil {
		s := make(map[int]struct{})
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
func (m IntSlice) GetRepeat() (map[int]int, bool) {
	if m != nil {
		var r map[int]int
		s := make(map[int]struct{})
		for _, v := range m {
			if _, exist := s[v]; exist {
				if r == nil {
					r = make(map[int]int)
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
func (m IntSlice) SortDesc() []int {
	if m != nil {
		sort.Slice(m, func(i, j int) bool {
			return m[i] > m[j]
		})
		return m
	}

	return nil
}

// Sort Ascascending sort
func (m IntSlice) SortAsc() []int {
	if m != nil {
		sort.Slice(m, func(i, j int) bool {
			return m[i] < m[j]
		})
		return m
	}

	return nil
}

// RemoveIf 移除符合数组中某个条件的元素
func (m IntSlice) RemoveIf(condition func(int) bool) []int {
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
	result := make([]int, 0, len(m))
	for i, num := range m {
		if !marked[i] {
			result = append(result, num)
		}
	}

	return result
}

// AppendIf 追加符合莫格条件的元素到数组
func (m IntSlice) AppendIf(condition func(int) bool, sl []int) []int {
	if sl == nil {
		return m
	}

	result := make([]int, 0, len(m))
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

func (m IntSlice) Max() int {
	max := 0
	for _, v := range m {
		if v > max {
			max = v
		}
	}

	return max
}

func (m IntSlice) MoveFront(condition func(int) bool) []int {
	return moveElementToFrontIf(m, condition)
}

func (m IntSlice) MoveAfter(condition func(int) bool) []int {
	return moveElementToAfterIf(m, condition)
}

func moveElementToFrontIf(arr []int, condition func(int) bool) []int {
	for i := 0; i < len(arr); i++ {
		if condition(arr[i]) {
			// 找到满足条件的元素，将其移动至切片的开头
			// 先将满足条件的元素移动到切片的第一个位置
			temp := arr[i]
			copy(arr[1:i+1], arr[:i])
			arr[0] = temp
			break
		}
	}
	return arr
}

func moveElementToAfterIf(arr []int, condition func(int) bool) []int {
	for i := 0; i < len(arr); i++ {
		if condition(arr[i]) {
			// 找到满足条件的元素，将其移动至切片的开头
			// 先将满足条件的元素移动到切片的第一个位置
			temp := arr[i]
			arr = append(arr[0:i], arr[i+1:]...)
			//copy(arr[1:i+1], arr[:i])
			arr = append(arr, temp)
			break
		}
	}
	return arr
}

func moveElementToFront(arr []int, i int) []int {
	temp := arr[i]
	copy(arr[1:i+1], arr[:i])
	arr[0] = temp
	return arr
}

// MergeDifferentElementsFromEnd m/b从最后一个元素开始比较，直到找到和b不同的元素的索引。m中不同的部分和b合并
func (m IntSlice) MergeDifferentElementsFromEnd(b IntSlice) IntSlice {
	lenM, lenB := len(m), len(b)
	var offset int

	for {
		iM, iB := lenM-offset-1, lenB-offset-1

		if iM < 0 || iB < 0 {
			break
		}

		if m[iM] != b[iB] {
			break
		}
		offset += 1
	}

	if offset == lenM || offset == lenB {
		if lenM > lenB {
			return append(m[:lenM-lenB], b...)
		}
		return b
	}

	result := append(m[0:lenM-offset], b...)
	return result
}
