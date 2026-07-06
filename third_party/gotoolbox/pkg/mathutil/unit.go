package mathutil

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

func ConvertMBToByte(sizeInMB int64) int64 {
	return sizeInMB * 1024 * 1024
}

func ConvertByteToMB(sizeInByte int64) int64 {
	return sizeInByte / 1024 / 1024
}

func ConvertGBToByte(sizeInGB int64) int64 {
	return sizeInGB * 1024 * 1024 * 1024
}

// 最小是B，最大是TB
func ParseSizeByteToStr(size uint64) string {
	kb := uint64(1024)
	mb := 1024 * kb
	gb := 1024 * mb
	tb := 1024 * gb

	unit := size / tb
	if unit > 0 {
		return fmt.Sprintf("%dT", unit)
	}

	unit = size / gb
	if unit > 0 {
		return fmt.Sprintf("%dG", unit)
	}

	unit = size / mb
	if unit > 0 {
		return fmt.Sprintf("%dM", unit)
	}

	unit = size / kb
	if unit > 0 {
		return fmt.Sprintf("%dK", unit)
	}

	return fmt.Sprintf("%dB", size)
}

func ParseSizeBitToStr(size uint64) (string, error) {
	kb := uint64(1024)
	mb := 1024 * kb
	gb := 1024 * mb
	tb := 1024 * gb

	if size%kb != 0 {
		return "", errors.New("size is not multiple of KB")
	}

	if size%tb == 0 {
		return fmt.Sprintf("%dT", size/tb), nil
	}

	if size%mb == 0 {
		return fmt.Sprintf("%dM", size/mb), nil
	}

	return fmt.Sprintf("%dK", size/kb), nil
}

func ParseSizeInMb(size string) (int64, error) {
	if size == "" {
		return 0, errors.New("size is empty")
	}

	size = strings.ToLower(size)
	readableSize := regexp.MustCompile(`^[0-9.]+[kmgt]$`)
	if !readableSize.MatchString(size) {
		value, err := strconv.ParseInt(size, 10, 64)
		return value, err
	}

	last := len(size) - 1
	unit := string(size[last])
	value, err := strconv.ParseInt(size[:last], 10, 64)
	if err != nil {
		return 0, err
	}

	kb := int64(1024)
	mb := 1024 * kb
	gb := 1024 * mb
	tb := 1024 * gb
	switch unit {
	case "k":
		value *= kb
	case "m":
		value *= mb
	case "g":
		value *= gb
	case "t":
		value *= tb
	default:
		return 0, fmt.Errorf("Unrecongized size value %v", size)
	}

	valueMb := (value / mb)
	return valueMb, err
}

func ParseSizeInByte(size string) (uint64, error) {
	if size == "" {
		return 0, errors.New("size is empty")
	}

	size = strings.ToLower(size)
	readableSize := regexp.MustCompile(`^[0-9.]+[kmgtKMGT]$`)
	if !readableSize.MatchString(size) {
		value, err := strconv.ParseUint(size, 10, 64)
		return value, err
	}

	last := len(size) - 1
	unit := string(size[last])
	value, err := strconv.ParseFloat(size[:last], 64)
	if err != nil {
		return 0, err
	}

	kb := float64(1024)
	mb := 1024 * kb
	gb := 1024 * mb
	tb := 1024 * gb
	switch unit {
	case "k", "K":
		value *= kb
	case "m", "M":
		value *= mb
	case "g", "G":
		value *= gb
	case "t", "T":
		value *= tb
	default:
		return 0, fmt.Errorf("Unrecongized size value %v", size)
	}
	return uint64(value), err
}

func ParseSizeRoundUpInByte(size string) (uint64, error) {
	if size == "" {
		return 0, errors.New("size is empty")
	}

	size = strings.ToLower(size)
	readableSize := regexp.MustCompile(`^[0-9.]+[kmgtKMGT]$`)
	if !readableSize.MatchString(size) {
		value, err := strconv.ParseUint(size, 10, 64)
		return value, err
	}

	last := len(size) - 1
	unit := string(size[last])
	value, err := strconv.ParseFloat(size[:last], 64)
	if err != nil {
		return 0, err
	}

	retValue := uint64(0)
	switch unit {
	case "k", "K":
		retValue = uint64(math.Ceil(value * 1024)) // 字节向上取整
	case "m", "M":
		retValue = uint64(math.Ceil(value * 1024)) // kb向上取整
		retValue <<= 10                            // kb换算为字节
	case "g", "G":
		retValue = uint64(math.Ceil(value * 1024)) // mb向上取整
		retValue <<= 20                            // mb换算为字节
	case "t", "T":
		retValue = uint64(math.Ceil(value * 1024)) // gb向上取整
		retValue <<= 30                            // gb换算为字节
	default:
		return 0, fmt.Errorf("Unrecongized size value %s ", size)
	}

	return retValue, err
}

func ParseSizeInBit(size string) (uint64, error) {
	if size == "" {
		return 0, errors.New("size is empty")
	}

	size = strings.ToLower(size)
	readableSize := regexp.MustCompile(`^[0-9.]+[kmgt]$`)
	if !readableSize.MatchString(size) {
		value, err := strconv.ParseUint(size, 10, 64)
		return value, err
	}

	last := len(size) - 1
	unit := string(size[last])
	value, err := strconv.ParseUint(size[:last], 10, 64)
	if err != nil {
		return 0, err
	}

	kb := uint64(1024)
	mb := 1024 * kb
	gb := 1024 * mb
	tb := 1024 * gb
	switch unit {
	case "k":
		value *= kb
	case "m":
		value *= mb
	case "g":
		value *= gb
	case "t":
		value *= tb
	default:
		return 0, fmt.Errorf("Unrecongized size value %v", size)
	}
	return uint64(value), err
}

func ParseSizeByteToStrExactly(size int64, scale int) string {
	kb := float64(1024)
	mb := 1024 * kb
	gb := 1024 * mb
	tb := 1024 * gb

	sizef := float64(size)

	desc := fmt.Sprintf("%%.%df%%s", scale)
	unit := sizef / tb
	if unit >= 1 {
		return fmt.Sprintf(desc, unit, "T")
	}

	unit = sizef / gb
	if unit >= 1 {
		return fmt.Sprintf(desc, unit, "G")
	}

	unit = sizef / mb
	if unit >= 1 {
		return fmt.Sprintf(desc, unit, "M")
	}

	unit = sizef / kb
	if unit >= 1 {
		return fmt.Sprintf(desc, unit, "K")
	}

	return fmt.Sprintf("%d", size)
}

func ConvertSizeStructToBytes(value uint64, unit string) (uint64, error) {
	capacity := value
	switch unit {
	case "", "B", "bytes":
	case "KB":
		capacity *= 1000
	case "K", "KiB":
		capacity *= 1024
	case "MB":
		capacity *= 1000 * 1000
	case "M", "MiB":
		capacity *= 1024 * 1024
	case "GB":
		capacity *= 1000 * 1000 * 1000
	case "G", "GiB":
		capacity *= 1024 * 1024 * 1024
	case "TB":
		capacity *= 1000 * 1000 * 1000 * 1000
	case "T", "TiB":
		capacity *= 1024 * 1024 * 1024 * 1024
	case "PB":
		capacity *= 1000 * 1000 * 1000 * 1000 * 1000
	case "P", "PiB":
		capacity *= 1024 * 1024 * 1024 * 1024 * 1024
	case "EB":
		capacity *= 1000 * 1000 * 1000 * 1000 * 1000 * 1000
	case "E", "EiB":
		capacity *= 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unit")
	}
	return capacity, nil
}
