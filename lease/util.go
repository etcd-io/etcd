// Copyright 2018 The etcd Authors
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

package lease

import (
	"encoding/binary"
	"fmt"
)

const idBytesLen = 8

// IDToBytes encodes int64 lease ID in big-endian binary format.
func IDToBytes(n int64) []byte {
	bytes := make([]byte, idBytesLen)
	binary.BigEndian.PutUint64(bytes, uint64(n))
	return bytes
}

// BytesToID decodes big-endian binary format lease ID to int64.
func BytesToID(d []byte) int64 {
	if len(d) != idBytesLen {
		panic(fmt.Errorf("lease ID must be 8-byte, got %d", len(d)))
	}
	return int64(binary.BigEndian.Uint64(d))
}
