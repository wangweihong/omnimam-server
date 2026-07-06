package typeutil

import "strconv"

func GenericIndirectValue[T any](p *T) T {
	var zero T
	if p != nil {
		return *p
	}
	return zero
}

// 不为空则使用指定的值，否则使用预定值
func GenericIndirectPrefined[T any](p *T, prefine T) T {
	if p != nil {
		return *p
	}
	return prefine
}

func MustAtoi(d string) int {
	r, _ := strconv.Atoi(d)
	return r
}

func MustAtoi64(d string) int64 {
	r, _ := strconv.Atoi(d)
	return int64(r)
}
