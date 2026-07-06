package mathutil

const (
	LeftMask4K = (1<<63 - 1) ^ 4095
)

func RoundUp(length int64, roundSize int64) int64 {
	return (length + (roundSize - 1)) / roundSize * roundSize
}

func RoundDown(length int64, roundSize int64) int64 {
	return length / roundSize * roundSize
}

func RoundUp4K(length int64) int64 {
	return (length + 4095) & LeftMask4K
}

func RoundDown4K(length int64) int64 {
	return length & LeftMask4K
}
