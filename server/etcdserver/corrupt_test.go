// Copyright 2022 The etcd Authors
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

package etcdserver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.uber.org/zap/zaptest"
)

func TestInitialCheck(t *testing.T) {
	tcs := []struct {
		name          string
		hasher        fakeHasher
		expectError   bool
		expectCorrupt bool
		expectActions []string
	}{
		{
			name: "No peers",
			hasher: fakeHasher{
				hashByRevResponses: []hashByRev{{revision: 10}},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(10)", "MemberId()"},
		},
		{
			name:          "Error getting hash",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{err: fmt.Errorf("error getting hash")}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "MemberId()"},
			expectError:   true,
		},
		{
			name:          "Peer with empty response",
			hasher:        fakeHasher{peerHashes: []*peerHashKVResp{{}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(0)", "MemberId()"},
		},
		{
			name:          "Peer returned ErrFutureRev",
			hasher:        fakeHasher{peerHashes: []*peerHashKVResp{{err: rpctypes.ErrFutureRev}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(0)", "MemberId()", "MemberId()"},
		},
		{
			name:          "Peer returned ErrCompacted",
			hasher:        fakeHasher{peerHashes: []*peerHashKVResp{{err: rpctypes.ErrCompacted}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(0)", "MemberId()", "MemberId()"},
		},
		{
			name:          "Peer returned other error",
			hasher:        fakeHasher{peerHashes: []*peerHashKVResp{{err: rpctypes.ErrCorrupt}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(0)", "MemberId()"},
		},
		{
			name:          "Peer returned same hash",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1}}}, peerHashes: []*peerHashKVResp{{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{}, Hash: 1}}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(0)", "MemberId()", "MemberId()"},
		},
		{
			name:          "Peer returned different hash with same compaction rev",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1}}}, peerHashes: []*peerHashKVResp{{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{}, Hash: 2, CompactRevision: 1}}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(0)", "MemberId()", "MemberId()"},
			expectError:   true,
		},
		{
			name:          "Peer returned different hash and compaction rev",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1}}}, peerHashes: []*peerHashKVResp{{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{}, Hash: 2, CompactRevision: 2}}}},
			expectActions: []string{"MemberId()", "ReqTimeout()", "HashByRev(0)", "PeerHashByRev(0)", "MemberId()", "MemberId()"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			monitor := corruptionChecker{
				lg:     zaptest.NewLogger(t),
				hasher: &tc.hasher,
			}
			err := monitor.InitialCheck()
			if gotError := err != nil; gotError != tc.expectError {
				t.Errorf("Unexpected error, got: %v, expected?: %v", err, tc.expectError)
			}
			if tc.hasher.alarmTriggered != tc.expectCorrupt {
				t.Errorf("Unexpected corrupt triggered, got: %v, expected?: %v", tc.hasher.alarmTriggered, tc.expectCorrupt)
			}
			assert.Equal(t, tc.expectActions, tc.hasher.actions)
		})
	}
}

