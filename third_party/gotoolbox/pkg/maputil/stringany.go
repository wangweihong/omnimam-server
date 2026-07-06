package maputil

import (
	"maps"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/wangweihong/gotoolbox/pkg/json"
)

type StringAny map[string]any

func ToStringAny(d map[string]any) StringAny {
	return StringAny(d)
}

// TODO : lock
func NewStringAny() StringAny {
	nm := make(map[string]any)
	return nm
}

func (m StringAny) DeepCopy() StringAny {
	return maps.Clone(m)
}

func (m StringAny) Init() StringAny {
	if m == nil {
		return make(map[string]any)
	}
	return m
}

func (m StringAny) Delete(key string) {
	if m == nil {
		return
	}
	delete(m, key)
}

func (m StringAny) DeleteIfKey(condition func(string) bool) {
	if m == nil {
		return
	}

	for k := range m {
		if condition(k) {
			delete(m, k)
		}
	}
}

func (m StringAny) DeleteIfValue(condition func(any) bool) {
	if m == nil {
		return
	}

	for k, v := range m {
		if condition(v) {
			delete(m, k)
		}
	}
}

func (m StringAny) Has(key string) bool {
	if m != nil {
		if _, exist := m[key]; exist {
			return true
		}
	}
	return false
}

func (m StringAny) HasKeyAndValue(key string, value any) bool {
	if m != nil {
		v, exist := m[key]
		if !exist {
			return false
		}

		if reflect.DeepEqual(v, value) {
			return true
		}
	}
	return false
}

func (m StringAny) IsSuperSet(m2 map[string]any) bool {
	if m2 == nil || m == nil {
		return false
	}

	for k, v := range m2 {
		if !m.HasKeyAndValue(k, v) {
			return false
		}
	}
	return true
}

func (m StringAny) Set(key string, value any) StringAny {
	if m == nil {
		o := make(map[string]any)
		o[key] = value
		return o
	}
	m[key] = value
	return m
}

func (m StringAny) Get(key string) any {
	if m == nil {
		return nil
	}
	v, _ := m[key]
	return v
}

func (m StringAny) Keys() []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m StringAny) Equal(m2 map[string]any) bool {
	return maps.Equal(m, m2)
	// if len(m) != len(m2) {
	// 	return false
	// }

	// for k1, v1 := range m {
	// 	v2, ok := m2[k1]
	// 	if !ok {
	// 		return false
	// 	}

	// 	if !reflect.DeepEqual(v1, v2) {
	// 		return false
	// 	}
	// }
	// return true
}

func (m StringAny) GetInt(key string) int {
	return TypedGet[string, int](m, key)
	// if m != nil && key != "" {
	// 	d := m.Get(key)
	// 	if d != nil {
	// 		dv, _ := d.(int)
	// 		return dv
	// 	}
	// }
	// return 0
}

func (m StringAny) GetString(key string) string {
	return TypedGet[string, string](m, key)
	// if m != nil && key != "" {
	// 	d := m.Get(key)
	// 	if d != nil {
	// 		dv, _ := d.(string)
	// 		return dv
	// 	}
	// }
	// return ""
}

func (m StringAny) GetMap(key string) map[string]any {
	if m != nil && key != "" {
		d := m.Get(key)
		if d != nil {
			dv, _ := d.(map[string]any)
			return dv
		}
	}
	return nil
}

func (m StringAny) GetMapSlice(key string) []map[string]any {
	if m != nil && key != "" {
		d := m.Get(key)
		if d != nil {
			dv, _ := d.([]any)
			if dv != nil {
				m := make([]map[string]any, 0, len(dv))
				for _, v := range dv {
					vd, _ := v.(map[string]any)
					if vd != nil {
						m = append(m, vd)
					}
				}
				return m
			}
		}
	}
	return nil
}

func (m StringAny) String() string {
	return json.ToString(m)
}

func (m StringAny) Decode(d any) error {
	if m != nil {
		return mapstructure.Decode(m, d)
	}
	return nil
}
