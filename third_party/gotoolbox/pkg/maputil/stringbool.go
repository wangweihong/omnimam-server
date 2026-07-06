package maputil

import "github.com/wangweihong/gotoolbox/pkg/sets"

// Deprecated: use generic function instead
type StringBool map[string]bool

func (m StringBool) DeepCopy() StringBool {
	o := make(map[string]bool, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

func (m StringBool) Init() StringBool {
	if m == nil {
		return make(map[string]bool)
	}
	return m
}

func (m StringBool) Delete(key string) {
	if m == nil {
		return
	}
	delete(m, key)
}

func (m StringBool) DeleteIfKey(condition func(string) bool) {
	if m == nil {
		return
	}

	for k := range m {
		if condition(k) {
			delete(m, k)
		}
	}
}

func (m StringBool) DeleteIfValue(condition func(bool) bool) {
	if m == nil {
		return
	}

	for k, v := range m {
		if condition(v) {
			delete(m, k)
		}
	}
}

func (m StringBool) Has(key string) bool {
	if m != nil {
		if _, exist := m[key]; exist {
			return true
		}
	}
	return false
}

func (m StringBool) HasAny(key string) bool {
	if m != nil {
		if _, exist := m[key]; exist {
			return true
		}
	}
	return false
}

func (m StringBool) Set(key string, value bool) StringBool {
	if m == nil {
		o := make(map[string]bool)
		o[key] = value
		return o
	}
	m[key] = value
	return m
}

func (m StringBool) Get(key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key]
	return v
}

func (m StringBool) Keys() []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m StringBool) ToSetString() sets.String {
	ss := sets.NewString()

	if m == nil {
		return ss
	}
	for k := range m {
		ss.Insert(k)
	}
	return ss
}
