// Copyright ©2020 The go-latex Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package symbols

import (
	"sort"
)

type Set map[string]struct{}

func NewSet(vs ...string) Set {
	o := make(Set, len(vs))
	for _, k := range vs {
		o[k] = struct{}{}
	}
	return o
}

func (set Set) Has(k string) bool {
	_, ok := set[k]
	return ok
}

func (set Set) Keys() []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func UnionOf(sets ...Set) Set {
	o := make(Set, len(sets))
	for _, set := range sets {
		for k := range set {
			o[k] = struct{}{}
		}
	}
	return o
}
