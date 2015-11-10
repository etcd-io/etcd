// Copyright 2015 CoreOS, Inc.
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

package storage

import (
	"encoding/binary"

	"github.com/coreos/etcd/storage/storagepb"
)

// backendKeyBytesLen is the length of backendKey bytes.
// First a few bytes stores the revision of backendKey. The following byte
// is a ' ' as delimiter. The last 4 bytes is the event type in big-endian
// format.
const backendKeyBytesLen = revBytesLen + 1 + 4

type backendKey struct {
	rev revision
	t   storagepb.Event_EventType
}

func newBackendKeyBytes() []byte {
	return make([]byte, backendKeyBytesLen)
}

// backendKeyToBytes converts backendKey to bytes.
// The lexicograph order of backendKey's byte slice representation
// is the same as the order of its revision.
func backendKeyToBytes(key backendKey, bytes []byte) {
	revToBytes(key.rev, bytes[:revBytesLen])
	bytes[revBytesLen] = ' '
	binary.BigEndian.PutUint32(bytes[revBytesLen+1:], uint32(key.t))
}

func bytesToBackendKey(bytes []byte) backendKey {
	return backendKey{
		rev: bytesToRev(bytes[:revBytesLen]),
		t:   storagepb.Event_EventType(binary.BigEndian.Uint32(bytes[revBytesLen+1:])),
	}
}

func backendKeyBytesRange(rev revision) (start, end []byte) {
	start = newRevBytes()
	revToBytes(rev, start)

	end = newBackendKeyBytes()
	endRev := revision{main: rev.main, sub: rev.sub + 1}
	revToBytes(endRev, end)

	return start, end
}
