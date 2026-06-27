// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collections

type Set[T comparable] map[T]struct{}

func (s Set[T]) Add(el ...T) {
	for _, el := range el {
		s[el] = struct{}{}
	}
}

func (s Set[T]) Remove(el T) bool {
	if !s.Contains(el) {
		return false
	}
	delete(s, el)
	return true
}

func (s Set[T]) Contains(el T) bool {
	_, ok := s[el]
	return ok
}

func (s Set[T]) ToSlice() []T {
	sl := []T{}
	for el := range s {
		sl = append(sl, el)
	}
	return sl
}

func (s Set[T]) Clone() Set[T] {
	newSet := Set[T]{}
	for el := range s {
		newSet.Add(el)
	}
	return newSet
}

func (s Set[T]) Equals(rhs Set[T]) bool {
	if len(s) != len(rhs) {
		return false
	}
	rhsClone := rhs.Clone()
	for el := range s {
		rhsClone.Remove(el)
	}
	return len(rhsClone) == 0
}

func SliceToSet[T comparable](sl []T) Set[T] {
	set := Set[T]{}
	for _, el := range sl {
		set.Add(el)
	}
	return set
}
