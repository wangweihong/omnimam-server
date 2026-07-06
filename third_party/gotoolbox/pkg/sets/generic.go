package sets

import (
	"sort"
	"strings"
)

type GenericSet[T comparable] map[T]struct{}

// NewGenericSet creates a GenericSet from a list of values.
func NewGenericSet[T comparable](items ...T) GenericSet[T] {
	ss := GenericSet[T]{}
	ss.Insert(items...)
	return ss
}

// Insert adds items to the set.
func (s GenericSet[T]) Insert(items ...T) GenericSet[T] {
	for _, item := range items {
		s[item] = Empty{}
	}
	return s
}

// InsertIf adds items to the set if match condition.
func (s GenericSet[T]) InsertIf(condition func(T) bool, items ...T) GenericSet[T] {
	for _, item := range items {
		if condition(item) {
			s[item] = Empty{}
		}
	}
	return s
}

// Delete removes all items from the set.

func (s GenericSet[T]) Delete(items ...T) GenericSet[T] {
	for _, item := range items {
		delete(s, item)
	}
	return s
}

// DeleteIf removes  items from the set if match condition.
func (s GenericSet[T]) DeleteIf(condition func(T) bool) GenericSet[T] {
	for item := range s {
		if condition(item) {
			delete(s, item)
		}
	}
	return s
}

// Match return true if key and item match condition.
func (s GenericSet[T]) Match(condition func(setData, outside T) bool, item T) bool {
	for k := range s {
		if condition(k, item) {
			return true
		}
	}
	return false
}

// FindMatch return newSet match condition
func (s GenericSet[T]) FindMatch(condition func(T) bool) GenericSet[T] {
	ns := NewGenericSet[T]()
	for k := range s {
		if condition(k) {
			ns.Insert(k)
		}
	}
	return ns
}

// MatchAny return true if key  match any items condition.
func (s GenericSet[T]) MatchAny(condition func(T, T) bool, items ...T) bool {
	for _, item := range items {
		if s.Match(condition, item) {
			return true
		}
	}
	return false
}

// Has returns true if and only if item is contained in the set.
func (s GenericSet[T]) Has(item T) bool {
	_, contained := s[item]
	return contained
}

// HasAll returns true if and only if all items are contained in the set.
func (s GenericSet[T]) HasAll(items ...T) bool {
	for _, item := range items {
		if !s.Has(item) {
			return false
		}
	}
	return true
}

// HasAny returns true if any items are contained in the set.
func (s GenericSet[T]) HasAny(items ...T) bool {
	for _, item := range items {
		if s.Has(item) {
			return true
		}
	}
	return false
}

// Difference returns a set of objects that are not in s2
// For example:
// s1 = {a1, a2, a3}
// s2 = {a1, a2, a4, a5}
// s1.Difference(s2) = {a3}
// s2.Difference(s1) = {a4, a5}.
func (s GenericSet[T]) Difference(s2 GenericSet[T]) GenericSet[T] {
	result := NewGenericSet[T]()
	for key := range s {
		if !s2.Has(key) {
			result.Insert(key)
		}
	}
	return result
}

// Union returns a new set which includes items in either s1 or s2.
// For example:
// s1 = {a1, a2}
// s2 = {a3, a4}
// s1.Union(s2) = {a1, a2, a3, a4}
// s2.Union(s1) = {a1, a2, a3, a4}.
func (s GenericSet[T]) Union(s2 GenericSet[T]) GenericSet[T] {
	result := NewGenericSet[T]()
	for key := range s {
		result.Insert(key)
	}
	for key := range s2 {
		result.Insert(key)
	}
	return result
}

// Intersection returns a new set which includes the item in BOTH s1 and s2
// For example:
// s1 = {a1, a2}
// s2 = {a2, a3}
// s1.Intersection(s2) = {a2}.
func (s GenericSet[T]) Intersection(s2 GenericSet[T]) GenericSet[T] {
	var walk, other GenericSet[T]
	result := NewGenericSet[T]()
	if s.Len() < s2.Len() {
		walk = s
		other = s2
	} else {
		walk = s2
		other = s
	}
	for key := range walk {
		if other.Has(key) {
			result.Insert(key)
		}
	}
	return result
}

