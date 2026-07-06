package paging

import (
	"math"

	"github.com/wangweihong/gotoolbox/pkg/generic"
)

func Index[T generic.Int](length, page, size T) (sIndex, eIndex T) {
	if length < 0 {
		length = 0
	}

	if page < 0 {
		page = 0
	}

	if size < 0 {
		size = 0
	}

	if page == 0 && size == 0 {
		sIndex = 0
		eIndex = length
		return
	}

	// page从0开始
	sIndex = page * size
	eIndex = T(math.Min(float64(sIndex+size), float64(length)))
	if sIndex > eIndex {
		sIndex = 0
		eIndex = 0
		return
	}

	return
}

func Cut[T generic.Int, R any](length, page, size T, list []R) []R {
	if len(list) == 0 {
		return list
	}
	s, e := Index(length, page, size)
	return list[s:e]
}
