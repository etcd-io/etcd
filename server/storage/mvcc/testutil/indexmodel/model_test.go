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

	"github.com/stretchr/testify/require"
)

func TestSnapshotRead(t *testing.T) {
	// snapshot at revision 3
	//
	//	key   modified  created  version
	//	foo   3.0       2.0      2
	//	foo1  2.1       2.1      1
	foo := Item{Key: "foo", Modified: Revision{Main: 3}, Created: Revision{Main: 2}, Version: 2}
	foo1Rev := Revision{Main: 2, Sub: 1}
	foo1 := Item{Key: "foo1", Modified: foo1Rev, Created: foo1Rev, Version: 1}
	s := snapshot{currentRev: 3, keys: map[string]Item{"foo": foo, "foo1": foo1}}
	key := []byte("foo")

	tests := []struct {
		name string
		op   Op
		want Result
	}{
		{
			name: "get",
			op:   Op{Kind: Get, Key: key},
			want: Result{Items: []Item{foo}},
		},
		{
			name: "get missing key",
			op:   Op{Kind: Get, Key: []byte("missing")},
			want: Result{Err: true, Items: []Item{{Key: "missing"}}},
		},
		{
			name: "single-key range",
			op:   Op{Kind: Range, Key: key},
			want: Result{Items: []Item{foo}, Total: 1},
		},
		{
			name: "exclusive range end",
			op:   Op{Kind: Range, Key: key, End: []byte("foo1")},
			want: Result{Items: []Item{foo}, Total: 1},
		},
		{
			name: "open-ended range",
			op:   Op{Kind: Range, Key: key, End: []byte{}},
			want: Result{Items: []Item{foo, foo1}, Total: 2},
		},
		{
			name: "range with limit and total",
			op:   Op{Kind: Range, Key: key, End: []byte{}, Limit: 1, WithTotalCount: true},
			want: Result{Items: []Item{foo}, Total: 2},
		},
		{
			name: "revisions with limit",
			op:   Op{Kind: Revisions, Key: key, End: []byte{}, Limit: 1},
			want: Result{Items: []Item{{Modified: foo.Modified}}, Total: 1},
		},
		{
			name: "count revisions",
			op:   Op{Kind: CountRevisions, Key: key, End: []byte{}},
			want: Result{Total: 2},
		},
		{
			name: "keep",
			op:   Op{Kind: Keep},
			want: Result{Kept: map[Revision]struct{}{foo.Modified: {}, foo1Rev: {}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, s.read(tt.op))
		})
	}
}

func TestSnapshotWriteAndCompact(t *testing.T) {
	// snapshot at each revision
	//
	//	revision  foo  foo1                          tombstones
	//	2         2.0  2.1 (created 2.1, version 1)
	//	3         2.0  3.0 (created 2.1, version 2)
	//	4              3.0 (created 2.1, version 2)  4.0
	foo1Rev := Revision{Main: 2, Sub: 1}

	// revision 2 creates foo and foo1
	created := snapshot{currentRev: 2, keys: make(map[string]Item)}
	mustWrite(t, &created, putAtRev("foo", 2, 0).Op)
	mustWrite(t, &created, putAtRev("foo1", 2, 1).Op)

	// revision 3 updates foo1
	updated := created.cloneForNextRevision()
	mustWrite(t, &updated, putAtRev("foo1", 3, 0).Op)

	// revision 4 deletes foo
	deleted := updated.cloneForNextRevision()
	mustWrite(t, &deleted, tombstoneAtRev("foo", 4, 0).Op)

	// each snapshot should keep its own foo1, and the update should keep
	// the created revision with its sub-revision
	want := Item{Key: "foo1", Modified: foo1Rev, Created: foo1Rev, Version: 1}
	require.Equal(t, want, created.keys["foo1"])
	want = Item{Key: "foo1", Modified: Revision{Main: 3}, Created: foo1Rev, Version: 2}
	require.Equal(t, want, updated.keys["foo1"])
	require.NotContains(t, deleted.keys, "foo")

	// Keep should return live keys only, and Compact should also keep
	// the tombstone at revision 4
	live := map[Revision]struct{}{{Main: 3}: {}}
	withTombstone := map[Revision]struct{}{{Main: 3}: {}, {Main: 4}: {}}
	require.Equal(t, live, deleted.liveKeyModRevisions())
	require.Equal(t, withTombstone, deleted.revisionsKeptByCompact())

	// the next revision shouldn't keep that tombstone
	require.Equal(t, live, deleted.cloneForNextRevision().revisionsKeptByCompact())
}

func mustWrite(t *testing.T, s *snapshot, op Op) {
	t.Helper()
	result, err := s.write(op)
	require.NoError(t, err)
	require.Equal(t, Result{}, result)
}
