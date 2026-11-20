// Copyright 2026 The etcd Authors
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
	"strings"
	"testing"
)

func TestBytesToRev(t *testing.T) {
	b := make([]byte, revBytesLen)
	revToBytes(b, revision{main: 2, sub: 3})
	if rev, err := bytesToRev(b); err != nil || rev != (revision{main: 2, sub: 3}) {
		t.Errorf("bytesToRev(17 bytes) = %+v, %v, want {2 3}, nil", rev, err)
	}

	// Marked (tombstone) revisions are 18 bytes and must still decode.
	marked := make([]byte, markedRevBytesLen)
	copy(marked, b)
	marked[revBytesLen] = 't'
	if rev, err := bytesToRev(marked); err != nil || rev != (revision{main: 2, sub: 3}) {
		t.Errorf("bytesToRev(18 bytes) = %+v, %v, want {2 3}, nil", rev, err)
	}
}

func TestBytesToRevInvalidLength(t *testing.T) {
	for _, n := range []int{0, 7, 8, 9, 16, 19} {
		_, err := bytesToRev(make([]byte, n))
		if err == nil {
			t.Errorf("bytesToRev(%d bytes): expected error, got nil", n)
			continue
		}
		if !strings.Contains(err.Error(), "invalid revision length") {
			t.Errorf("bytesToRev(%d bytes): error = %v, want \"invalid revision length\"", n, err)
		}
	}
}
