package lx

import (
	"fmt"
	"strings"
)

// Field represents a key-value pair where the key is a string and the value is of any type.
type Field struct {
	Key   string
	Value interface{}
}

// Fields represents a slice of key-value pairs.
type Fields []Field

// Map converts the Fields slice to a map[string]interface{}.
// This is useful for backward compatibility or when map operations are needed.
// Example:
//
//	fields := lx.Fields{{"user", "alice"}, {"age", 30}}
//	m := fields.Map() // Returns map[string]interface{}{"user": "alice", "age": 30}
func (f Fields) Map() map[string]interface{} {
	m := make(map[string]interface{}, len(f))
	for _, pair := range f {
		m[pair.Key] = pair.Value
	}
	return m
}

// Get returns the value for a given key and a boolean indicating if the key was found.
// This provides O(n) lookup, which is fine for small numbers of fields.
// Example:
//
//	fields := lx.Fields{{"user", "alice"}, {"age", 30}}
//	value, found := fields.Get("user") // Returns "alice", true
func (f Fields) Get(key string) (interface{}, bool) {
	for _, pair := range f {
		if pair.Key == key {
			return pair.Value, true
		}
	}
	return nil, false
}

// Filter returns a new Fields slice containing only pairs where the predicate returns true.
// Example:
//
//	fields := lx.Fields{{"user", "alice"}, {"password", "secret"}, {"age", 30}}
//	filtered := fields.Filter(func(key string, value interface{}) bool {
//	    return key != "password" // Remove sensitive fields
//	})
func (f Fields) Filter(predicate func(key string, value interface{}) bool) Fields {
	result := make(Fields, 0, len(f))
	for _, pair := range f {
		if predicate(pair.Key, pair.Value) {
			result = append(result, pair)
		}
	}
	return result
}

// Translate returns a new Fields slice with keys translated according to the provided mapping.
// Keys not in the mapping are passed through unchanged. This is useful for adapters like Victoria.
// Example:
//
//	fields := lx.Fields{{"user", "alice"}, {"timestamp", time.Now()}}
//	translated := fields.Translate(map[string]string{
//	    "user": "username",
//	    "timestamp": "ts",
//	})
//	// Returns: {{"username", "alice"}, {"ts", time.Now()}}
func (f Fields) Translate(mapping map[string]string) Fields {
	result := make(Fields, len(f))
	for i, pair := range f {
		if newKey, ok := mapping[pair.Key]; ok {
			result[i] = Field{Key: newKey, Value: pair.Value}
		} else {
			result[i] = pair
		}
	}
	return result
}

// Merge merges another Fields slice into this one, with the other slice's fields taking precedence
// for duplicate keys (overwrites existing keys).
// Example:
//
//	base := lx.Fields{{"user", "alice"}, {"age", 30}}
//	additional := lx.Fields{{"age", 31}, {"city", "NYC"}}
//	merged := base.Merge(additional)
//	// Returns: {{"user", "alice"}, {"age", 31}, {"city", "NYC"}}
func (f Fields) Merge(other Fields) Fields {
	result := make(Fields, 0, len(f)+len(other))

	// Create a map to track which keys from 'other' we've seen
	seen := make(map[string]bool, len(other))

	// First add all fields from 'f'
	result = append(result, f...)

	// Then add fields from 'other', overwriting duplicates
	for _, pair := range other {
		// Check if this key already exists in result
		found := false
		for i, existing := range result {
			if existing.Key == pair.Key {
				result[i] = pair // Overwrite
				found = true
				break
			}
		}
		if !found {
			result = append(result, pair)
		}
		seen[pair.Key] = true
	}

	return result
}

// String returns a human-readable string representation of the fields.
// Example:
//
//	fields := lx.Fields{{"user", "alice"}, {"age", 30}}
//	str := fields.String() // Returns: "[user=alice age=30]"
func (f Fields) String() string {
	var builder strings.Builder
	builder.WriteString(LeftBracket)
	for i, pair := range f {
		if i > 0 {
			builder.WriteString(Space)
		}
		builder.WriteString(pair.Key)
		builder.WriteString("=")
		builder.WriteString(fmt.Sprint(pair.Value))
	}
	builder.WriteString(RightBracket)
	return builder.String()
}
