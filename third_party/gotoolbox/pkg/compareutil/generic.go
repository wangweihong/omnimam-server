package compareutil

import (
	"cmp"
	//"golang.org/x/exp/constraints"
)

// // 定义可比较类型约束
// type Comparable interface {
// 	constraints.Integer | constraints.Float | ~string
// }

func EqualCompare[T cmp.Ordered](a, b T) int {
	if a == b {
		return 0
	}
	if a > b {
		return 1
	}
	return -1
}

func Compare[T cmp.Ordered](a, b T, asc bool) bool {
	if asc {
		return a < b
	}
	return a > b
}
