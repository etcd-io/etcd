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

package indexmodel

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func TestHistoryCalls(t *testing.T) {
	// A goroutine makes one call at a time.
	//
	//	overlap, invalid:  | read [0, 3] |
	//	                           | read [2, 4] |
	//	touch, valid:      | read [0, 2] | read [2, 3] |
	read := countKeysAtRev(1, 0)
	first, second := read.during(0, 3), read.during(2, 4)
	twoErrors := apply(
		txn(callAtRev(Put, 2).withError()),
		reader(read.during(1, 2)),
		reader(read.during(4, 3)),
	)

	tests := []struct {
		name    string
		history historyStep
		wantErr string
	}{
		{"writer calls Range", txn(callAtRev(Range, 1)), ""},
		{"writer calls Revisions", txn(callAtRev(Revisions, 1)), ""},
		{"writer calls Keep", txn(callAtRev(Keep, 1)), `writer: unexpected operation kind "Keep"`},
		{"reader calls Range", reader(callAtRev(Range, 1)), ""},
		{"reader calls Revisions", reader(callAtRev(Revisions, 1)), ""},
		{"reader calls Keep", reader(callAtRev(Keep, 1)), ""},
		{"reader calls Put", reader(callAtRev(Put, 2)), `reader 0: unexpected operation kind "Put"`},
		{"reader calls Scan", reader(callAtRev("Scan", 1)), `reader 0: unexpected operation kind "Scan"`},
		{
			"reader uses revision 0", reader(callAtRev(Get, 0)),
			`reader 0: invalid main revision 0 for Get("", rev 0)`,
		},
		{
			"return before call", reader(read.during(4, 3)),
			`reader 0: CountRevisions(["", "\x00"), rev 1): invalid interval [4, 3]`,
		},
		{
			"negative timestamp", reader(read.during(-1, 0)),
			`reader 0: CountRevisions(["", "\x00"), rev 1): invalid interval [-1, 0]`,
		},
		{
			"reader calls overlap", reader(first, second),
			`reader 0: CountRevisions(["", "\x00"), rev 1): call at 2 precedes previous return at 3`,
		},
		{"calls of different readers overlap", apply(reader(first), reader(second)), ""},
		{
			"writer calls overlap", apply(txn(first), txn(second)),
			`writer: CountRevisions(["", "\x00"), rev 1): call at 2 precedes previous return at 3`,
		},
		{
			"writer calls overlap within a transaction", txn(first, second),
			`writer: CountRevisions(["", "\x00"), rev 1): call at 2 precedes previous return at 3`,
		},
		{"reader calls touch", reader(read.during(0, 2), read.during(2, 2)), ""},
		{"writer calls touch", apply(txn(read.during(0, 2)), txn(read.during(2, 3))), ""},
		{
			"validates the readers before the replay", twoErrors,
			`reader 1: CountRevisions(["", "\x00"), rev 1): invalid interval [4, 3]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkHistory(t, tt.history, tt.wantErr)
		})
	}
}

func TestHistoryTransactions(t *testing.T) {
	// Each case writes revision 2 on the empty store.
	//
	//	writes                                          keys          tombstones
	//	putAtRev("a", 2, 0)                             a=2.0
	//	putAtRev("a", 2, 0), putAtRev("b", 2, 1)        a=2.0, b=2.1
	//	putAtRev("a", 2, 0), tombstoneAtRev("a", 2, 1)                2.1
	getMissing := OpRecord{
		Op:  Op{Kind: Get, Key: []byte("a"), Rev: 2},
		Got: Result{Err: true, Items: []Item{{Key: "a"}}},
	}
	wrongVersion := getMissing
	wrongVersion.Got.Items = []Item{{Key: "a", Version: 4}}
	tombstoneMissing := tombstoneAtRev("a", 2, 0).withError()
	putThenDelete := txn(putAtRev("a", 2, 0), tombstoneAtRev("a", 2, 1), getMissing)

	tests := []struct {
		name    string
		history historyStep
		wantErr string
	}{
		{
			name: "no revision without a write",
			history: apply(
				txn(countKeysAtRev(1, 0)),
				txn(tombstoneMissing),
				txn(tombstoneMissing, putAtRev("a", 2, 0)),
			),
		},
		{
			name: "reads and a failed tombstone do not publish a revision",
			history: apply(
				txn(tombstoneMissing, countKeysAtRev(1, 0)),
				reader(countKeysAtRev(2, 0).during(1, 2)),
			),
			wantErr: `CountRevisions(["", "\x00"), rev 2) requests an unavailable revision`,
		},
		{
			name: "reuses revision 2",
			history: apply(
				txn(putAtRev("a", 2, 0)),
				txn(putAtRev("a", 2, 0)),
			),
			wantErr: "invalid write revision 2, expected 3",
		},
		{
			name:    "skips sub-revision 0",
			history: txn(putAtRev("b", 2, 1)),
			wantErr: `invalid sub-revision of Put("b", rev 2.1), expected 0`,
		},
		{
			name:    "reuses sub-revision 0",
			history: txn(putAtRev("a", 2, 0), putAtRev("b", 2, 0)),
			wantErr: `invalid sub-revision of Put("b", rev 2.0), expected 1`,
		},
		{
			name: "publishes a revision after putting and deleting the same key",
			history: apply(
				putThenDelete,
				reader(countKeysAtRev(2, 0).during(1, 2)),
			),
		},
		{
			name: "two writes in one transaction publish only one revision",
			history: apply(
				putThenDelete,
				reader(countKeysAtRev(3, 0).during(1, 2)),
			),
			wantErr: `CountRevisions(["", "\x00"), rev 3) requests an unavailable revision`,
		},
		{
			name:    "puts a key twice",
			history: txn(putAtRev("a", 2, 0), putAtRev("a", 2, 1)),
			wantErr: `invalid write Put("a", rev 2.1), the transaction already wrote the key`,
		},
		{
			name:    "puts a key it deleted",
			history: txn(putAtRev("a", 2, 0), tombstoneAtRev("a", 2, 1), putAtRev("a", 2, 2)),
			wantErr: `invalid write Put("a", rev 2.2), the transaction already wrote the key`,
		},
		{
			name: "sees its writes one by one",
			history: txn(
				countKeysAtRev(1, 0),
				putAtRev("a", 2, 0),
				countKeysAtRev(1, 0), countKeysAtRev(2, 1),
				putAtRev("b", 2, 1), countKeysAtRev(2, 2),
			),
		},
		{
			name:    "reads the working revision before the first write",
			history: txn(countKeysAtRev(2, 0), putAtRev("a", 2, 0)),
			wantErr: `CountRevisions(["", "\x00"), rev 2) requests an unavailable revision`,
		},
		{
			name:    "sees a later put",
			history: txn(putAtRev("a", 2, 0), countKeysAtRev(2, 2), putAtRev("b", 2, 1)),
			wantErr: wantMismatch(`CountRevisions(["", "\x00"), rev 2), time 0 to 0`,
				Result{Total: 1}, Result{Total: 2},
			),
		},
		{
			name:    "wrong version on Get error",
			history: txn(putAtRev("a", 2, 0), tombstoneAtRev("a", 2, 1), wrongVersion),
			wantErr: wantMismatch(`Get("a", rev 2), time 0 to 0`, getMissing.Got, wrongVersion.Got),
		},
		{
			name:    "put returns an error",
			history: txn(putAtRev("a", 2, 0).withError().during(1, 2)),
			wantErr: wantMismatch(`Put("a", rev 2.0), time 1 to 2`, Result{}, Result{Err: true}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkHistory(t, tt.history, tt.wantErr)
		})
	}
}

// Revision 2 is available at 10, once its final call, a read, returned.
//
//	time:    0        4        8             10            20
//	writer:  | Put a  | Put b  | count rev 2 | Tombstone a |
//	         |------------ rev 2 ------------|--- rev 3 ---|
var txnsWithDelete = apply(
	txn(
		putAtRev("a", 2, 0).during(0, 4),
		putAtRev("b", 2, 1).during(4, 8),
		countKeysAtRev(2, 2).during(8, 10),
	),
	txn(tombstoneAtRev("a", 3, 0).during(10, 20)),
)

func TestHistoryReads(t *testing.T) {
	// The compactor runs after the writer of txnsWithDelete.
	//
	//	time:       20           30           40
	//	compactor:  | Compact(2) | Compact(3) |
	base := apply(
		txnsWithDelete,
		compactAtRev(2, 20, 30, Revision{2, 0}, Revision{2, 1}),
		compactAtRev(3, 30, 40, Revision{2, 1}, Revision{3, 0}),
	)

	tests := []struct {
		name    string
		read    OpRecord
		wantErr string
	}{
		{"previous revision during a write", countKeysAtRev(1, 0).during(5, 6), ""},
		{"touches the final call and the next compaction", countKeysAtRev(2, 2).during(10, 30), ""},
		{
			"sees only part of the transaction", countKeysAtRev(2, 1).during(10, 30),
			wantMismatch(`CountRevisions(["", "\x00"), rev 2), time 10 to 30`,
				Result{Total: 2}, Result{Total: 1},
			),
		},
		{
			"starts before the final call returns", countKeysAtRev(2, 2).during(9, 30),
			`CountRevisions(["", "\x00"), rev 2) starts before the transaction of revision 2 returned`,
		},
		{
			"overlaps Compact(3)", countKeysAtRev(2, 2).during(10, 31),
			`CountRevisions(["", "\x00"), rev 2) ends after Compact(rev 3) started`,
		},
		{
			"starts during Compact(3)", countKeysAtRev(2, 2).during(31, 35),
			`CountRevisions(["", "\x00"), rev 2) ends after Compact(rev 3) started`,
		},
		{
			"Compact(2) already returned", countKeysAtRev(1, 0).during(31, 35),
			`CountRevisions(["", "\x00"), rev 1) ends after Compact(rev 2) started`,
		},
		{"overlaps Compact at its own revision", countKeysAtRev(3, 1).during(30, 31), ""},
		{"compacted revision after Compact returned", countKeysAtRev(3, 1).during(41, 42), ""},
		{
			"revision 4 doesn't exist", countKeysAtRev(4, 0).during(30, 30),
			`CountRevisions(["", "\x00"), rev 4) requests an unavailable revision`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkHistory(t, apply(base, reader(tt.read)), tt.wantErr)
		})
	}
}

func TestHistoryCompactions(t *testing.T) {
	// Revisions of txnsWithDelete.
	//
	//	revision  available at  keys          tombstones
	//	2         10            a=2.0, b=2.1
	//	3         20            b=2.1         3.0
	tests := []struct {
		name    string
		calls   historyStep
		wantErr string
	}{
		{
			name:    "negative revision",
			calls:   compactAtRev(-1, 0, 0),
			wantErr: "invalid compaction revision -1",
		},
		{
			name:    "repeated revision",
			calls:   apply(compactAtRev(1, 0, 0), compactAtRev(1, 0, 0)),
			wantErr: "Compact(rev 1) revision must exceed previous revision 1",
		},
		{
			name:    "calls overlap",
			calls:   apply(compactAtRev(0, 0, 3), compactAtRev(1, 0, 4)),
			wantErr: "Compact(rev 1): call at 0 precedes previous return at 3",
		},
		{
			name:  "calls touch",
			calls: apply(compactAtRev(0, 0, 2), compactAtRev(1, 2, 3)),
		},
		{
			name:    "revision 4 doesn't exist",
			calls:   compactAtRev(4, 0, 0),
			wantErr: "Compact(rev 4) requests an unavailable revision",
		},
		{
			name:    "starts before the last write returns",
			calls:   compactAtRev(3, 19, 20),
			wantErr: "Compact(rev 3) starts before the transaction of revision 3 returned",
		},
		{
			name:  "drops the tombstone",
			calls: compactAtRev(3, 20, 21, Revision{2, 1}),
			wantErr: wantMismatch("Compact(rev 3), time 20 to 21",
				map[Revision]struct{}{{Main: 2, Sub: 1}: {}, {Main: 3}: {}},
				map[Revision]struct{}{{Main: 2, Sub: 1}: {}},
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkHistory(t, apply(txnsWithDelete, tt.calls), tt.wantErr)
		})
	}
}

// checkHistory builds a fresh history. An empty wantErr means it should pass Check.
func checkHistory(t *testing.T, setup historyStep, wantErr string) {
	t.Helper()
	var h History
	setup(&h)
	err := h.Check()
	if wantErr == "" {
		require.NoError(t, err)
		return
	}
	require.EqualError(t, err, wantErr)
}

func wantMismatch(header string, want, got any) string {
	return header + " (-want +got):\n" + cmp.Diff(want, got, cmpopts.EquateEmpty())
}

// A historyStep appends read-only test records to a history. Their timestamps
// determine how calls from different goroutines overlap.
type historyStep func(*History)

func apply(steps ...historyStep) historyStep {
	return func(h *History) {
		for _, step := range steps {
			step(h)
		}
	}
}

func txn(calls ...OpRecord) historyStep {
	return func(h *History) {
		h.Transactions = append(h.Transactions, calls)
	}
}

func reader(calls ...OpRecord) historyStep {
	return func(h *History) {
		h.Readers = append(h.Readers, calls)
	}
}

func compactAtRev(rev, call, ret int64, kept ...Revision) historyStep {
	c := CompactRecord{Rev: rev, Call: call, Return: ret, Kept: make(map[Revision]struct{}, len(kept))}
	for _, r := range kept {
		c.Kept[r] = struct{}{}
	}
	return func(h *History) {
		h.Compactions = append(h.Compactions, c)
	}
}

func putAtRev(key string, rev, sub int64) OpRecord {
	return OpRecord{Op: Op{Kind: Put, Key: []byte(key), Rev: rev, Sub: sub}}
}

func tombstoneAtRev(key string, rev, sub int64) OpRecord {
	return OpRecord{Op: Op{Kind: Tombstone, Key: []byte(key), Rev: rev, Sub: sub}}
}

func countKeysAtRev(rev int64, total int) OpRecord {
	return OpRecord{Op: Op{Kind: CountRevisions, End: []byte{}, Rev: rev}, Got: Result{Total: total}}
}

func callAtRev(kind Kind, rev int64) OpRecord {
	return OpRecord{Op: Op{Kind: kind, Rev: rev}}
}

func (record OpRecord) during(call, ret int64) OpRecord {
	record.Call, record.Return = call, ret
	return record
}

func (record OpRecord) withError() OpRecord {
	record.Got.Err = true
	return record
}
