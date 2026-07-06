package maputil

import (
	"maps"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

// Deprecated: use generic function instead
type StringInt map[string]int

//TODO : lock

func (m StringInt) DeepCopy() StringInt {
	o := make(map[string]int, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

func (m StringInt) Init() StringInt {
	if m == nil {
		return make(map[string]int)
	}
	return m
}

func (m StringInt) AddKeys(keys ...string) StringInt {
	if m == nil {
		m = make(map[string]int)
	}
	for _, key := range keys {
		if _, exist := m[key]; !exist {
			m[key] = 0
		}
	}
	return m
}

func (m StringInt) Delete(key string) {
	if m == nil {
		return
	}
	delete(m, key)
}

func (m StringInt) DeleteIfKey(condition func(string) bool) {
	if m == nil {
		return
	}

	for k := range m {
		if condition(k) {
			delete(m, k)
		}
	}
}

func (m StringInt) DeleteIfValue(condition func(int) bool) {
	if m == nil {
		return
	}

	for k, v := range m {
		if condition(v) {
			delete(m, k)
		}
	}
}

func (m StringInt) Has(key string) bool {
	if m != nil {
		if _, exist := m[key]; exist {
			return true
		}
	}
	return false
}

func (m StringInt) Set(key string, value int) StringInt {
	if m == nil {
		o := make(map[string]int)
		o[key] = value
		return o
	}
	m[key] = value
	return m
}

func (m StringInt) Get(key string) int {
	if m == nil {
		return 0
	}
	v, _ := m[key]
	return v
}

func (m StringInt) Keys() []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m StringInt) ToSetString() sets.String {
	ss := sets.NewString()
	if m == nil {
		return ss
	}
	for k := range m {
		ss.Insert(k)
	}
	return ss
}

func (m StringInt) Equal(m2 map[string]int) bool {
	return maps.Equal(m,m2)
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

// SetAndIncrementKey 如果键存在则增1，否则设置为1
func (m StringInt) SetAndIncrementKey(key string) StringInt {
	if m == nil {
		m = make(map[string]int)
	}
	d, exist := m[key]
	if !exist {
		m[key] = 1
		return m
	}
	m[key] = d + 1
	return m
}

// SetAndIncrementKey 合并两个map的键.
func (m StringInt) MergeMaps(a map[string]int) StringInt {
	if m == nil {
		m = make(map[string]int)
	}
	if a == nil {
		return m
	}

	for key, _ := range a {
		if _, exists := m[key]; !exists {
			m[key] = 0
		}
	}

	for key, _ := range m {
		if _, exists := a[key]; !exists {
			a[key] = 0
		}
	}
	return m
}