func TestPeriodicCheck(t *testing.T) {
	tcs := []struct {
		name          string
		hasher        fakeHasher
		expectError   bool
		expectCorrupt bool
		expectActions []string
	}{
		{
			name:          "Same local hash and no peers",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{revision: 10}, {revision: 10}}},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(10)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name:          "Error getting hash first time",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{err: fmt.Errorf("error getting hash")}}},
			expectActions: []string{"HashByRev(0)"},
			expectError:   true,
		},
		{
			name:          "Error getting hash second time",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{revision: 11}, {err: fmt.Errorf("error getting hash")}}},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(11)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
			expectError:   true,
		},
		{
			name:          "Error linearizableReadNotify",
			hasher:        fakeHasher{linearizableReadNotify: fmt.Errorf("error getting linearizableReadNotify")},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()"},
			expectError:   true,
		},
		{
			name:          "Different local hash and revision",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2}, revision: 2}}},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(1)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name:          "Different local hash and compaction revision",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1}}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 2}}}},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name:          "Different local hash and same revisions",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 1}, revision: 1}}},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(1)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)", "MemberId()", "TriggerCorruptAlarm(1)"},
			expectCorrupt: true,
		},
		{
			name: "Peer with nil response",
			hasher: fakeHasher{
				peerHashes: []*peerHashKVResp{{}},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name: "Peer with newer revision",
			hasher: fakeHasher{
				peerHashes: []*peerHashKVResp{{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{Revision: 1}}}},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)", "TriggerCorruptAlarm(42)"},
			expectCorrupt: true,
		},
		{
			name: "Peer with newer compact revision",
			hasher: fakeHasher{
				peerHashes: []*peerHashKVResp{{peerInfo: peerInfo{id: 88}, resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{Revision: 10}, CompactRevision: 2}}},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)", "TriggerCorruptAlarm(88)"},
			expectCorrupt: true,
		},
		{
			name: "Peer with same hash and compact revision",
			hasher: fakeHasher{
				hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 2}, revision: 2}},
				peerHashes:         []*peerHashKVResp{{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{Revision: 1}, CompactRevision: 1, Hash: 1}}},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(1)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name: "Peer with different hash and same compact revision as first local",
			hasher: fakeHasher{
				hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 2}, revision: 2}},
				peerHashes:         []*peerHashKVResp{{peerInfo: peerInfo{id: 666}, resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{Revision: 1}, CompactRevision: 1, Hash: 2}}},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(1)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)", "TriggerCorruptAlarm(666)"},
			expectCorrupt: true,
		},
		{
			name: "Multiple corrupted peers trigger one alarm",
			hasher: fakeHasher{
				peerHashes: []*peerHashKVResp{
					{peerInfo: peerInfo{id: 88}, resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{Revision: 10}, CompactRevision: 2}},
					{peerInfo: peerInfo{id: 89}, resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{Revision: 10}, CompactRevision: 2}},
				},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)", "TriggerCorruptAlarm(88)"},
			expectCorrupt: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			monitor := corruptionChecker{
				lg:     zaptest.NewLogger(t),
				hasher: &tc.hasher,
			}
			err := monitor.PeriodicCheck()
			if gotError := err != nil; gotError != tc.expectError {
				t.Errorf("Unexpected error, got: %v, expected?: %v", err, tc.expectError)
			}
			if tc.hasher.alarmTriggered != tc.expectCorrupt {
				t.Errorf("Unexpected corrupt triggered, got: %v, expected?: %v", tc.hasher.alarmTriggered, tc.expectCorrupt)
			}
			assert.Equal(t, tc.expectActions, tc.hasher.actions)
		})
	}
}

