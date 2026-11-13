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

package snapshot

import (
	"encoding/binary"
	"fmt"
)

type revision struct {
	main int64
	sub  int64
}

// GreaterThan should be synced with function in server
// https://github.com/etcd-io/etcd/blob/main/server/storage/mvcc/revision.go
func (a revision) GreaterThan(b revision) bool {
	if a.main > b.main {
		return true
	}
	if a.main < b.main {
		return false
	}
	return a.sub > b.sub
}

// revBytesLen is the byte length of a normal revision.
// First 8 bytes is the revision.main in big-endian format. The 9th byte
// is a '_'. The last 8 bytes is the revision.sub in big-endian format.
const revBytesLen = 8 + 1 + 8

// markedRevBytesLen is the byte length of marked revision.
// The first `revBytesLen` bytes represents a normal revision. The last
// one byte is the mark.
const markedRevBytesLen = revBytesLen + 1

// bytesToRev should be synced with function in server
// https://github.com/etcd-io/etcd/blob/main/server/storage/mvcc/revision.go
func bytesToRev(bytes []byte) (revision, error) {
	if len(bytes) != revBytesLen && len(bytes) != markedRevBytesLen {
		return revision{}, fmt.Errorf("invalid revision length: %d", len(bytes))
	}
	return revision{
		main: int64(binary.BigEndian.Uint64(bytes[0:8])),
		sub:  int64(binary.BigEndian.Uint64(bytes[9:])),
	}, nil
}

// revToBytes should be synced with function in server
// https://github.com/etcd-io/etcd/blob/main/server/storage/mvcc/revision.go
func revToBytes(bytes []byte, rev revision) {
	binary.BigEndian.PutUint64(bytes[0:8], uint64(rev.main))
	bytes[8] = '_'
	binary.BigEndian.PutUint64(bytes[9:], uint64(rev.sub))
}
