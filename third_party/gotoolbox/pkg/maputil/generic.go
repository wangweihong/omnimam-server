package maputil

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/generic"
)

func Get[K comparable, V any](m map[K]V, key K) V {
	if m != nil {
		val, exists := m[key]
		if exists {
			return val
		}
	}

	// 如果类型不匹配或者空map，返回零值
	var zeroValue V
	return zeroValue
}

func Has[K comparable, V any](m map[K]V, key K) bool {
	if m != nil {
		_, exists := m[key]
		return exists
	}
	return false
}

func Delete[K comparable, V any](m map[K]V, keys ...K) {
	if m != nil {
		for _, key := range keys {
			delete(m, key)
		}
	}
}

func Insert[K comparable, V any](m map[K]V, key K, v V) map[K]V {
	if m == nil {
		m = make(map[K]V)
	}
	m[key] = v
	return m
}

func Equal[K comparable, V comparable](m map[K]V, n map[K]V) bool {
	return maps.Equal(m, n)
}

func Clone[K comparable, V any](m map[K]V) map[K]V {
	return maps.Clone(m)
	// if m == nil {
	// 	return nil
	// }
	// n := make(map[K]V, len(m))
	// for k, v := range m {
	// 	n[k] = v
	// }
	// return n
}

// Copy 将src的数据拷贝到dst
func Copy[K comparable, V any](src map[K]V, dst map[K]V) map[K]V {
	if src != nil && dst == nil {
		dst = make(map[K]V, len(src))
	}
	maps.Copy(dst, src)
	return dst
	// if m == nil {
	// 	return nil
	// }
	// n := make(map[K]V, len(m))
	// for k, v := range m {
	// 	n[k] = v
	// }
	// return n
}

func DeleteIfKey[K comparable, V any](m map[K]V, condition func(d K) bool) {
	if m == nil {
		return
	}

	for k := range m {
		if condition(k) {
			delete(m, k)
		}
	}
}

func DeleteIfValue[K comparable, V any](m map[K]V, condition func(d V) bool) {
	if m == nil {
		return
	}

	for k, v := range m {
		if condition(v) {
			delete(m, k)
		}
	}
}

func ToString[K comparable, V any](m map[K]V) string {
	if m == nil {
		return ""
	}
	strs := make([]string, 0)
	for k, v := range m {
		strs = append(strs, fmt.Sprintf("%v", k)+"="+fmt.Sprintf("%v", v))
	}

	sort.Slice(strs, func(i, j int) bool {
		return strs[i] < strs[j]
	})
	return strings.Join(strs, ",")
}

func Keys[K generic.Ordered, V any](m map[K]V) []K {
	if m == nil {
		return nil
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

func TypedGet[K comparable, T any](m map[K]any, key K) T {
	var zero T

	if m != nil {
		if v, exists := m[key]; exists && v != nil {
			if t, ok := v.(T); ok {
				return t
			}
		}
	}
	return zero
}
