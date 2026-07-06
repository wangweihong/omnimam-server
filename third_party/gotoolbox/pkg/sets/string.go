package sets

import (
	"reflect"
	"sort"
	"strings"
)

type Empty struct{}

// sets.String is a set of strings, implemented via map[string]struct{} for minimal memory consumption.
type String map[string]Empty

// NewString creates a String from a list of values.
func NewString(items ...string) String {
	ss := String{}
	ss.Insert(items...)
	return ss
}

// StringKeySet creates a String from a keys of a map[string](? extends any).
// If the value passed in is not actually a map, this will panic.
func StringKeySet(theMap any) String {
	v := reflect.ValueOf(theMap)
	ret := String{}

	for _, keyValue := range v.MapKeys() {
		if str, ok := keyValue.Interface().(string); ok {
			ret.Insert(str)
		}
	}
	return ret
}

// Insert adds items to the set.
func (s String) Insert(items ...string) String {
	for _, item := range items {
		s[item] = Empty{}
	}
	return s
}

// InsertIf adds items to the set if match condition.
func (s String) InsertIf(condition func(string) bool, items ...string) String {
	for _, item := range items {
		if condition(item) {
			s[item] = Empty{}
		}
	}
	return s
}

// Delete removes all items from the set.
func (s String) Delete(items ...string) String {
	for _, item := range items {
		delete(s, item)
	}
	return s
}

// DeleteIf removes  items from the set if match condition.
func (s String) DeleteIf(condition func(string) bool, items ...string) String {
	for _, item := range items {
		if condition(item) {
			delete(s, item)
		}
	}
	return s
}

// Match return true if key and item match condition.
func (s String) Match(condition func(string, string) bool, item string) bool {
	for k := range s {
		if condition(k, item) {
			return true
		}
	}
	return false
}

// FindMatch return newSet match condition
func (s String) FindMatch(condition func(string, string) bool, item string) String {
	ns := NewString()
	for k := range s {
		if condition(k, item) {
			ns.Insert(k)
		}
	}
	return ns
}

// Match return true if key  match items condition.
func (s String) MatchAny(condition func(string, string) bool, items ...string) bool {
	for _, item := range items {
		if s.Match(condition, item) {
			return true
		}
	}
	return false
}

// Has returns true if and only if item is contained in the set.
func (s String) Has(item string) bool {
	_, contained := s[item]
	return contained
}

// Contain returns true if and only if item is contained in the set.
func (s String) Contain(item string) bool {
	return s.Match(func(key string, item string) bool {
		if strings.Contains(key, item) {
			return true
		}
		return false
	}, item)
}

// BeContain returns true if key in sets is contain by str.
func (s String) BeContain(str string) bool {
	for key := range s {
		if strings.Contains(str, key) {
			return true
		}
	}
	return false
}

func (s String) HasPrefix(item string) bool {
	return s.Match(func(key string, item string) bool {
		if strings.HasPrefix(key, item) {
			return true
		}
		return false
	}, item)
}

func (s String) AllHasPrefix(item string) bool {
	for key := range s {
		if !strings.HasPrefix(key, item) {
			return false
		}
	}
	return true
}

func (s String) HasSuffix(item string) bool {
	return s.Match(func(key string, item string) bool {
		if strings.HasSuffix(key, item) {
			return true
		}
		return false
	}, item)
}

func (s String) AllHasSuffix(item string) bool {
	for key := range s {
		if !strings.HasSuffix(key, item) {
			return false
		}
	}
	return true
}

// sets存在值为item的前缀.
func (s String) IsPrefixOf(item string) bool {
	return s.Match(func(key string, item string) bool {
		if strings.HasPrefix(item, key) {
			return true
		}
		return false
	}, item)
}

func (s String) IsSuffixOf(item string) bool {
	return s.Match(func(key string, item string) bool {
		if strings.HasSuffix(item, key) {
			return true
		}
		return false
	}, item)
}

// HasAll returns true if and only if all items are contained in the set.
func (s String) HasAll(items ...string) bool {
	for _, item := range items {
		if !s.Has(item) {
			return false
		}
	}
	return true
}

// HasAny returns true if any items are contained in the set.
func (s String) HasAny(items ...string) bool {
	for _, item := range items {
		if s.Has(item) {
			return true
		}
	}
	return false
}

// HasAny returns true if any items are contained in the set.
func (s String) HasAnyPrefix(items ...string) bool {
	for _, item := range items {
		if s.HasPrefix(item) {
			return true
		}
	}
	return false
}

// ContainsAny returns true if any items are string-contained in the set.
func (s String) ContainAny(items ...string) bool {
	for _, item := range items {
		if s.Contain(item) {
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
func (s String) Difference(s2 String) String {
	result := NewString()
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
func (s String) Union(s2 String) String {
	result := NewString()
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
func (s String) Intersection(s2 String) String {
	var walk, other String
	result := NewString()
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
func (s String) IsSuperset(s2 String) bool {
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
func (s String) Equal(s2 String) bool {
	return len(s) == len(s2) && s.IsSuperset(s2)
}

type sortableSliceOfString []string

func (s sortableSliceOfString) Len() int           { return len(s) }
func (s sortableSliceOfString) Less(i, j int) bool { return lessString(s[i], s[j]) }
func (s sortableSliceOfString) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// List returns the contents as a sorted string slice.
func (s String) List() []string {
	res := make(sortableSliceOfString, 0, len(s))
	for key := range s {
		res = append(res, key)
	}
	sort.Sort(res)
	return []string(res)
}

// UnsortedList returns the slice with contents in random order.
func (s String) UnsortedList() []string {
	res := make([]string, 0, len(s))
	for key := range s {
		res = append(res, key)
	}
	return res
}

// Returns a single element from the set.
func (s String) PopAny() (string, bool) {
	for key := range s {
		s.Delete(key)
		return key, true
	}
	var zeroValue string
	return zeroValue, false
}

// Len returns the size of the set.
func (s String) Len() int {
	return len(s)
}

func lessString(lhs, rhs string) bool {
	return lhs < rhs
}
