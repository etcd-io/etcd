/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package raft

import (
	"errors"
	"sync"

	pb "github.com/coreos/etcd/raft/raftpb"
)

// ErrSnapshotRequired is returned by Storage.Entries when a requested
// index is unavailable because it predates the last snapshot.
var ErrSnapshotRequired = errors.New("snapshot required; requested index is too old")

// Storage is an interface that may be implemented by the application
// to retrieve log entries from storage.
//
// If any Storage method returns an error, the raft instance will
// become inoperable and refuse to participate in elections; the
// application is responsible for cleanup and recovery in this case.
type Storage interface {
	// Entries returns a slice of log entries in the range [lo,hi).
	Entries(lo, hi uint64) ([]pb.Entry, error)
	// Term returns the term of entry i, which must be in the range
	// [FirstIndex()-1, LastIndex()]. The term of the entry before
	// FirstIndex is retained for matching purposes even though the
	// rest of that entry may not be available.
	Term(i uint64) (uint64, error)
	// LastIndex returns the index of the last entry in the log.
	LastIndex() (uint64, error)
	// FirstIndex returns the index of the first log entry that is
	// available via Entries (older entries have been incorporated
	// into the latest Snapshot).
	FirstIndex() (uint64, error)
	// Compact discards all log entries prior to i.
	// TODO(bdarnell): Create a snapshot which can be used to
	// reconstruct the state at that point.
	Compact(i uint64) error
}

// MemoryStorage implements the Storage interface backed by an
// in-memory array.
type MemoryStorage struct {
	// Protects access to all fields. Most methods of MemoryStorage are
	// run on the raft goroutine, but Append() is run on an application
	// goroutine.
	sync.Mutex

	ents []pb.Entry
	// offset is the position of the last compaction.
	// ents[i] has raft log position i+offset.
	offset uint64
}

// NewMemoryStorage creates an empty MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		// When starting from scratch populate the list with a dummy entry at term zero.
		ents: make([]pb.Entry, 1),
	}
}

// Entries implements the Storage interface.
func (ms *MemoryStorage) Entries(lo, hi uint64) ([]pb.Entry, error) {
	ms.Lock()
	defer ms.Unlock()
	if lo <= ms.offset {
		return nil, ErrSnapshotRequired
	}
	return ms.ents[lo-ms.offset : hi-ms.offset], nil
}

// Term implements the Storage interface.
func (ms *MemoryStorage) Term(i uint64) (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	if i < ms.offset || i > ms.offset+uint64(len(ms.ents)) {
		return 0, ErrSnapshotRequired
	}
	return ms.ents[i-ms.offset].Term, nil
}

// LastIndex implements the Storage interface.
func (ms *MemoryStorage) LastIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.offset + uint64(len(ms.ents)) - 1, nil
}

// FirstIndex implements the Storage interface.
func (ms *MemoryStorage) FirstIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.offset + 1, nil
}

// Compact implements the Storage interface.
func (ms *MemoryStorage) Compact(i uint64) error {
	ms.Lock()
	defer ms.Unlock()
	i -= ms.offset
	ents := make([]pb.Entry, 1, 1+uint64(len(ms.ents))-i)
	ents[0].Term = ms.ents[i].Term
	ents = append(ents, ms.ents[i+1:]...)
	ms.ents = ents
	ms.offset += i
	return nil
}

// Append the new entries to storage.
func (ms *MemoryStorage) Append(entries []pb.Entry) {
	ms.Lock()
	defer ms.Unlock()
	ms.ents = append(ms.ents, entries...)
}
