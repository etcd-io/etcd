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

package auth

import (
	"encoding/binary"
	"fmt"
)

const revBytesLen = 8

// RevToBytes encodes uint64 auth store revision in big-endian binary format.
func RevToBytes(n uint64) []byte {
	bytes := make([]byte, revBytesLen)
	binary.BigEndian.PutUint64(bytes, n)
	return bytes
}

// BytesToRev decodes big-endian binary format auth store revision to uint64.
func BytesToRev(d []byte) uint64 {
	if len(d) != revBytesLen {
		panic(fmt.Errorf("auth store revision must be 8-byte, got %d", len(d)))
	}
	return binary.BigEndian.Uint64(d)
}
