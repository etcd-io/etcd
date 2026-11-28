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

// Package indexmodel checks concurrent MVCC index calls by replaying write transactions.
package indexmodel

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
)

// Revision is a local copy of mvcc.Revision to avoid an import cycle in mvcc tests.
type Revision struct {
	Main, Sub int64
}

// Kind names the index method in a recorded call.
type Kind string

const (
	Put            Kind = "Put"
	Tombstone      Kind = "Tombstone"
	Get            Kind = "Get"
	Range          Kind = "Range"
	Revisions      Kind = "Revisions"
	CountRevisions Kind = "CountRevisions"
	Keep           Kind = "Keep"
)

// Op records a call at main revision Rev. For writes, Sub counts earlier successful
// writes in the transaction. Range reads use [Key, End). A nil End selects only Key.
// An empty, non-nil End selects all keys >= Key.
type Op struct {
	Kind           Kind
	Key, End       []byte
	Rev, Sub       int64
	Limit          int
	WithTotalCount bool
}

func (op Op) String() string {
	switch op.Kind {
	case Put, Tombstone:
		return fmt.Sprintf("%s(%q, rev %d.%d)", op.Kind, op.Key, op.Rev, op.Sub)
	case Get:
		return fmt.Sprintf("%s(%q, rev %d)", op.Kind, op.Key, op.Rev)
	case Range, Revisions:
		return fmt.Sprintf("%s(%s, rev %d, limit %d, total %t)",
			op.Kind, op.formatKeyRange(), op.Rev, op.Limit, op.WithTotalCount)
	case CountRevisions:
		return fmt.Sprintf("%s(%s, rev %d)", op.Kind, op.formatKeyRange(), op.Rev)
	case Keep:
		return fmt.Sprintf("%s(rev %d)", op.Kind, op.Rev)
	default:
		panic(fmt.Sprintf("unsupported op kind %q", op.Kind))
	}
}

func (op Op) formatKeyRange() string {
	switch {
	case op.End == nil:
		return fmt.Sprintf("%q", op.Key)
	case len(op.End) == 0:
		// The API uses "\x00" as the end of an unbounded range.
		return fmt.Sprintf("[%q, %q)", op.Key, "\x00")
	default:
		return fmt.Sprintf("[%q, %q)", op.Key, op.End)
	}
}

func (op Op) matchesKey(key []byte) bool {
	if op.End == nil {
		return bytes.Equal(key, op.Key)
	}
	return bytes.Compare(key, op.Key) >= 0 && (len(op.End) == 0 || bytes.Compare(key, op.End) < 0)
}

// Item represents one key. Get and Range use all fields. For Revisions, the
// model records each key's last modification revision as Item{Modified: rev}.
type Item struct {
	Key               string
	Modified, Created Revision
	Version           int64
}

// Result records the return values of an index call. Each kind sets only the
// fields that its index method returns:
//
//	Put             nothing
//	Tombstone       Err
//	Get             Err, Items
//	Range           Items, Total
//	Revisions       Items, Total
//	CountRevisions  Total
//	Keep            Kept
type Result struct {
	// Err records ErrRevisionNotFound from Get or Tombstone.
	Err bool
	// Items holds the returned keys in key order. Get always records one Item
	// for Op.Key. If the key doesn't exist, that Item has zero revisions and
	// version 0, since the store's put adds one to the version even if Get
	// returns an error.
	Items []Item
	// Total is the number of keys in the range. If Op.Limit cuts Items, Total
	// counts all the keys only with Op.WithTotalCount. Otherwise, it's len(Items).
	Total int
	// Kept holds the revisions returned by Keep.
	Kept map[Revision]struct{}
}

// A snapshot holds the live keys at currentRev and that revision's tombstones.
// writtenKeys holds the key of each write at currentRev, in sub-revision order.
type snapshot struct {
	currentRev  int64
	writtenKeys []string
	keys        map[string]Item
	tombstones  []Revision
}

func (s snapshot) cloneForNextRevision() snapshot {
	return snapshot{currentRev: s.currentRev + 1, keys: maps.Clone(s.keys)}
}

func (s *snapshot) write(op Op) (Result, error) {
	if op.Rev != s.currentRev {
		return Result{}, fmt.Errorf("invalid write revision %d, expected %d", op.Rev, s.currentRev)
	}
	if op.Sub != int64(len(s.writtenKeys)) {
		return Result{}, fmt.Errorf("invalid sub-revision of %v, expected %d", op, len(s.writtenKeys))
	}

	key := string(op.Key)
	item, exists := s.keys[key]
	if op.Kind == Tombstone && !exists {
		return Result{Err: true}, nil
	}

	if op.Kind == Put && slices.Contains(s.writtenKeys, key) {
		return Result{}, fmt.Errorf("invalid write %v, the transaction already wrote the key", op)
	}

	rev := Revision{Main: s.currentRev, Sub: op.Sub}
	switch op.Kind {
	case Tombstone:
		delete(s.keys, key)
		s.tombstones = append(s.tombstones, rev)
	case Put:
		if !exists {
			item = Item{Key: key, Created: rev}
		}
		item.Modified = rev
		item.Version++
		s.keys[key] = item
	default:
		panic(fmt.Sprintf("unsupported op %s in write", op))
	}
	s.writtenKeys = append(s.writtenKeys, key)
	return Result{}, nil
}

// The caller selects the snapshot for op.Rev.
func (s snapshot) read(op Op) Result {
	switch op.Kind {
	case Get:
		// A missing key has zero revisions and version 0.
		key := string(op.Key)
		item, ok := s.keys[key]
		item.Key = key
		return Result{Err: !ok, Items: []Item{item}}
	case Range, Revisions, CountRevisions:
		return s.readRange(op)
	case Keep:
		return Result{Kept: s.liveKeyModRevisions()}
	default:
		panic(fmt.Sprintf("unsupported op %s in read", op))
	}
}

func (s snapshot) liveKeyModRevisions() map[Revision]struct{} {
	kept := make(map[Revision]struct{}, len(s.keys))
	for _, item := range s.keys {
		kept[item.Modified] = struct{}{}
	}
	return kept
}

func (s snapshot) revisionsKeptByCompact() map[Revision]struct{} {
	kept := s.liveKeyModRevisions()
	// Watches starting at currentRev need these tombstones to report deletes.
	for _, tombstone := range s.tombstones {
		kept[tombstone] = struct{}{}
	}
	return kept
}

func (s snapshot) readRange(op Op) Result {
	var keys []string
	for key := range s.keys {
		if op.matchesKey([]byte(key)) {
			keys = append(keys, key)
		}
	}
	result := Result{Total: len(keys)}
	if op.Kind == CountRevisions {
		return result
	}

	slices.Sort(keys)
	if op.Limit > 0 && len(keys) > op.Limit {
		keys = keys[:op.Limit]
		if !op.WithTotalCount {
			result.Total = len(keys)
		}
	}
	for _, key := range keys {
		item := s.keys[key]
		if op.Kind == Revisions {
			item = Item{Modified: item.Modified}
		}
		result.Items = append(result.Items, item)
	}
	return result
}
