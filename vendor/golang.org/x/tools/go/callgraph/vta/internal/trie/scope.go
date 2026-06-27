// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trie

import (
	"strconv"
	"sync/atomic"
)

// Scope represents a distinct collection of maps.
// Maps with the same Scope can be equal. Maps in different scopes are distinct.
// Each Builder creates maps within a unique Scope.
type Scope struct {
	id int32
}

var nextScopeId atomic.Int32

func newScope() Scope {
	id := nextScopeId.Add(1)
	return Scope{id: id}
}

func (s Scope) String() string {
	return strconv.Itoa(int(s.id))
}
