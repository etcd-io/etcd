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

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"go.etcd.io/raft/v3/raftpb"
)

func TestProcessMessages(t *testing.T) {
	cases := []struct {
		name             string
		confState        *raftpb.ConfState
		InputMessages    []*raftpb.Message
		ExpectedMessages []*raftpb.Message
	}{
		{
			name: "only one snapshot message",
			confState: &raftpb.ConfState{
				Voters: []uint64{2, 6, 8, 10},
			},
			InputMessages: []*raftpb.Message{
				{
					Type: raftpb.MsgSnap.Enum(),
					To:   new(uint64(8)),
					Snapshot: &raftpb.Snapshot{
						Metadata: &raftpb.SnapshotMetadata{
							Index: new(uint64(100)),
							Term:  new(uint64(3)),
							ConfState: &raftpb.ConfState{
								Voters:    []uint64{2, 6, 8},
								AutoLeave: new(true),
							},
						},
					},
				},
			},
			ExpectedMessages: []*raftpb.Message{
				{
					Type: raftpb.MsgSnap.Enum(),
					To:   new(uint64(8)),
					Snapshot: &raftpb.Snapshot{
						Metadata: &raftpb.SnapshotMetadata{
							Index: new(uint64(100)),
							Term:  new(uint64(3)),
							ConfState: &raftpb.ConfState{
								Voters: []uint64{2, 6, 8, 10},
							},
						},
					},
				},
			},
		},
		{
			name: "one snapshot message and one other message",
			confState: &raftpb.ConfState{
				Voters: []uint64{2, 7, 8, 12},
			},
			InputMessages: []*raftpb.Message{
				{
					Type: raftpb.MsgSnap.Enum(),
					To:   new(uint64(8)),
					Snapshot: &raftpb.Snapshot{
						Metadata: &raftpb.SnapshotMetadata{
							Index: new(uint64(100)),
							Term:  new(uint64(3)),
							ConfState: &raftpb.ConfState{
								Voters:    []uint64{2, 6, 8},
								AutoLeave: new(true),
							},
						},
					},
				},
				{
					Type: raftpb.MsgApp.Enum(),
					From: new(uint64(6)),
					To:   new(uint64(8)),
				},
			},
			ExpectedMessages: []*raftpb.Message{
				{
					Type: raftpb.MsgSnap.Enum(),
					To:   new(uint64(8)),
					Snapshot: &raftpb.Snapshot{
						Metadata: &raftpb.SnapshotMetadata{
							Index: new(uint64(100)),
							Term:  new(uint64(3)),
							ConfState: &raftpb.ConfState{
								Voters: []uint64{2, 7, 8, 12},
							},
						},
					},
				},
				{
					Type: raftpb.MsgApp.Enum(),
					From: new(uint64(6)),
					To:   new(uint64(8)),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rn := &raftNode{
				confState: tc.confState,
			}

			outputMessages := rn.processMessages(tc.InputMessages)
			if diff := cmp.Diff(tc.ExpectedMessages, outputMessages, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}