// IsSuperset returns true if and only if s1 is a superset of s2.
func (s GenericSet[T]) IsSuperset(s2 GenericSet[T]) bool {
	for item := range s2 {
		if !s.Has(item) {
			return false
		}
	}
	return true
}

// Equal returns true if and only if s1 is equal (as a set) to s2.
// Two sets are equal if their membership is identical.
// (In practice, this means same elements, order doesn't matter).

func (s GenericSet[T]) Equal(s2 GenericSet[T]) bool {
	return len(s) == len(s2) && s.IsSuperset(s2)
}

// List returns the contents as a sorted GenericSet slice.
func (s GenericSet[T]) List(compare func(a, b T) bool) []T {
	res := make([]T, 0, len(s))
	for key := range s {
		res = append(res, key)
	}
	sort.Slice(res, func(i, j int) bool {
		return compare(res[i], res[j])
	})
	return res
}

// UnsortedList returns the slice with contents in random order.
func (s GenericSet[T]) UnsortedList() []T {
	res := make([]T, 0, len(s))
	for key := range s {
		res = append(res, key)
	}
	return res
}

// PopAny Returns a single element from the set.
func (s GenericSet[T]) PopAny() (T, bool) {
	for key := range s {
		s.Delete(key)
		return key, true
	}
	var zeroValue T
	return zeroValue, false
}

// Len returns the size of the set.
func (s GenericSet[T]) Len() int {
	return len(s)
}

// Contain returns true if and only if item is contained in the set.
func (s GenericSet[T]) Contain(item T) bool {
	var zero T
	if _, ok := any(zero).(string); !ok {
		return false
	}

	return s.Match(func(key T, item T) bool {
		k := any(key).(string)
		v := any(key).(string)
		return strings.Contains(k, v)
	}, item)
}

// ContainAny returns true if and only if item is contained in the set.
func (s GenericSet[T]) ContainAny(items ...T) bool {
	var zero T
	if _, ok := any(zero).(string); !ok {
		return false
	}

	for _, item := range items {
		if s.Contain(item) {
			return true
		}
	}

	return false
}

// HasPrefix 集合中的数据是否有指定数据作为前缀
func (s GenericSet[T]) HasPrefix(data T) bool {
	var zero T
	if _, ok := any(zero).(string); !ok {
		return false
	}
	return s.Match(func(key T, item T) bool {
		k := any(key).(string)
		v := any(key).(string)
		return strings.HasPrefix(k, v)
	}, data)
}

// HasAnyPrefix 集合中的数据是否有指定数据作为前缀
func (s GenericSet[T]) HasAnyPrefix(datas ...T) bool {
	var zero T
	if _, ok := any(zero).(string); !ok {
		return false
	}

	for _, data := range datas {
		if s.HasPrefix(data) {
			return true
		}
	}
	return false
}

// HasSuffix 集合中的数据是否有指定数据作为前缀
func (s GenericSet[T]) HasSuffix(data T) bool {
	var zero T
	if _, ok := any(zero).(string); !ok {
		return false
	}
	return s.Match(func(key T, item T) bool {
		k := any(key).(string)
		v := any(item).(string)
		return strings.HasSuffix(k, v)
	}, data)
}

// HasAnySuffix 集合中的数据是否有指定数据作为前缀
func (s GenericSet[T]) HasAnySuffix(datas ...T) bool {
	var zero T
	if _, ok := any(zero).(string); !ok {
		return false
	}
	for _, data := range datas {
		if s.HasSuffix(data) {
			return true
		}
	}
	return false
}

// BeSuffix 集合中的数据是否存在指定数据作为后缀
func (s GenericSet[T]) BeSuffix(data T) bool {
	var zero T
	if _, ok := any(zero).(string); !ok {
		return false
	}
	return s.Match(func(key T, item T) bool {
		k := any(key).(string)
		v := any(item).(string)
		return strings.HasSuffix(v, k)
	}, data)
}
