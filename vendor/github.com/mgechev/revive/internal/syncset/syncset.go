// Package syncset provides a simple, mutex-protected set for strings.
package syncset

import (
	"maps"
	"slices"
	"sync"
)

// Set is a concurrency-safe set of strings.
type Set struct {
	mu       sync.Mutex
	elements map[string]struct{}
}

// New returns an initialized, empty Set.
func New() *Set {
	return &Set{elements: map[string]struct{}{}}
}

// AddIfAbsent adds str to the set if it is not already present, and reports whether it was added.
func (s *Set) AddIfAbsent(str string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.elements == nil {
		s.elements = map[string]struct{}{str: {}}
		return true
	}

	_, exists := s.elements[str]
	if !exists {
		s.elements[str] = struct{}{}
	}
	return !exists
}

// Elements returns a slice of all elements in the set.
func (s *Set) Elements() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Collect(maps.Keys(s.elements))
}
