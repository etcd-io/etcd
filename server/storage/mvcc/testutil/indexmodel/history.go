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
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// OpRecord holds an index call and its result.
// Call and Return are nanoseconds since a shared monotonic start.
type OpRecord struct {
	Op           Op
	Got          Result
	Call, Return int64
}

// Transaction holds the calls of a single write transaction in execution order.
//
// Reads use an available committed revision. After the first successful write,
// they can also use the working revision, one past the last commit, and see
// the transaction's preceding writes.
//
// A transaction can put a key and then delete it. Putting a key that the same
// transaction already wrote isn't supported yet.
type Transaction []OpRecord

// CompactRecord holds a call to the index's Compact method and the revisions
// it kept. The store advances its compact revision before scheduling this call.
type CompactRecord struct {
	Rev          int64
	Kept         map[Revision]struct{}
	Call, Return int64
}

func (c CompactRecord) String() string {
	return fmt.Sprintf("Compact(rev %d)", c.Rev)
}

// History records one writer and one compactor in call order.
// Readers holds a separate sequence of calls for each reader.
type History struct {
	Transactions []Transaction
	Readers      [][]OpRecord
	Compactions  []CompactRecord
}

// Check validates the calls, replays the transactions and then checks the
// compactions and the reads. It returns the first error.
//
// Check uses the return time of a transaction's last index call as a lower
// bound on when other goroutines can observe its revision. A read at revision r
// may overlap Compact(r). Compact at a higher revision must not start before
// the read returns.
func (h History) Check() error {
	if err := h.validate(); err != nil {
		return err
	}

	r := newReplay(h.Compactions)
	for _, txn := range h.Transactions {
		if err := r.applyTransaction(txn); err != nil {
			return err
		}
	}
	for _, c := range h.Compactions {
		if err := r.checkCompaction(c); err != nil {
			return err
		}
	}
	for _, reads := range h.Readers {
		for _, read := range reads {
			if err := r.checkCommittedRead(read); err != nil {
				return err
			}
		}
	}
	return nil
}

// validate checks the calls of each goroutine before the replay.
func (h History) validate() error {
	// The compactor should use a higher revision each time.
	prevRev, prevReturn := int64(-1), int64(0)
	for _, c := range h.Compactions {
		if c.Rev < 0 {
			return fmt.Errorf("invalid compaction revision %d", c.Rev)
		}
		if c.Rev <= prevRev {
			return fmt.Errorf("%v revision must exceed previous revision %d", c, prevRev)
		}
		if err := checkInterval(c.Call, c.Return, prevReturn); err != nil {
			return fmt.Errorf("%v: %w", c, err)
		}
		prevRev, prevReturn = c.Rev, c.Return
	}

	// The writer runs the transactions one by one.
	writer := slices.Concat(h.Transactions...)
	err := validateCalls(writer, Put, Tombstone, Get, Range, Revisions, CountRevisions)
	if err != nil {
		return fmt.Errorf("writer: %w", err)
	}
	for i, reads := range h.Readers {
		err = validateCalls(reads, Get, Range, Revisions, CountRevisions, Keep)
		if err != nil {
			return fmt.Errorf("reader %d: %w", i, err)
		}
	}
	return nil
}

// validateCalls checks the calls of the writer or one reader.
func validateCalls(records []OpRecord, allowed ...Kind) error {
	var prevReturn int64
	for _, record := range records {
		op := record.Op
		if !slices.Contains(allowed, op.Kind) {
			return fmt.Errorf("unexpected operation kind %q", op.Kind)
		}
		if op.Rev < 1 {
			return fmt.Errorf("invalid main revision %d for %v", op.Rev, op)
		}
		if err := checkInterval(record.Call, record.Return, prevReturn); err != nil {
			return fmt.Errorf("%v: %w", op, err)
		}
		prevReturn = record.Return
	}
	return nil
}

// checkInterval verifies that a goroutine makes one call at a time.
func checkInterval(call, ret, prevReturn int64) error {
	if call < 0 || ret < call {
		return fmt.Errorf("invalid interval [%d, %d]", call, ret)
	}
	if call < prevReturn {
		return fmt.Errorf("call at %d precedes previous return at %d", call, prevReturn)
	}
	return nil
}

