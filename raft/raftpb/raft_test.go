// Copyright 2021 The etcd Authors
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

package raftpb

import (
	"math/bits"
	"testing"
	"unsafe"
)

func TestProtoMemorySizes(t *testing.T) {
	assert := func(size, exp uintptr, name string) {
		t.Helper()
		if size != exp {
			t.Errorf("expected size of %s proto to be %d bytes, found %d bytes", name, exp, size)
		}
	}

	if64Bit := func(yes, no uintptr) uintptr {
		if bits.UintSize == 64 {
			return yes
		}
		return no
	}

	var e Entry
	assert(unsafe.Sizeof(e), if64Bit(48, 32), "Entry")

	var sm SnapshotMetadata
	assert(unsafe.Sizeof(sm), if64Bit(120, 68), "SnapshotMetadata")

	var s Snapshot
	assert(unsafe.Sizeof(s), if64Bit(144, 80), "Snapshot")

	var m Message
	assert(unsafe.Sizeof(m), if64Bit(264, 168), "Message")

	var hs HardState
	assert(unsafe.Sizeof(hs), 24, "HardState")

	var cs ConfState
	assert(unsafe.Sizeof(cs), if64Bit(104, 52), "ConfState")

	var cc ConfChange
	assert(unsafe.Sizeof(cc), if64Bit(48, 32), "ConfChange")

	var ccs ConfChangeSingle
	assert(unsafe.Sizeof(ccs), if64Bit(16, 12), "ConfChangeSingle")

	var ccv2 ConfChangeV2
	assert(unsafe.Sizeof(ccv2), if64Bit(56, 28), "ConfChangeV2")
}
