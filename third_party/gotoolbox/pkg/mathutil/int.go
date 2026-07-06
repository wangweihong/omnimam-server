package mathutil

import "github.com/wangweihong/gotoolbox/pkg/generic"

func IntCompare[T generic.Int](a, b T) int {
	if a == b {
		return 0
	}
	if a < b {
		return 1
	}
	return -1
}

func IntMax[T generic.Int](a, b T) T {
	if b > a {
		return b
	}
	return a
}

func IntMin[T generic.Int](a, b T) T {
	if b > a {
		return a
	}
	return b
}
