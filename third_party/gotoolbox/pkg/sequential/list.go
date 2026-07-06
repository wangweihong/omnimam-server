package sequential

import (
	"sort"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/sets"
)

// 记录对象出现的索引.
type List struct {
	lock    sync.RWMutex
	data    []interface{}
	indices map[ /*value*/ interface{}][]int
	len     int
}

func NewSequentialList(datas ...interface{}) *List {
	l := &List{
		data:    make([]interface{}, 0),
		indices: make(map[interface{}][]int),
	}
	for _, d := range datas {
		l.Inject(d)
	}
	return l
}

func NewLimitSequentialList(len int, datas ...interface{}) *List {
	if len < 0 {
		len = 0
	}

	l := &List{
		data:    make([]interface{}, 0),
		indices: make(map[interface{}][]int),
		len:     len,
	}
	for _, d := range datas {
		l.Inject(d)
	}

	return l
}

func (m *List) DeepCopy() *List {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m == nil {
		return nil
	}

	nm := &List{
		data:    make([]interface{}, 0, m.len),
		indices: make(map[interface{}][]int, m.len),
		len:     m.len,
	}

	for k, v := range m.indices {
		indices := make([]int, 0, len(v))
		for _, j := range v {
			indices = append(indices, j)
		}
		nm.indices[k] = indices
	}

	for _, v := range m.data {
		nm.data = append(nm.data, v)
	}
	return nm
}

func (m *List) Get(index int) interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if index < 0 || index > len(m.data)-1 {
		return nil
	}

	return m.data[index]
}

func (m *List) has(value interface{}) bool {
	_, exist := m.indices[value]
	return exist
}

func (m *List) Has(value interface{}) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.has(value)
}

func (m *List) Indices(value interface{}) []int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	indices, _ := m.indices[value]
	return indices
}

func (m *List) ForEach(f func(value interface{}) error) error {
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

func (m *List) InjectList(values ...interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, value := range values {
		m.inject(value)
	}
}

func (m *List) Inject(value interface{}) int {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.inject(value)
}

func (m *List) inject(value interface{}) int {
	m.data = append(m.data, value)
	index := len(m.data) - 1

	indices, exist := m.indices[value]
	if !exist {
		indices = make([]int, 0)
	}
	indices = append(indices, len(m.data)-1)
	m.indices[value] = indices

	if m.len != 0 && len(m.data) > m.len {
		m.deleteAtIndex(0)
		index = index - 1
	}
	return index
}

func (m *List) Update(index int, value interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if index < 0 || index > len(m.data)-1 {
		return
	}

	oldValue := m.data[index]
	if oldValue == value {
		return
	}

	oldIndices := m.indices[oldValue]
	oldIndicesClean := sets.NewInt(oldIndices...).Delete(index).List()
	m.indices[oldValue] = oldIndicesClean
	if len(oldIndicesClean) == 0 {
		delete(m.indices, oldValue)
	}

	indices, exist := m.indices[value]
	if !exist {
		indices = make([]int, 0)
	}
	indices = append(indices, index)
	// 索引排序
	sort.SliceStable(indices, func(i, j int) bool {
		return indices[i] < indices[j]
	})

	m.data[index] = value
	m.indices[value] = indices
}

func (m *List) List() []interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()

	nl := make([]interface{}, 0, len(m.data))
	for _, v := range m.data {
		nl = append(nl, v)
	}
	return nl
}

func (m *List) Len() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.data)
}

func (m *List) DeleteAtIndex(i int) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.deleteAtIndex(i)
}

func (m *List) deleteAtIndex(i int) {
	if i < -1 || i > len(m.data)-1 {
		return
	}
	nm := NewSequentialList()
	for k := range m.data {
		if k == i {
			continue
		}
		nm.Inject(m.data[k])
	}
	m.data = nm.data
	m.indices = nm.indices
}

func (m *List) DeleteIf(condition func(value interface{}) bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, v := range m.data {
		if condition(v) {
			m.delete(v)
		}
	}
}

func (m *List) Delete(value interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.delete(value)
}

