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

// bytesToRev should be synced with function in server
// https://github.com/etcd-io/etcd/blob/main/server/storage/mvcc/revision.go
func bytesToRev(bytes []byte) revision {
	return revision{
		main: int64(binary.BigEndian.Uint64(bytes[0:8])),
		sub:  int64(binary.BigEndian.Uint64(bytes[9:])),
	}
}

// revToBytes should be synced with function in server
// https://github.com/etcd-io/etcd/blob/main/server/storage/mvcc/revision.go
func revToBytes(bytes []byte, rev revision) {
	binary.BigEndian.PutUint64(bytes[0:8], uint64(rev.main))
	bytes[8] = '_'
	binary.BigEndian.PutUint64(bytes[9:], uint64(rev.sub))
}
