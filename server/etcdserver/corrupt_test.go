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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
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
				hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Revision: 10}}},
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
		{
			name: "Cluster ID Mismatch does not fail CorruptionChecker.InitialCheck()",
			hasher: fakeHasher{
				peerHashes: []*peerHashKVResp{{err: rpctypes.ErrClusterIdMismatch}},
			},
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
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Revision: 10}}, {hash: mvcc.KeyValueHash{Revision: 10}}}},
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
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Revision: 11}}, {err: fmt.Errorf("error getting hash")}}},
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
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, Revision: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2}, revision: 2}}},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(1)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name:          "Different local hash and compaction revision",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1}}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 2}}}},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name:          "Different local hash and same revisions",
			hasher:        fakeHasher{hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1, Revision: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 1, Revision: 1}, revision: 1}}},
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
				hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1, Revision: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 2, Revision: 2}, revision: 2}},
				peerHashes:         []*peerHashKVResp{{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{Revision: 1}, CompactRevision: 1, Hash: 1}}},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(1)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
		},
		{
			name: "Peer with different hash and same compact revision as first local",
			hasher: fakeHasher{
				hashByRevResponses: []hashByRev{{hash: mvcc.KeyValueHash{Hash: 1, CompactRevision: 1, Revision: 1}, revision: 1}, {hash: mvcc.KeyValueHash{Hash: 2, CompactRevision: 2}, revision: 2}},
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
		{
			name: "Cluster ID Mismatch does not fail CorruptionChecker.PeriodicCheck()",
			hasher: fakeHasher{
				peerHashes: []*peerHashKVResp{{err: rpctypes.ErrClusterIdMismatch}},
			},
			expectActions: []string{"HashByRev(0)", "PeerHashByRev(0)", "ReqTimeout()", "LinearizableReadNotify()", "HashByRev(0)"},
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
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "PeerHashByRev(1)", "MemberId()"},
		},
		{
			name: "Peer returned different compaction revision is skipped",
			hasher: fakeHasher{
				hashes:     []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1}, {Revision: 2, CompactRevision: 2}},
				peerHashes: []*peerHashKVResp{{resp: &pb.HashKVResponse{CompactRevision: 3}}},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "PeerHashByRev(1)", "MemberId()"},
		},
		{
			name: "Etcd can identify two corrupted members in 5 member cluster",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 1}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
					{peerInfo: peerInfo{id: 43}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
					{peerInfo: peerInfo{id: 44}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 7}},
					{peerInfo: peerInfo{id: 45}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 7}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "TriggerCorruptAlarm(44)", "TriggerCorruptAlarm(45)"},
			expectCorrupt: true,
		},
		{
			name: "Etcd checks next hash when one member is unresponsive in 3 member cluster",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 2}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{err: fmt.Errorf("failed getting hash")},
					{peerInfo: peerInfo{id: 43}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "PeerHashByRev(1)", "MemberId()"},
			expectCorrupt: false,
		},
		{
			name: "Etcd can identify single corrupted member in 3 member cluster",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 2}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
					{peerInfo: peerInfo{id: 43}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "TriggerCorruptAlarm(43)"},
			expectCorrupt: true,
		},
		{
			name: "Etcd can identify single corrupted member in 5 member cluster",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 2}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
					{peerInfo: peerInfo{id: 43}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
					{peerInfo: peerInfo{id: 44}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
					{peerInfo: peerInfo{id: 45}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "TriggerCorruptAlarm(44)"},
			expectCorrupt: true,
		},
		{
			name: "Etcd triggers corrupted alarm on whole cluster if in 3 member cluster one member is down and one member corrupted",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 2}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{err: fmt.Errorf("failed getting hash")},
					{peerInfo: peerInfo{id: 43}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "TriggerCorruptAlarm(0)"},
			expectCorrupt: true,
		},
		{
			name: "Etcd triggers corrupted alarm on whole cluster if no quorum in 5 member cluster",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 1}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
					{peerInfo: peerInfo{id: 43}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
					{peerInfo: peerInfo{id: 44}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
					{peerInfo: peerInfo{id: 45}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
					{peerInfo: peerInfo{id: 46}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 4}},
					{peerInfo: peerInfo{id: 47}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "TriggerCorruptAlarm(0)"},
			expectCorrupt: true,
		},
		{
			name: "Etcd can identify corrupted member in 5 member cluster even if one member is down",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 2}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
					{err: fmt.Errorf("failed getting hash")},
					{peerInfo: peerInfo{id: 44}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
					{peerInfo: peerInfo{id: 45}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 2}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "TriggerCorruptAlarm(44)"},
			expectCorrupt: true,
		},
		{
			name: "Etcd can identify that leader is corrupted",
			hasher: fakeHasher{
				hashes: []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 2}, {Revision: 2, CompactRevision: 1, Hash: 2}},
				peerHashes: []*peerHashKVResp{
					{peerInfo: peerInfo{id: 42}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
					{peerInfo: peerInfo{id: 43}, resp: &pb.HashKVResponse{CompactRevision: 1, Hash: 3}},
				},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()", "TriggerCorruptAlarm(1)"},
			expectCorrupt: true,
		},
		{
			name: "Peer returned same hash bumps last revision checked",
			hasher: fakeHasher{
				hashes:     []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 1}, {Revision: 2, CompactRevision: 1, Hash: 1}},
				peerHashes: []*peerHashKVResp{{resp: &pb.HashKVResponse{Header: &pb.ResponseHeader{MemberId: 42}, CompactRevision: 1, Hash: 1}}},
			},
			expectActions:             []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(2)", "MemberId()"},
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
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(1)", "MemberId()"},
		},
		{
			name: "Cluster ID Mismatch does not fail CorruptionChecker.CompactHashCheck()",
			hasher: fakeHasher{
				hashes:     []mvcc.KeyValueHash{{Revision: 1, CompactRevision: 1, Hash: 1}},
				peerHashes: []*peerHashKVResp{{err: rpctypes.ErrClusterIdMismatch}},
			},
			expectActions: []string{"MemberId()", "ReqTimeout()", "Hashes()", "PeerHashByRev(1)", "MemberId()"},
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

func TestHashKVHandler(t *testing.T) {
	var remoteClusterID = 111195
	var localClusterID = 111196
	var revision = 1

	etcdSrv := &EtcdServer{}
	etcdSrv.cluster = newTestCluster(t, nil)
	etcdSrv.cluster.SetID(types.ID(localClusterID), types.ID(localClusterID))
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)
	etcdSrv.kv = mvcc.New(zap.NewNop(), be, &lease.FakeLessor{}, mvcc.StoreConfig{})
	ph := &hashKVHandler{
		lg:     zap.NewNop(),
		server: etcdSrv,
	}
	srv := httptest.NewServer(ph)
	defer srv.Close()

	tests := []struct {
		name            string
		remoteClusterID int
		wcode           int
		wKeyWords       string
	}{
		{
			name:            "HashKV returns 200 if cluster hash matches",
			remoteClusterID: localClusterID,
			wcode:           http.StatusOK,
			wKeyWords:       "",
		},
		{
			name:            "HashKV returns 400 if cluster hash doesn't matche",
			remoteClusterID: remoteClusterID,
			wcode:           http.StatusPreconditionFailed,
			wKeyWords:       "cluster ID mismatch",
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashReq := &pb.HashKVRequest{Revision: int64(revision)}
			hashReqBytes, err := json.Marshal(hashReq)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}
			req, err := http.NewRequest(http.MethodGet, srv.URL+PeerHashKVPath, bytes.NewReader(hashReqBytes))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("X-Etcd-Cluster-ID", strconv.FormatUint(uint64(tt.remoteClusterID), 16))

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to get http response: %v", err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatalf("unexpected io.ReadAll error: %v", err)
			}
			if resp.StatusCode != tt.wcode {
				t.Fatalf("#%d: code = %d, want %d", i, resp.StatusCode, tt.wcode)
			}
			if resp.StatusCode != http.StatusOK {
				if !strings.Contains(string(body), tt.wKeyWords) {
					t.Errorf("#%d: body: %s, want body to contain keywords: %s", i, string(body), tt.wKeyWords)
				}
				return
			}

			hashKVResponse := pb.HashKVResponse{}
			err = json.Unmarshal(body, &hashKVResponse)
			if err != nil {
				t.Fatalf("unmarshal response error: %v", err)
			}
			hashValue, _, err := etcdSrv.KV().HashStorage().HashByRev(int64(revision))
			if err != nil {
				t.Fatalf("etcd server hash failed: %v", err)
			}
			if hashKVResponse.Hash != hashValue.Hash {
				t.Fatalf("hash value inconsistent: %d != %d", hashKVResponse.Hash, hashValue)
			}
		})
	}
}
