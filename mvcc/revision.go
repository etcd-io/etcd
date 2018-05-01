// Copyright 2015 The etcd Authors
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

package mvcc

import (
	"encoding/binary"

	"go.uber.org/zap"
)

const (
	// revBytesLen is the byte length of a normal revision.
	// The first 8-byte is the revision.main in big-endian format.
	// The 9th byte is a '_'.
	// The last 8-byte is the revision.sub in big-endian format.
	revBytesLen = 8 + 1 + 8

	// markedRevBytesLen is the byte length of marked revision.
	// The first `revBytesLen` bytes represents a normal revision. The last
	// one byte is the mark.
	markedRevBytesLen = revBytesLen + 1
	markBytePosition  = markedRevBytesLen - 1

	markTombstone byte = 't'
)

// Revision indicates modification of the key-value space.
// The set of changes that share same main revision changes the key-value space atomically.
type Revision struct {
	// Main is the main revision of a set of changes that happen atomically.
	Main int64

	// Sub is the the sub revision of a change in a set of changes that happen
	// atomically. Each change has different increasing sub revision in that
	// set.
	Sub int64
}

func (a Revision) GreaterThan(b Revision) bool {
	if a.Main > b.Main {
		return true
	}
	if a.Main < b.Main {
		return false
	}
	return a.Sub > b.Sub
}

type revisions []Revision

func (rs revisions) Len() int           { return len(rs) }
func (rs revisions) Less(i, j int) bool { return rs[j].GreaterThan(rs[i]) }
func (rs revisions) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

func NewRevBytes() []byte {
	return make([]byte, revBytesLen, markedRevBytesLen)
}

func RevToBytes(rev Revision, bytes []byte) {
	binary.BigEndian.PutUint64(bytes, uint64(rev.Main))
	bytes[8] = '_'
	binary.BigEndian.PutUint64(bytes[9:], uint64(rev.Sub))
}

func BytesToRev(bytes []byte) Revision {
	return Revision{
		Main: int64(binary.BigEndian.Uint64(bytes[0:8])),
		Sub:  int64(binary.BigEndian.Uint64(bytes[9:])),
	}
}

// appendMarkTombstone appends tombstone mark to normal revision bytes.
func appendMarkTombstone(lg *zap.Logger, b []byte) []byte {
	if len(b) != revBytesLen {
		if lg != nil {
			lg.Panic(
				"cannot append tombstone mark to non-normal revision bytes",
				zap.Int("expected-revision-bytes-size", revBytesLen),
				zap.Int("given-revision-bytes-size", len(b)),
			)
		} else {
			plog.Panicf("cannot append mark to non normal revision bytes")
		}
	}
	return append(b, markTombstone)
}

// isTombstone checks whether the revision bytes is a tombstone.
func isTombstone(b []byte) bool {
	return len(b) == markedRevBytesLen && b[markBytePosition] == markTombstone
}
