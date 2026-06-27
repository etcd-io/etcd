package tw

import "slices"

// Slicer is a generic slice type that provides additional methods for slice manipulation.
type Slicer[T any] []T

// NewSlicer creates and returns a new initialized Slicer.
func NewSlicer[T any]() Slicer[T] {
	return make(Slicer[T], 0)
}

// Get returns the element at the specified index.
// Returns the zero value if the index is out of bounds or the slice is nil.
func (s Slicer[T]) Get(index int) T {
	if s == nil || index < 0 || index >= len(s) {
		var zero T
		return zero
	}
	return s[index]
}

// GetOK returns the element at the specified index and a boolean indicating whether the index was valid.
func (s Slicer[T]) GetOK(index int) (T, bool) {
	if s == nil || index < 0 || index >= len(s) {
		var zero T
		return zero, false
	}
	return s[index], true
}

// Append appends elements to the slice and returns the new slice.
func (s Slicer[T]) Append(elements ...T) Slicer[T] {
	return append(s, elements...)
}

// Prepend adds elements to the beginning of the slice and returns the new slice.
func (s Slicer[T]) Prepend(elements ...T) Slicer[T] {
	return append(elements, s...)
}

// Len returns the number of elements in the slice.
// Returns 0 if the slice is nil.
func (s Slicer[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s)
}

// IsEmpty returns true if the slice is nil or has zero elements.
func (s Slicer[T]) IsEmpty() bool {
	return s.Len() == 0
}

// Has returns true if the index exists in the slice.
func (s Slicer[T]) Has(index int) bool {
	return index >= 0 && index < s.Len()
}

// First returns the first element of the slice, or the zero value if empty.
func (s Slicer[T]) First() T {
	return s.Get(0)
}

// Last returns the last element of the slice, or the zero value if empty.
func (s Slicer[T]) Last() T {
	return s.Get(s.Len() - 1)
}

// Each iterates over each element in the slice and calls the provided function.
// Does nothing if the slice is nil.
func (s Slicer[T]) Each(fn func(T)) {
	for _, v := range s {
		fn(v)
	}
}

// Filter returns a new Slicer containing only elements that satisfy the predicate.
func (s Slicer[T]) Filter(fn func(T) bool) Slicer[T] {
	result := NewSlicer[T]()
	for _, v := range s {
		if fn(v) {
			result = result.Append(v)
		}
	}
	return result
}

// Map returns a new Slicer with each element transformed by the provided function.
func (s Slicer[T]) Map(fn func(T) T) Slicer[T] {
	result := NewSlicer[T]()
	for _, v := range s {
		result = result.Append(fn(v))
	}
	return result
}

// Contains returns true if the slice contains an element that satisfies the predicate.
func (s Slicer[T]) Contains(fn func(T) bool) bool {
	return slices.ContainsFunc(s, fn)
}

// Find returns the first element that satisfies the predicate, along with a boolean indicating if it was found.
func (s Slicer[T]) Find(fn func(T) bool) (T, bool) {
	for _, v := range s {
		if fn(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// Clone returns a shallow copy of the Slicer.
func (s Slicer[T]) Clone() Slicer[T] {
	result := NewSlicer[T]()
	if s != nil {
		result = append(result, s...)
	}
	return result
}

// SlicerToMapper converts a Slicer of KeyValuePair to a Mapper.
func SlicerToMapper[K comparable, V any](s Slicer[KeyValuePair[K, V]]) Mapper[K, V] {
	result := make(Mapper[K, V])
	if s == nil {
		return result
	}
	for _, pair := range s {
		result[pair.Key] = pair.Value
	}
	return result
}
