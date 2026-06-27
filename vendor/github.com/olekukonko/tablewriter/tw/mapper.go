package tw

import (
	"fmt"
	"sort"
)

// KeyValuePair represents a single key-value pair from a Mapper.
type KeyValuePair[K comparable, V any] struct {
	Key   K
	Value V
}

// Mapper is a generic map type with comparable keys and any value type.
// It provides type-safe operations on maps with additional convenience methods.
type Mapper[K comparable, V any] map[K]V

// NewMapper creates and returns a new initialized Mapper.
func NewMapper[K comparable, V any]() Mapper[K, V] {
	return make(Mapper[K, V])
}

// Get returns the value associated with the key.
// If the key doesn't exist or the map is nil, it returns the zero value for the value type.
func (m Mapper[K, V]) Get(key K) V {
	if m == nil {
		var zero V
		return zero
	}
	return m[key]
}

// OK returns the value associated with the key and a boolean indicating whether the key exists.
func (m Mapper[K, V]) OK(key K) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	val, ok := m[key]
	return val, ok
}

// Set sets the value for the specified key.
// Does nothing if the map is nil.
func (m Mapper[K, V]) Set(key K, value V) Mapper[K, V] {
	if m != nil {
		m[key] = value
	}
	return m
}

// Delete removes the specified key from the map.
// Does nothing if the key doesn't exist or the map is nil.
func (m Mapper[K, V]) Delete(key K) Mapper[K, V] {
	if m != nil {
		delete(m, key)
	}
	return m
}

// Has returns true if the key exists in the map, false otherwise.
func (m Mapper[K, V]) Has(key K) bool {
	if m == nil {
		return false
	}
	_, exists := m[key]
	return exists
}

// Len returns the number of elements in the map.
// Returns 0 if the map is nil.
func (m Mapper[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return len(m)
}

// Keys returns a slice containing all keys in the map.
// Returns nil if the map is nil or empty.
func (m Mapper[K, V]) Keys() []K {
	if m == nil {
		return nil
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m Mapper[K, V]) Clear() {
	if m == nil {
		return
	}
	for k := range m {
		delete(m, k)
	}
}

// Values returns a slice containing all values in the map.
// Returns nil if the map is nil or empty.
func (m Mapper[K, V]) Values() []V {
	if m == nil {
		return nil
	}
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// Each iterates over each key-value pair in the map and calls the provided function.
// Does nothing if the map is nil.
func (m Mapper[K, V]) Each(fn func(K, V)) {
	for k, v := range m {
		fn(k, v)
	}
}

// Filter returns a new Mapper containing only the key-value pairs that satisfy the predicate.
func (m Mapper[K, V]) Filter(fn func(K, V) bool) Mapper[K, V] {
	result := NewMapper[K, V]()
	for k, v := range m {
		if fn(k, v) {
			result[k] = v
		}
	}
	return result
}

// MapValues returns a new Mapper with the same keys but values transformed by the provided function.
func (m Mapper[K, V]) MapValues(fn func(V) V) Mapper[K, V] {
	result := NewMapper[K, V]()
	for k, v := range m {
		result[k] = fn(v)
	}
	return result
}

// Clone returns a shallow copy of the Mapper.
func (m Mapper[K, V]) Clone() Mapper[K, V] {
	result := NewMapper[K, V]()
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Slicer converts the Mapper to a Slicer of key-value pairs.
func (m Mapper[K, V]) Slicer() Slicer[KeyValuePair[K, V]] {
	if m == nil {
		return nil
	}
	result := make(Slicer[KeyValuePair[K, V]], 0, len(m))
	for k, v := range m {
		result = append(result, KeyValuePair[K, V]{Key: k, Value: v})
	}
	return result
}

func (m Mapper[K, V]) SortedKeys() []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := any(keys[i]), any(keys[j])

		switch va := a.(type) {
		case int:
			if vb, ok := b.(int); ok {
				return va < vb
			}
		case int32:
			if vb, ok := b.(int32); ok {
				return va < vb
			}
		case int64:
			if vb, ok := b.(int64); ok {
				return va < vb
			}
		case uint:
			if vb, ok := b.(uint); ok {
				return va < vb
			}
		case uint64:
			if vb, ok := b.(uint64); ok {
				return va < vb
			}
		case float32:
			if vb, ok := b.(float32); ok {
				return va < vb
			}
		case float64:
			if vb, ok := b.(float64); ok {
				return va < vb
			}
		case string:
			if vb, ok := b.(string); ok {
				return va < vb
			}
		}

		// fallback to string comparison
		return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
	})

	return keys
}

func NewBoolMapper[K comparable](keys ...K) Mapper[K, bool] {
	if len(keys) == 0 {
		return nil
	}
	mapper := NewMapper[K, bool]()
	for _, key := range keys {
		mapper.Set(key, true)
	}
	return mapper
}

func NewIntMapper[K comparable](keys ...K) Mapper[K, int] {
	if len(keys) == 0 {
		return nil
	}
	mapper := NewMapper[K, int]()
	for _, key := range keys {
		mapper.Set(key, 0)
	}
	return mapper
}

func NewIdentityMapper[K comparable](keys ...K) Mapper[K, K] {
	if len(keys) == 0 {
		return nil
	}
	mapper := NewMapper[K, K]()
	for _, key := range keys {
		mapper.Set(key, key)
	}
	return mapper
}
