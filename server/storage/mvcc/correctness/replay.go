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

package correctness

import (
	"errors"
	"fmt"
	"sort"

	"github.com/anishathalye/porcupine"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

// StorageReplay reconstructs the deterministic per-revision MVCC state history
// and watch event sequence from committed write operations using StepDeterministic.
type StorageReplay struct {
	revisionToState []*StorageState
	events          []*mvccpb.Event
	eventsByRev     map[int64][]*mvccpb.Event
}

type committedWrite struct {
	rev int64
	req StorageRequest
}

// NewStorageReplay builds a StorageReplay from recorded Porcupine operations,
// verifying that every mutating MVCC write committed at a unique, consecutive revision.
func NewStorageReplay(operations []porcupine.Operation) (*StorageReplay, error) {
	writes := collectCommittedWrites(operations)

	state := newStorageState()
	// Indices 0 and 1 hold the initial empty MVCC state (currentRev=1).
	revisionToState := []*StorageState{
		state.clone(),
		state.clone(),
	}
	var events []*mvccpb.Event
	eventsByRev := make(map[int64][]*mvccpb.Event, len(writes))

	for _, w := range writes {
		expectedNextRev := state.currentRev + 1
		if w.rev != expectedNextRev {
			return nil, fmt.Errorf("non-consecutive or duplicate write revision %d (expected %d) for %s",
				w.rev, expectedNextRev, describeRequest(w.req))
		}

		nextState, expectedRes, evs := StepDeterministic(state, w.req)
		if expectedRes.Rev != w.rev {
			return nil, fmt.Errorf("replay revision mismatch for %s: model produced rev %d, observed %d",
				describeRequest(w.req), expectedRes.Rev, w.rev)
		}

		state = nextState
		revisionToState = append(revisionToState, state.clone())
		events = append(events, evs...)
		eventsByRev[w.rev] = evs
	}

	return &StorageReplay{
		revisionToState: revisionToState,
		events:          events,
		eventsByRev:     eventsByRev,
	}, nil
}

// StateAtRevision returns the full StorageState snapshot as of the requested revision.
func (r *StorageReplay) StateAtRevision(rev int64) (*StorageState, error) {
	if rev <= 0 {
		return nil, errors.New("invalid revision")
	}
	if int(rev) >= len(r.revisionToState) {
		return nil, mvcc.ErrFutureRev
	}
	return r.revisionToState[rev], nil
}

// StateForRevision returns the MVCC key-value map as of the requested revision.
func (r *StorageReplay) StateForRevision(rev int64) (map[string]*mvccpb.KeyValue, error) {
	st, err := r.StateAtRevision(rev)
	if err != nil {
		return nil, err
	}
	return st.items, nil
}

// MaxRevision returns the highest committed MVCC revision recorded in the replay.
func (r *StorageReplay) MaxRevision() int64 {
	return int64(len(r.revisionToState) - 1)
}

// Events returns the ordered sequence of watch events produced by all committed writes.
func (r *StorageReplay) Events() []*mvccpb.Event {
	return r.events
}

// EventsMatching returns all expected watch events at revisions >= startRev matching [key, end) and filter.
func (r *StorageReplay) EventsMatching(startRev int64, key, end []byte, filter WatchFilterType) []*mvccpb.Event {
	var matched []*mvccpb.Event
	for _, ev := range r.events {
		if ev.Kv.ModRevision < startRev {
			continue
		}
		if !matchKeyRange(string(ev.Kv.Key), key, end) {
			continue
		}
		if !filter.AllowsEvent(ev.Type) {
			continue
		}
		matched = append(matched, ev)
	}
	return matched
}

func collectCommittedWrites(operations []porcupine.Operation) []committedWrite {
	var writes []committedWrite
	for _, op := range operations {
		req := op.Input.(StorageRequest)
		res := op.Output.(StorageResponse)
		if !isMutatingMVCCWrite(req, res) {
			continue
		}
		writes = append(writes, committedWrite{rev: res.Rev, req: req})
	}
	sort.Slice(writes, func(i, j int) bool {
		return writes[i].rev < writes[j].rev
	})
	return writes
}

func isMutatingMVCCWrite(req StorageRequest, res StorageResponse) bool {
	if res.Err != nil || res.Rev <= 0 {
		return false
	}
	switch req.Op {
	case OpPut, OpCrossLayerTxn:
		return true
	case OpDelete, OpDeleteRange, OpLeaseRevoke:
		return res.Deleted > 0
	case OpTxn:
		if res.Deleted > 0 {
			return true
		}
		for _, sub := range req.TxnOps {
			if sub.Type == TxnSubOpPut {
				return true
			}
		}
		return false
	default:
		return false
	}
}
