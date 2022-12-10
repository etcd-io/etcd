// Copyright 2015 The etcd Authors
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

package raft

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	pb "go.etcd.io/etcd/raft/v3/raftpb"
)

var testFormatter EntryFormatter = func(data []byte) string {
	return strings.ToUpper(string(data))
}

func TestDescribeEntry(t *testing.T) {
	entry := pb.Entry{
		Term:  1,
		Index: 2,
		Type:  pb.EntryNormal,
		Data:  []byte("hello\x00world"),
	}
	require.Equal(t, `1/2 EntryNormal "hello\x00world"`, DescribeEntry(entry, nil))
	require.Equal(t, "1/2 EntryNormal HELLO\x00WORLD", DescribeEntry(entry, testFormatter))
}

func TestLimitSize(t *testing.T) {
	ents := []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}}
	tests := []struct {
		maxsize  uint64
		wentries []pb.Entry
	}{
		{math.MaxUint64, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}}},
		// Even if maxsize is zero, the first entry should be returned.
		{0, []pb.Entry{{Index: 4, Term: 4}}},
		// Limit to 2.
		{uint64(ents[0].Size() + ents[1].Size()), []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
		// Limit to 2.
		{uint64(ents[0].Size() + ents[1].Size() + ents[2].Size()/2), []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
		{uint64(ents[0].Size() + ents[1].Size() + ents[2].Size() - 1), []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
		// All.
		{uint64(ents[0].Size() + ents[1].Size() + ents[2].Size()), []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}}},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			require.Equal(t, tt.wentries, limitSize(ents, entryEncodingSize(tt.maxsize)))
		})
	}
}

func TestIsLocalMsg(t *testing.T) {
	tests := []struct {
		msgt    pb.MessageType
		isLocal bool
	}{
		{pb.MsgHup, true},
		{pb.MsgBeat, true},
		{pb.MsgUnreachable, true},
		{pb.MsgSnapStatus, true},
		{pb.MsgCheckQuorum, true},
		{pb.MsgTransferLeader, false},
		{pb.MsgProp, false},
		{pb.MsgApp, false},
		{pb.MsgAppResp, false},
		{pb.MsgVote, false},
		{pb.MsgVoteResp, false},
		{pb.MsgSnap, false},
		{pb.MsgHeartbeat, false},
		{pb.MsgHeartbeatResp, false},
		{pb.MsgTimeoutNow, false},
		{pb.MsgReadIndex, false},
		{pb.MsgReadIndexResp, false},
		{pb.MsgPreVote, false},
		{pb.MsgPreVoteResp, false},
		{pb.MsgStorageAppend, true},
		{pb.MsgStorageAppendResp, true},
		{pb.MsgStorageApply, true},
		{pb.MsgStorageApplyResp, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.msgt), func(t *testing.T) {
			require.Equal(t, tt.isLocal, IsLocalMsg(tt.msgt))
		})
	}
}

func TestIsResponseMsg(t *testing.T) {
	tests := []struct {
		msgt       pb.MessageType
		isResponse bool
	}{
		{pb.MsgHup, false},
		{pb.MsgBeat, false},
		{pb.MsgUnreachable, true},
		{pb.MsgSnapStatus, false},
		{pb.MsgCheckQuorum, false},
		{pb.MsgTransferLeader, false},
		{pb.MsgProp, false},
		{pb.MsgApp, false},
		{pb.MsgAppResp, true},
		{pb.MsgVote, false},
		{pb.MsgVoteResp, true},
		{pb.MsgSnap, false},
		{pb.MsgHeartbeat, false},
		{pb.MsgHeartbeatResp, true},
		{pb.MsgTimeoutNow, false},
		{pb.MsgReadIndex, false},
		{pb.MsgReadIndexResp, true},
		{pb.MsgPreVote, false},
		{pb.MsgPreVoteResp, true},
		{pb.MsgStorageAppend, false},
		{pb.MsgStorageAppendResp, true},
		{pb.MsgStorageApply, false},
		{pb.MsgStorageApplyResp, true},
	}

	for i, tt := range tests {
		got := IsResponseMsg(tt.msgt)
		if got != tt.isResponse {
			t.Errorf("#%d: got %v, want %v", i, got, tt.isResponse)
		}
	}
}