// revisionView is the snapshot of one main revision. Readers and the
// compactor can use it once the final call of its transaction returned.
type revisionView struct {
	snapshot
	availableAt int64
}

// replay holds the snapshot of each main revision.
type replay struct {
	revisions   []revisionView // Indexed by main revision.
	compactions []CompactRecord
}

func newReplay(compactions []CompactRecord) *replay {
	// Revision 1 is the initial empty store. Revision 0 is here only because
	// Compact(0) is valid.
	empty := snapshot{currentRev: 1, keys: make(map[string]Item)}
	return &replay{revisions: []revisionView{{}, {snapshot: empty}}, compactions: compactions}
}

// applyTransaction replays txn on a copy of the latest snapshot. The copy
// becomes the next main revision only if txn wrote something.
func (r *replay) applyTransaction(txn Transaction) error {
	working := r.revisions[len(r.revisions)-1].cloneForNextRevision()

	for _, step := range txn {
		op := step.Op
		switch {
		case op.Kind == Put || op.Kind == Tombstone:
			want, err := working.write(op)
			if err != nil {
				return err
			}
			if err := step.checkResult(want); err != nil {
				return err
			}
		case op.Rev == working.currentRev && len(working.writtenKeys) > 0:
			// The working revision exists only after the first successful write.
			if err := step.checkResult(working.read(op)); err != nil {
				return err
			}
		default:
			if err := r.checkCommittedRead(step); err != nil {
				return err
			}
		}
	}
	if len(working.writtenKeys) > 0 {
		view := revisionView{snapshot: working, availableAt: txn[len(txn)-1].Return}
		r.revisions = append(r.revisions, view)
	}
	return nil
}

// viewAtRev returns the snapshot of rev if it's available at time call.
func (r *replay) viewAtRev(op fmt.Stringer, rev, call int64) (*revisionView, error) {
	if rev >= int64(len(r.revisions)) {
		return nil, fmt.Errorf("%v requests an unavailable revision", op)
	}

	view := &r.revisions[rev]
	if call < view.availableAt {
		return nil, fmt.Errorf("%v starts before the transaction of revision %d returned", op, rev)
	}
	return view, nil
}

func (r *replay) checkCompaction(c CompactRecord) error {
	view, err := r.viewAtRev(c, c.Rev, c.Call)
	if err != nil {
		return err
	}

	if want := view.revisionsKeptByCompact(); !maps.Equal(c.Kept, want) {
		return mismatchError(c, c.Call, c.Return, want, c.Kept)
	}
	return nil
}

// checkCommittedRead checks a read of a committed revision. It should return before
// the first Compact above its revision starts. Compact at its revision can
// run together with it, since that keeps what the read needs.
func (r *replay) checkCommittedRead(read OpRecord) error {
	rev := read.Op.Rev
	view, err := r.viewAtRev(read.Op, rev, read.Call)
	if err != nil {
		return err
	}

	// Compactions are in revision order. So it finds the earliest one above rev.
	i := sort.Search(len(r.compactions), func(i int) bool {
		return r.compactions[i].Rev > rev
	})
	if i < len(r.compactions) && read.Return > r.compactions[i].Call {
		return fmt.Errorf("%v ends after %v started", read.Op, r.compactions[i])
	}
	return read.checkResult(view.read(read.Op))
}

func (record OpRecord) checkResult(want Result) error {
	got := record.Got
	if got.Err == want.Err && got.Total == want.Total &&
		slices.Equal(got.Items, want.Items) && maps.Equal(got.Kept, want.Kept) {
		return nil
	}
	return mismatchError(record.Op, record.Call, record.Return, want, got)
}

func mismatchError(op fmt.Stringer, call, ret int64, want, got any) error {
	diff := cmp.Diff(want, got, cmpopts.EquateEmpty())
	return fmt.Errorf("%v, time %d to %d (-want +got):\n%s", op, call, ret, diff)
}
