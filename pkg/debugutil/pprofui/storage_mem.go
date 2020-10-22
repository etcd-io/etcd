// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package pprofui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type record struct {
	id string
	t  time.Time
	b  []byte
}

// A MemStorage is a Storage implementation that holds recent profiles in memory.
type MemStorage struct {
	mu struct {
		sync.Mutex
		records []record // sorted by record.t
	}
	idGen        int32         // accessed atomically
	keepDuration time.Duration // zero for disabled
	keepNumber   int           // zero for disabled
}

var _ Storage = &MemStorage{}

// NewMemStorage creates a MemStorage that retains the most recent n records
// as long as they are less than d old.
//
// Records are dropped only when there is activity (i.e. an old record will
// only be dropped the next time the storage is accessed).
func NewMemStorage(n int, d time.Duration) *MemStorage {
	return &MemStorage{
		keepNumber:   n,
		keepDuration: d,
	}
}

// ID implements Storage.
func (s *MemStorage) ID() string {
	return fmt.Sprint(atomic.AddInt32(&s.idGen, 1))
}

func (s *MemStorage) cleanLocked() {
	if l, m := len(s.mu.records), s.keepNumber; l > m && m != 0 {
		s.mu.records = append([]record(nil), s.mu.records[l-m:]...)
	}
	now := time.Now()
	if pos := sort.Search(len(s.mu.records), func(i int) bool {
		return s.mu.records[i].t.Add(s.keepDuration).After(now)
	}); pos < len(s.mu.records) && s.keepDuration != 0 {
		s.mu.records = append([]record(nil), s.mu.records[pos:]...)
	}
}

// Store implements Storage.
func (s *MemStorage) Store(id string, write func(io.Writer) error) error {
	var b bytes.Buffer
	if err := write(&b); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mu.records = append(s.mu.records, record{id: id, t: time.Now(), b: b.Bytes()})
	sort.Slice(s.mu.records, func(i, j int) bool {
		return s.mu.records[i].t.Before(s.mu.records[j].t)
	})
	s.cleanLocked()
	return nil
}

// Get implements Storage.
func (s *MemStorage) Get(id string, read func(io.Reader) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.mu.records {
		if v.id == id {
			return read(bytes.NewReader(v.b))
		}
	}
	return errors.New("profile not found; it may have expired")
}
