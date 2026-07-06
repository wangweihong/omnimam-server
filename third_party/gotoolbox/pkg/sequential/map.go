package sequential

import (
	"sync"
)

// 顺序表, 结合表的功能，提供插入数据的顺序.
type Map struct {
	lock sync.RWMutex
	// 存放键
	data []interface{}
	// 用于记录插入的键的顺序
	key []interface{}
	// 指定某个键的索引
	indices map[ /*key*/ interface{}]int
	// 控制表的长度
	len int
}

func NewSequentialMap() *Map {
	return &Map{
		data:    make([]interface{}, 0),
		key:     make([]interface{}, 0),
		indices: make(map[interface{}]int),
	}
}

func NewLimitSequentialMap(len int) *Map {
	if len < 0 {
		len = 0
	}

	return &Map{
		data:    make([]interface{}, 0),
		key:     make([]interface{}, 0),
		indices: make(map[interface{}]int),
		len:     len,
	}
}

func (m *Map) Get(value interface{}) interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if value != nil {
		i, exist := m.indices[value]
		if exist {
			return m.data[i]
		}
	}
	return nil
}

func (m *Map) DeepCopy() *Map {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m == nil {
		return nil
	}

	nm := &Map{
		data:    make([]interface{}, 0, m.len),
		key:     make([]interface{}, 0, m.len),
		indices: make(map[interface{}]int, m.len),
		len:     m.len,
	}
	for k, v := range m.indices {
		nm.indices[k] = v
	}

	for _, v := range m.key {
		nm.key = append(nm.key, v)
	}

	for _, v := range m.data {
		nm.data = append(nm.data, v)
	}
	return nm
}

func (m *Map) Has(key interface{}) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if key == nil {
		return false
	}

	_, exist := m.indices[key]
	return exist
}

func (m *Map) HasValue(value interface{}) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	for _, v := range m.data {
		if v == value {
			return true
		}
	}

	return false
}

func (m *Map) ForEach(f func(value interface{}) error) error {
	m.lock.RLock()
	defer m.lock.RUnlock()

	for _, v := range m.data {
		err := f(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Map) Inject(key interface{}, value interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if key == nil {
		return
	}

	i, exist := m.indices[key]
	if exist {
		// update new value if exist
		m.data[i] = value

		return
	}

	m.data = append(m.data, value)
	m.key = append(m.key, key)
	m.indices[key] = len(m.data) - 1

	// 达到表的上限
	if m.len != 0 && len(m.data) > m.len {
		m.delete(m.key[0])
	}
}

func (m *Map) Map() map[interface{}]interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	nm := make(map[interface{}]interface{})
	for k, v := range m.indices {
		nm[k] = m.data[v]
	}
	return nm
}

// Last return last inject element.
func (m *Map) Last() interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if len(m.data) == 0 {
		return nil
	}
	return m.data[len(m.data)-1]
}

// Last return first inject element.
func (m *Map) First() interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if len(m.data) == 0 {
		return nil
	}
	return m.data[0]
}

func (m *Map) Keys() []interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	keys := make([]interface{}, 0, len(m.data))
	keys = append(keys, m.key...)
	return keys
}

func (m *Map) Values() []interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	vals := make([]interface{}, 0, len(m.data))
	vals = append(vals, m.data...)
	return vals
}

func (m *Map) Len() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.data)
}

func (m *Map) Delete(key interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.delete(key)
}

func (m *Map) DeleteIfKey(condition func(key interface{}) bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, v := range m.key {
		if condition(v) {
			m.delete(v)
		}
	}
}

func (m *Map) DeleteIfValue(condition func(value interface{}) bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i, v := range m.data {
		if condition(v) {
			key := m.key[i]
			m.delete(key)
		}
	}
}

func (m *Map) delete(key interface{}) {
	if key == nil {
		return
	}

	index, ok := m.indices[key]
	if !ok {
		return
	}

	delete(m.indices, key)

	m.data = append(m.data[:index], m.data[index+1:]...)
	m.key = append(m.key[:index], m.key[index+1:]...)
	// 更新被移除元素后面元素的索引
	for i := index; i < len(m.data); i++ {
		m.indices[m.data[i]] = i
	}
}

func (m *Map) Clear() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.data = make([]interface{}, 0)
	m.key = make([]interface{}, 0)
	m.indices = make(map[interface{}]int)
}
