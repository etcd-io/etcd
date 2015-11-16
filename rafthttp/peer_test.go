// Copyright 2015 CoreOS, Inc.
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

package rafthttp

import (
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
)

func TestPeerPick(t *testing.T) {
	tests := []struct {
		msgappWorking  bool
		messageWorking bool
		m              raftpb.Message
		wpicked        string
	}{
		{
			true, true,
			raftpb.Message{Type: raftpb.MsgSnap},
			pipelineMsg,
		},
		{
			true, true,
			raftpb.Message{Type: raftpb.MsgApp, Term: 1, LogTerm: 1},
			streamAppV2,
		},
		{
			true, true,
			raftpb.Message{Type: raftpb.MsgProp},
			streamMsg,
		},
		{
			true, true,
			raftpb.Message{Type: raftpb.MsgHeartbeat},
			streamMsg,
		},
		{
			false, true,
			raftpb.Message{Type: raftpb.MsgApp, Term: 1, LogTerm: 1},
			streamMsg,
		},
		{
			false, false,
			raftpb.Message{Type: raftpb.MsgApp, Term: 1, LogTerm: 1},
			pipelineMsg,
		},
		{
			false, false,
			raftpb.Message{Type: raftpb.MsgProp},
			pipelineMsg,
		},
		{
			false, false,
			raftpb.Message{Type: raftpb.MsgSnap},
			pipelineMsg,
		},
		{
			false, false,
			raftpb.Message{Type: raftpb.MsgHeartbeat},
			pipelineMsg,
		},
	}
	for i, tt := range tests {
		peer := &peer{
			msgAppV2Writer: &streamWriter{working: tt.msgappWorking},
			writer:         &streamWriter{working: tt.messageWorking},
			pipeline:       &pipeline{},
		}
		_, picked := peer.pick(tt.m)
		if picked != tt.wpicked {
			t.Errorf("#%d: picked = %v, want %v", i, picked, tt.wpicked)
		}
	}
}

func TestDescribeMsgType(t *testing.T) {
	ts := map[raftpb.MessageType]string{
		raftpb.MsgHup:           "MsgHup(candidate requests to start a new election)",
		raftpb.MsgBeat:          "MsgBeat(leader requests to send a heartbeat)",
		raftpb.MsgProp:          "MsgProp(proposal to change configurations)",
		raftpb.MsgApp:           "MsgApp(leader requests to replicate logs)",
		raftpb.MsgAppResp:       "MsgAppResp(leader handles response to its log replication message)",
		raftpb.MsgVote:          "MsgVote(follower votes for a new leader)",
		raftpb.MsgVoteResp:      "MsgVoteResp(candidate receives votes from followers)",
		raftpb.MsgSnap:          "MsgSnap(leader requests to install a snapshot message)",
		raftpb.MsgHeartbeat:     "MsgHeartbeat(candidate/follower receives a heartbeat from a leader)",
		raftpb.MsgHeartbeatResp: "MsgHeartbeatResp(leader handles response to its heartbeat message)",
		raftpb.MsgUnreachable:   "MsgUnreachable(leader finds out that its request wasn't delivered)",
		raftpb.MsgSnapStatus:    "MsgSnapStatus(leader handles response to its snapshot install request)",
	}
	for k, v := range ts {
		if rs := describeMsgType(k); rs != v {
			t.Errorf("expected %s for %s but got %s", v, k, rs)
		}
	}
}
