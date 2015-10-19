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

package rafthttp

import (
	"io"
)

// snapshotStore is the store of snapshot. Caller could put one
// snapshot into the store, and get it later.
// snapshotStore stores at most one snapshot at a time, or it panics.
type snapshotStore struct {
	rc io.ReadCloser
	// index of the stored snapshot
	// index is 0 if and only if there is no snapshot stored.
	index uint64
}

func (s *snapshotStore) put(rc io.ReadCloser, index uint64) {
	if s.index != 0 {
		plog.Panicf("unexpected put when there is one snapshot stored")
	}
	s.rc, s.index = rc, index
}

func (s *snapshotStore) get(index uint64) io.ReadCloser {
	if s.index == index {
		// set index to 0 to indicate no snapshot stored
		s.index = 0
		return s.rc
	}
	return nil
}
