package sets

import "sort"

type Object struct {
	Name string
}

// sets.ObjectString is a set of strings, implemented via map[string]struct{} for minimal memory consumption.
type ObjectString map[string]Object

// NewObjectString creates a ObjectString from a list of values.
func NewObjectString(items ...Object) ObjectString {
	ss := ObjectString{}
	ss.Insert(items...)
	return ss
}

// Insert adds items to the set.
func (os ObjectString) Insert(items ...Object) ObjectString {
	for _, item := range items {
		os[item.Name] = item
	}
	return os
}

// Delete removes all items from the set.
func (os ObjectString) Delete(items ...Object) ObjectString {
	for _, item := range items {
		delete(os, item.Name)
	}
	return os
}

// Has returns true if and only if item is contained in the set.
func (os ObjectString) Has(item Object) bool {
	_, contained := os[item.Name]
	return contained
}

// HasAll returns true if and only if all items are contained in the set.
func (os ObjectString) HasAll(items ...Object) bool {
	for _, item := range items {
		if !os.Has(item) {
			return false
		}
	}
	return true
}

// HasAny returns true if any items are contained in the set.
func (os ObjectString) HasAny(items ...Object) bool {
	for _, item := range items {
		if os.Has(item) {
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
func (os ObjectString) Difference(s2 ObjectString) ObjectString {
	result := NewObjectString()
	for _, value := range os {
		if !s2.Has(value) {
			result.Insert(value)
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
func (s1 ObjectString) Union(s2 ObjectString) ObjectString {
	result := NewObjectString()
	for _, value := range s1 {
		result.Insert(value)
	}
	for _, value := range s2 {
		result.Insert(value)
	}
	return result
}

// Intersection returns a new set which includes the item in BOTH s1 and s2
// For example:
// s1 = {a1, a2}
// s2 = {a2, a3}
// s1.Intersection(s2) = {a2}.
func (s1 ObjectString) Intersection(s2 ObjectString) ObjectString {
	var walk, other ObjectString
	result := NewObjectString()
	if s1.Len() < s2.Len() {
		walk = s1
		other = s2
	} else {
		walk = s2
		other = s1
	}
	for _, value := range walk {
		if other.Has(value) {
			result.Insert(value)
		}
	}
	return result
}

// IsSuperset returns true if and only if s1 is a superset of s2.
func (os ObjectString) IsSuperset(s2 ObjectString) bool {
	for _, item := range s2 {
		if !os.Has(item) {
			return false
		}
	}
	return true
}

// Equal returns true if and only if s1 is equal (as a set) to s2.
// Two sets are equal if their membership is identical.
// (In practice, this means same elements, order doesn't matter).
func (os ObjectString) Equal(s2 ObjectString) bool {
	return len(os) == len(s2) && os.IsSuperset(s2)
}

type sortableSliceOfObject []Object

func (os sortableSliceOfObject) Len() int           { return len(os) }
func (os sortableSliceOfObject) Less(i, j int) bool { return lessString(os[i].Name, os[j].Name) }
func (os sortableSliceOfObject) Swap(i, j int)      { os[i], os[j] = os[j], os[i] }

// List returns the contents as a sorted string slice.
func (os ObjectString) List() []Object {
	res := make(sortableSliceOfObject, 0, len(os))
	for _, value := range os {
		res = append(res, value)
	}
	sort.Sort(res)
	return []Object(res)
}

// Returns a single element from the set.
func (os ObjectString) PopAny() (Object, bool) {
	for _, key := range os {
		os.Delete(key)
		return key, true
	}
	var zeroValue Object
	return zeroValue, false
}

// Len returns the size of the set.
func (os ObjectString) Len() int {
	return len(os)
}
