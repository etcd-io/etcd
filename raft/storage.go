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

// ErrStorageEmpty is returned by Storage.GetLastIndex when there is
// no data.
var ErrStorageEmpty = errors.New("storage is empty")

// Storage is an interface that may be implemented by the application
// to retrieve log entries from storage.
//
// If any Storage method returns an error, the raft instance will
// become inoperable and refuse to participate in elections; the
// application is responsible for cleanup and recovery in this case.
type Storage interface {
	// GetEntries returns a slice of log entries in the range [lo,hi).
	GetEntries(lo, hi uint64) ([]pb.Entry, error)
	// GetLastIndex returns the index of the last entry in the log.
	// If the log is empty it returns ErrStorageEmpty.
	GetLastIndex() (uint64, error)
	// GetFirstIndex returns the index of the first log entry that is
	// available via GetEntries (older entries have been incorporated
	// into the latest Snapshot).
	GetFirstIndex() (uint64, error)
	// Compact discards all log entries prior to i, creating a snapshot
	// which can be used to reconstruct the state at that point.
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
	return &MemoryStorage{}
}

// GetEntries implements the Storage interface.
func (ms *MemoryStorage) GetEntries(lo, hi uint64) ([]pb.Entry, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.ents[lo-ms.offset : hi-ms.offset], nil
}

// GetLastIndex implements the Storage interface.
func (ms *MemoryStorage) GetLastIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	if len(ms.ents) == 0 {
		return 0, ErrStorageEmpty
	}
	return ms.offset + uint64(len(ms.ents)) - 1, nil
}

// GetFirstIndex implements the Storage interface.
func (ms *MemoryStorage) GetFirstIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.offset, nil
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
