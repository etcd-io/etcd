// Copyright 2024 The etcd Authors
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

package mvcc

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/raft/raftpb"
)

func TestUnsafeDetectSchemaVersion(t *testing.T) {
	tests := []struct {
		storageVersion  string
		term            uint64
		confState       *raftpb.ConfState
		expectedVersion string
	}{
		{
			storageVersion:  "3.6.1",
			expectedVersion: "3.6.1",
		},
		{
			expectedVersion: "3.4.0",
		},
		{
			term:            1,
			expectedVersion: "3.5.0",
		},
		{
			term:            1,
			confState:       &raftpb.ConfState{Voters: []uint64{1}},
			expectedVersion: "3.5.0",
		},
		{
			confState:       &raftpb.ConfState{Voters: []uint64{1}},
			expectedVersion: "3.5.0",
		},
		{
			term:            0,
			confState:       &raftpb.ConfState{Voters: []uint64{}},
			expectedVersion: "3.5.0",
		},
		{
			term:            1,
			confState:       &raftpb.ConfState{Voters: []uint64{}},
			expectedVersion: "3.5.0",
		},
	}
	for _, tt := range tests {
		s := newFakeStore()
		b := s.b.(*fakeBackend)
		if len(tt.storageVersion) > 0 {
			b.tx.rangeRespc <- rangeResp{[][]byte{storageVersionKeyName}, [][]byte{[]byte(tt.storageVersion)}}
		} else {
			b.tx.rangeRespc <- rangeResp{nil, nil}
		}
		if tt.confState != nil {
			confStateBytes, err := json.Marshal(tt.confState)
			if err != nil {
				t.Fatalf("Cannot marshal raftpb.ConfState, conf-state = %s, err = %s\n", tt.confState.String(), err)
			}
			b.tx.rangeRespc <- rangeResp{[][]byte{confStateKeyName}, [][]byte{confStateBytes}}
		} else {
			b.tx.rangeRespc <- rangeResp{nil, nil}
		}
		if tt.term > 0 {
			bs2 := make([]byte, 8)
			binary.BigEndian.PutUint64(bs2, tt.term)
			b.tx.rangeRespc <- rangeResp{[][]byte{termKeyName}, [][]byte{bs2}}
		} else {
			b.tx.rangeRespc <- rangeResp{nil, nil}
		}

		v := UnsafeDetectSchemaVersion(s.lg, b.BatchTx())
		expectedVersion := semver.Must(semver.NewVersion(tt.expectedVersion))
		if !v.Equal(*expectedVersion) {
			t.Fatalf("want version: %s, got: %s", expectedVersion, v)
		}
	}
}