func TestCompactHashCheck(t *testing.T) {
	tcs := []struct {
		name                string
		hasher              fakeHasher
		lastRevisionChecked int64

		expectError               bool
		expectCorrupt             bool
		expectActions             []string
		expectLastRevisionChecked int64
	}{
		{
			name:          "No hashes",
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()"},
		},
		{
			name: "No peers, check new checked from largest to smallest",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1}, {Revision: 2}, {Revision: 3}, {Revision: 4}},
			},
			lastRevisionChecked:       2,
			expectActions:             []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(4)", "PeerHashByRev(3)"},
			expectLastRevisionChecked: 2,
		},
		{
			name: "Peer error",
			hasher: fakeHasher{
				hashes:     []mvcc.KeyValueHash{{Revision: 1}, {Revision: 2}},
				peerHashes: []*peerHashKVResp{{err: fmt.Errorf("failed getting hash")}},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "PeerHashByRev(1)"},
		},
		{
			name: "Peer returned different compaction revision is skipped",
			hasher: fakeHasher{
				hashes:     []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1}, {Revision: 2, CompactRevision: 2}},
				peerHashes: []*peerHashKVResp{{resp: &pb.HashKVResponse{CompactRevision: 3}}},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "PeerHashByRev(1)"},
		},
		{
			name: "Peer returned same compaction revision but different hash triggers alarm",
			hasher: fakeHasher{
				hashes:     []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 1}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}}},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "TriggerCorruptAlarm(42)"},
			expectCorrupt: true,
		},
		{
			name: "Peer returned same hash bumps last revision checked",
			hasher: fakeHasher{
				hashes:     []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 1}, {Revision: 2, CompactRevision: 1, Hash: 1}},
				peerHashes: []*peerHashKVResp{{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{MemberId: 42}, CompactRevision: 1, Hash: 1}}},
			},
			expectActions:             []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)"},
			expectLastRevisionChecked: 2,
		},
		{
			name: "Only one peer succeeded check",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 1}},
				peerHashes: []*peerHashKVResp{
					{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{MemberId: 42}, CompactRevision: 1, Hash: 1}},
					{err: fmt.Errorf("failed getting hash")},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(1)"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			monitor := corruptionChecker{
				latestRevisionChecked: tc.lastRevisionChecked,
				lg:                    zaptest.NewLogger(t),
				hasher:                &tc.hasher,
			}
			monitor.CompactHashCheck()
			if tc.hasher.alarmTriggered != tc.expectCorrupt {
				t.Errorf("Unexpected corrupt triggered, got: %v, expected?: %v", tc.hasher.alarmTriggered, tc.expectCorrupt)
			}
			if tc.expectLastRevisionChecked != monitor.latestRevisionChecked {
				t.Errorf("Unexpected last revision checked, got: %v, expected?: %v", monitor.latestRevisionChecked, tc.expectLastRevisionChecked)
			}
			assert.Equal(t, tc.expectActions, tc.hasher.actions)
		})
	}
}

type fakeHasher struct {
	peerHashes             []*peerHashKVResp
	hashByRevIndex         int
	hashByRevResponses     []hashByRev
	linearizableReadNotify error
	hashes                 []mvcc.KeyValueHash

	alarmTriggered bool
	actions        []string
}

type hashByRev struct {
	hash     mvcc.KeyValueHash
	revision int64
	err      error
}

func (f *fakeHasher) Hash() (hash uint32, revision int64, err error) {
	panic("not implemented")
}

func (f *fakeHasher) HashByRev(rev int64) (hash mvcc.KeyValueHash, revision int64, err error) {
	f.actions = append(f.actions, fmt.Sprintf("HashByRev(%d)", rev))
	if len(f.hashByRevResponses) == 0 {
		return mvcc.KeyValueHash{}, 0, nil
	}
	hashByRev := f.hashByRevResponses[f.hashByRevIndex]
	f.hashByRevIndex++
	return hashByRev.hash, hashByRev.revision, hashByRev.err
}

func (f *fakeHasher) Store(hash mvcc.KeyValueHash) {
	f.actions = append(f.actions, fmt.Sprintf("Store(%v)", hash))
	f.hashes = append(f.hashes, hash)
}

func (f *fakeHasher) Hashes() []mvcc.KeyValueHash {
	f.actions = append(f.actions, "Hashes()")
	return f.hashes
}

func (f *fakeHasher) ReqTimeout() time.Duration {
	f.actions = append(f.actions, "ReqTimeout()")
	return time.Second
}

func (f *fakeHasher) MemberId() types.ID {
	f.actions = append(f.actions, "MemberId()")
	return 1
}

func (f *fakeHasher) PeerHashByRev(rev int64) []*peerHashKVResp {
	f.actions = append(f.actions, fmt.Sprintf("PeerHashByRev(%d)", rev))
	return f.peerHashes
}

func (f *fakeHasher) LinearizableReadNotify(ctx context.Context) error {
	f.actions = append(f.actions, "LinearizableReadNotify()")
	return f.linearizableReadNotify
}

func (f *fakeHasher) TriggerCorruptAlarm(memberId types.ID) {
	f.actions = append(f.actions, fmt.Sprintf("TriggerCorruptAlarm(%d)", memberId))
	f.alarmTriggered = true
}
