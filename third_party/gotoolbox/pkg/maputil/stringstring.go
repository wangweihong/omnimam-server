package maputil

import (
	"maps"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

// Deprecated: use generic function instead
type StringString map[string]string

//TODO : lock

func NewStringString() StringString {
	nm := make(map[string]string)
	return nm
}

func (m StringString) DeepCopy() StringString {
	o := make(map[string]string, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

func (m StringString) Init() StringString {
	if m == nil {
		return make(map[string]string)
	}
	return m
}

func (m StringString) Delete(key string) {
	if m == nil {
		return
	}
	delete(m, key)
}

func (m StringString) DeleteIfKey(condition func(string) bool) {
	if m == nil {
		return
	}

	for k := range m {
		if condition(k) {
			delete(m, k)
		}
	}
}

func (m StringString) DeleteIfValue(condition func(string) bool) {
	if m == nil {
		return
	}

	for k, v := range m {
		if condition(v) {
			delete(m, k)
		}
	}
}

func (m StringString) Has(key string) bool {
	if m != nil {
		if _, exist := m[key]; exist {
			return true
		}
	}
	return false
}

func (m StringString) Set(key string, value string) StringString {
	if m == nil {
		o := make(map[string]string)
		o[key] = value
		return o
	}
	m[key] = value
	return m
}

func (m StringString) Get(key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key]
	return v
}

func (m StringString) Keys() []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m StringString) ToSetString() sets.String {
	ss := sets.NewString()

	if m == nil {
		return ss
	}
	for k := range m {
		ss.Insert(k)
	}
	return ss
}

func (m StringString) Equal(m2 map[string]string) bool {
	return maps.Equal(m, m2)
	// if len(m) != len(m2) {
	// 	return false
	// }

	// for k1, v1 := range m {
	// 	v2, ok := m2[k1]
	// 	if !ok {
	// 		return false
	// 	}

	// 	if v1 != v2 {
	// 		return false
	// 	}
	// }
	// return true
}
