package mathutil

import (
	"fmt"
	"math"
	"strconv"
)

func FloatEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func FloatBiggerThan(a, b float64) bool {
	epsilon := 1e-9

	return a > b && !FloatEqual(a, b, epsilon)
}

func FloatDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func FloatToString(a float64, digitNum int) string {
	if digitNum <= 0 {
		digitNum = 1
	}
	f := "%." + strconv.Itoa(digitNum) + "f"
	return fmt.Sprintf(f, a)
}

// FloatRoundToInt rounds floats into integer numbers.
func FloatRoundToInt(a float64) int {
	if a < 0 {
		return int(a - 0.5)
	}
	return int(a + 0.5)
}
