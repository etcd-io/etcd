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
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/raft/v3/raftpb"
)

func TestProcessMessages(t *testing.T) {
	cases := []struct {
		name             string
		confState        raftpb.ConfState
		InputMessages    []raftpb.Message
		ExpectedMessages []raftpb.Message
	}{
		{
			name: "only one snapshot message",
			confState: raftpb.ConfState{
				Voters: []uint64{2, 6, 8, 10},
			},
			InputMessages: []raftpb.Message{
				{
					Type: raftpb.MsgSnap,
					To:   8,
					Snapshot: &raftpb.Snapshot{
						Metadata: raftpb.SnapshotMetadata{
							Index: 100,
							Term:  3,
							ConfState: raftpb.ConfState{
								Voters:    []uint64{2, 6, 8},
								AutoLeave: true,
							},
						},
					},
				},
			},
			ExpectedMessages: []raftpb.Message{
				{
					Type: raftpb.MsgSnap,
					To:   8,
					Snapshot: &raftpb.Snapshot{
						Metadata: raftpb.SnapshotMetadata{
							Index: 100,
							Term:  3,
							ConfState: raftpb.ConfState{
								Voters: []uint64{2, 6, 8, 10},
							},
						},
					},
				},
			},
		},
		{
			name: "one snapshot message and one other message",
			confState: raftpb.ConfState{
				Voters: []uint64{2, 7, 8, 12},
			},
			InputMessages: []raftpb.Message{
				{
					Type: raftpb.MsgSnap,
					To:   8,
					Snapshot: &raftpb.Snapshot{
						Metadata: raftpb.SnapshotMetadata{
							Index: 100,
							Term:  3,
							ConfState: raftpb.ConfState{
								Voters:    []uint64{2, 6, 8},
								AutoLeave: true,
							},
						},
					},
				},
				{
					Type: raftpb.MsgApp,
					From: 6,
					To:   8,
				},
			},
			ExpectedMessages: []raftpb.Message{
				{
					Type: raftpb.MsgSnap,
					To:   8,
					Snapshot: &raftpb.Snapshot{
						Metadata: raftpb.SnapshotMetadata{
							Index: 100,
							Term:  3,
							ConfState: raftpb.ConfState{
								Voters: []uint64{2, 7, 8, 12},
							},
						},
					},
				},
				{
					Type: raftpb.MsgApp,
					From: 6,
					To:   8,
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
			require.Truef(t, reflect.DeepEqual(outputMessages, tc.ExpectedMessages), "Unexpected messages, expected: %v, got %v", tc.ExpectedMessages, outputMessages)
		})
	}
}
