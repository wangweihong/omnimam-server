package mathutil

import "github.com/wangweihong/gotoolbox/pkg/generic"

func Divide[T generic.Number](a, b T) T {
	if b == 0 {
		return 0
	}
	return a / b
}

func Add[T generic.Number](a, b T) T {
	return a + b
}

func Min[T generic.Number](a, b T) T {
	switch any(a).(type) {
	case float32, float64:
		epsilon := 1e-9
		aFloat, bFloat := float64(a), float64(b)
		if aFloat > bFloat && !FloatEqual(aFloat, bFloat, epsilon) {
			return b
		}
		return a
	default:
		if a < b {
			return a
		}
		return b
	}
}

func Max[T generic.Number](a, b T) T {
	switch any(a).(type) {
	case float32, float64:
		epsilon := 1e-9
		aFloat, bFloat := float64(a), float64(b)
		if aFloat > bFloat && !FloatEqual(aFloat, bFloat, epsilon) {
			return b
		}
		return a
	default:
		if a > b {
			return a
		}
		return b
	}
}