func (m *List) delete(value interface{}) {
	nm := NewSequentialList()
	for _, v := range m.data {
		if v == value {
			continue
		}
		nm.Inject(v)
	}
	m.data = nm.data
	m.indices = nm.indices
	return
}

func (m *List) MoveFrontNumIf(condition func(v interface{}) bool, num int) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.moveFrontIf(condition, num)
}

func (m *List) MoveFrontIf(condition func(v interface{}) bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.moveFrontIf(condition, 0)
	//nm := NewSequentialList()
	//
	//indices := make([]int, 0)
	//for k, v := range m.data {
	//	if condition(v) {
	//		nm.inject(v)
	//		indices = append(indices, k)
	//	}
	//}
	//
	//if len(indices) == 0 {
	//	return
	//}
	//
	//for k := range m.data {
	//	if sets.NewInt(indices...).Has(k) {
	//		continue
	//	}
	//	nm.inject(m.data[k])
	//}
	//m.data = nm.data
	//m.indices = nm.indices
}

func (m *List) moveFrontIf(condition func(v interface{}) bool, num int) {
	if num < 0 {
		num = 0
	}

	nm := NewSequentialList()
	indices := make([]int, 0)
	i := 0
	for k, v := range m.data {
		if num == 0 || i < num {
			if condition(v) {
				nm.inject(v)
				indices = append(indices, k)
			}
		}
		i++
	}

	if len(indices) == 0 {
		return
	}

	for k := range m.data {
		if sets.NewInt(indices...).Has(k) {
			continue
		}
		nm.inject(m.data[k])
	}
	m.data = nm.data
	m.indices = nm.indices
}

func (m *List) MoveFront(value interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	indices, _ := m.indices[value]
	if len(indices) == 0 {
		return
	}

	nm := NewSequentialList()
	for i := 0; i < len(indices); i++ {
		nm.inject(value)
	}
	for k := range m.data {
		if sets.NewInt(indices...).Has(k) {
			continue
		}
		nm.inject(m.data[k])
	}
	m.data = nm.data
	m.indices = nm.indices
}

func (m *List) MoveAfterIf(condition func(v interface{}) bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	nm := NewSequentialList()
	indices := make([]int, 0)

	for k, v := range m.data {
		if !condition(v) {
			nm.inject(v)
		} else {
			indices = append(indices, k)
		}
	}

	if len(indices) == 0 {
		return
	}

	for _, index := range indices {
		nm.inject(m.data[index])
	}

	m.data = nm.data
	m.indices = nm.indices
}

func (m *List) moveAfterIf(condition func(v interface{}) bool, num int) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if num < 0 {
		num = 0
	}
	nm := NewSequentialList()
	indices := make([]int, 0)

	for k, v := range m.data {
		if !condition(v) {
			nm.inject(v)
		} else {
			indices = append(indices, k)
		}
	}

	if len(indices) == 0 {
		return
	}

	for _, index := range indices {
		nm.inject(m.data[index])
	}

	m.data = nm.data
	m.indices = nm.indices
}

func (m *List) MoveAfter(value interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()

	indices, _ := m.indices[value]
	if len(indices) == 0 {
		return
	}

	nm := NewSequentialList()

	for k := range m.data {
		if sets.NewInt(indices...).Has(k) {
			continue
		}
		nm.Inject(m.data[k])
	}

	for i := 0; i < len(indices); i++ {
		nm.inject(value)
	}
	m.data = nm.data
	m.indices = nm.indices
}

// 存在性能问题.
func (m *List) removeIndex(index int) {
	data := m.data[index]
	m.data = append(m.data[:index], m.data[index+1:]...)

	dataIndices := m.indices[data]
	newDataIndices := sets.NewInt(dataIndices...).Delete(index).List()
	m.indices[data] = newDataIndices

	for key, indices := range m.indices {
		var newIndices []int
		for _, idx := range indices {
			if idx != index {
				if idx > index {
					newIndices = append(newIndices, idx-1)
				} else {
					newIndices = append(newIndices, idx)
				}
			}
		}
		m.indices[key] = newIndices
	}
}

func (m *List) Clear() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.data = make([]interface{}, 0)
	m.indices = make(map[interface{}][]int)
}
