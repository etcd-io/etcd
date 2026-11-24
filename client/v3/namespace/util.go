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

package namespace

func prefixInterval(pfx string, key, end []byte) (pfxKey []byte, pfxEnd []byte) {
	pfxKey = make([]byte, len(pfx)+len(key))
	copy(pfxKey[copy(pfxKey, pfx):], key)

	if len(end) == 1 && end[0] == 0 {
		// the edge of the keyspace
		pfxEnd = []byte{0}
		for i := len(pfx) - 1; i >= 0; i-- {
			if pfx[i] < 0xff {
				// drop trailing 0xff bytes, then increment the last byte
				pfxEnd = append([]byte(pfx[:i]), pfx[i]+1)
				break
			}
		}
	} else if len(end) >= 1 {
		pfxEnd = make([]byte, len(pfx)+len(end))
		copy(pfxEnd[copy(pfxEnd, pfx):], end)
	}

	return pfxKey, pfxEnd
}
