// Copyright 2017 The etcd Authors
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

// Package keyutil exports utility functions related to etcd3's key name.

package keyutil

// NextKey returns a name of next key of the given key. The key and the next key don't have any keys between them.
func NextKey(key []byte) []byte {
	for i := len(key) - 1; 0 <= i; i-- {
		if key[i] < 0xff {
			key[i]++
			return key[:i+1]
		}
	}

	return []byte{0}
}

// MkGteRange determines if the range end is a >= range. This works around grpc
// sending empty byte strings as nil; >= is encoded in the range end as '\0'.
// If it is a GTE range, then []byte{} is returned to indicate the empty byte
// string (vs nil being no byte string).
func MkGteRange(rangeEnd []byte) []byte {
	if len(rangeEnd) == 1 && rangeEnd[0] == 0 {
		return []byte{}
	}
	return rangeEnd
}
