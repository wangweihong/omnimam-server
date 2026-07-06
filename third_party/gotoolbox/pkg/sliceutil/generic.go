package sliceutil

import (
	"fmt"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

// Unique 去重
func Unique[T comparable](slice []T) []T {
	ts := sets.NewGenericSet[T]()

	ds := make([]T, 0, len(slice))
	for _, v := range slice {
		if !ts.Has(v) {
			ts.Insert(v)
			ds = append(ds, v)
		}
	}
	return ds
}

// Push 向切片添加一个元素
func Push[T any](slice []T, element T) []T {
	return append(slice, element)
}

// Pop 从切片中移除并返回最后一个元素
func Pop[T any](slice []T) ([]T, T, bool) {
	if len(slice) == 0 {
		var zero T
		return slice, zero, false
	}
	lastIndex := len(slice) - 1
	lastElement := slice[lastIndex]
	slice = slice[:lastIndex]
	return slice, lastElement, true
}

// Len 返回切片的长度
func Len[T any](slice []T) int {
	return len(slice)
}

// Delete 删除切片中的指定元素
func Delete[T comparable](slice []T, element T) []T {
	for i, v := range slice {
		if v == element {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// DeleteIf 根据条件删除切片中的元素
func DeleteIf[T any](slice []T, condition func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if !condition(v) {
			result = append(result, v)
		}
	}
	return result
}

// Filter 根据条件过滤切片，返回符合条件的元素
func Filter[T any](slice []T, fn func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Has 检查切片中是否包含指定元素
func Has[T comparable](slice []T, element T) bool {
	ts := sets.NewGenericSet[T](slice...)
	return ts.Has(element)
}

// ElemNum 切片中某元素的数量
func ElemNum[T comparable](slice []T, elem T) int {
	var num int
	for _, v := range slice {
		if v == elem {
			num++
		}
	}
	return num
}

// ElemNumIf 切片中满足指定函数的数量
func ElemNumIf[T comparable](slice []T, fn func(T) bool) int {
	var num int
	for _, v := range slice {
		if fn(v) {
			num++
		}
	}
	return num
}

// IndexOf 返回切片中第一个匹配元素的索引，未找到返回 -1
func IndexOf[T comparable](slice []T, element T) int {
	for i, v := range slice {
		if v == element {
			return i
		}
	}
	return -1
}

// IndexOfIf 返回切片中第一个匹配函数的元素的索引，未找到返回 -1
func IndexOfIf[T comparable](slice []T, fn func(T) bool) int {
	for i, v := range slice {
		if fn(v) {
			return i
		}
	}
	return -1
}

// Reverse 反转切片
func Reverse[T any](slice []T) []T {
	n := len(slice)
	reversed := make([]T, n)
	for i := 0; i < n; i++ {
		reversed[i] = slice[n-i-1]
	}
	return reversed
}

// Chunk 将切片分成多个小块，每块大小为 chunkSize
func Chunk[T any](slice []T, chunkSize int) [][]T {
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// Intersection 返回两个切片的交集
func Intersection[T comparable](slice1, slice2 []T) []T {
	ts1 := sets.NewGenericSet[T](slice1...)
	ts2 := sets.NewGenericSet[T](slice2...)
	return ts1.Intersection(ts2).UnsortedList()
}

// Union 返回两个切片的并集
func Union[T comparable](slice1, slice2 []T) []T {
	ts1 := sets.NewGenericSet[T](slice1...)
	ts2 := sets.NewGenericSet[T](slice2...)
	return ts1.Union(ts2).UnsortedList()
}

// Max 返回切片中的最大元素
func Max[T any](slice []T, biggerThan func(a, b T) bool) T {
	if len(slice) == 0 {
		var zero T
		return zero
	}
	max := slice[0]
	for _, v := range slice[1:] {
		if biggerThan(v, max) {
			max = v
		}
	}
	return max
}

func Min[T any](slice []T, litterThan func(a, b T) bool) T {
	if len(slice) == 0 {
		var zero T
		return zero
	}
	min := slice[0]
	for _, v := range slice[1:] {
		if litterThan(v, min) {
			min = v
		}
	}
	return min
}

// 危险操作：end>len(s) 时 panic
// s := []int{1,2,3}
// fmt.Println(s[:5]) // panic: runtime error: slice bounds out of range
// 安全操作：使用 sliceContent
//
//	fmt.Println(sliceContent(s, 5)) // 返回 [1 2 3]
func SliceContent[T any](s []T, end int) []T {
	if len(s) > end {
		return s[:end]
	}
	return s
}

func ZeroCount[T comparable](s []T) int {
	var zero T
	if len(s) == 0 {
		return 0
	}
	var count int
	for _, v := range s {
		if zero == v {
			count++
		}
	}
	return count
}

func NewPointerSlice[T any](ts []T) []*T {
	pts := make([]*T, 0, len(ts))
	for i := range ts {
		pts = append(pts, &ts[i])
	}
	return pts
}

func CopyIf[T any](slice []T, condition func(o T) bool) []T {
	ds := make([]T, 0, len(slice))
	for _, v := range slice {
		if condition(v) {
			ds = append(ds, v)
		}
	}
	return ds
}

func Strings[T fmt.Stringer](fs []T) []string {
	strs := make([]string, 0, len(fs))
	for _, f := range fs {
		strs = append(strs, f.String())
	}

	return strs
}



func Append[T any](s []T, list ...T) []T {
	return append(s, list...)
}
