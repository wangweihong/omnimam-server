package sortutil

import (
	"sort"
)

type SortFunc func(v1, v2 any, Asc bool) bool

type Item struct {
	Value any
	Index int
}

type ByValue struct {
	Items    []Item
	Comparer func(a, b any) bool
}

func NewByValue(items []Item, comparer func(a, b any) bool) ByValue {
	return ByValue{
		Items:    items,
		Comparer: comparer,
	}
}

func (a ByValue) Len() int      { return len(a.Items) }
func (a ByValue) Swap(i, j int) { a.Items[i], a.Items[j] = a.Items[j], a.Items[i] }
func (a ByValue) Less(i, j int) bool {
	return a.Comparer(a.Items[i].Value, a.Items[j].Value)
}

// 根据master的排序顺序更改slaves的顺序
type MasterBasedSorter struct {
	master   []any
	comparer func(a, b any) bool
	indexes  []Item
}

func NewMasterBasedSorter(master []any, Comparer func(a, b any) bool) MasterBasedSorter {
	return MasterBasedSorter{
		master:   master,
		comparer: Comparer,
		indexes:  nil,
	}
}

func (ms MasterBasedSorter) Sort(dps ...*[]any) {
	if dps == nil {
		return
	}

	for _, dp := range dps {
		if dp == nil {
			continue
		}
		data := *dp
		indexes := ms.GetSortIndexes()
		//  长度不一致则忽略
		if len(ms.master) != len(data) {
			return
		}

		sortedC := make([]any, len(ms.master))
		for i, item := range indexes {
			sortedC[i] = data[item.Index]
		}
		*dp = sortedC
	}
}

func (ms MasterBasedSorter) GetSortIndexes() []Item {
	if ms.indexes != nil {
		return ms.indexes
	}
	ms.indexes = make([]Item, len(ms.master))
	for i, value := range ms.master {
		ms.indexes[i] = Item{Value: value, Index: i}
	}

	sort.Sort(NewByValue(ms.indexes, ms.comparer))
	return ms.indexes
}
