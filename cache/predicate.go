// Copyright 2025 The etcd Authors
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

package cache

import "bytes"

type Prefix string

func (prefix Prefix) Match(key []byte) bool {
	if prefix == "" {
		return true
	}
	prefixLen := len(prefix)
	return len(key) >= prefixLen && string(key[:prefixLen]) == string(prefix)
}

func ExactKey(key []byte) KeyPredicate {
	return func(k []byte) bool { return bytes.Equal(k, key) }
}

func FromKey(start []byte) KeyPredicate {
	return func(k []byte) bool { return bytes.Compare(k, start) >= 0 }
}

func Range(start, end []byte) KeyPredicate {
	return func(k []byte) bool {
		return bytes.Compare(k, start) >= 0 &&
			bytes.Compare(k, end) < 0
	}
}

func KeyPredForRange(start, end []byte) KeyPredicate {
	if len(end) == 0 {
		return ExactKey(start)
	}
	if len(end) == 1 && end[0] == 0 {
		return FromKey(start)
	}
	return Range(start, end)
}
